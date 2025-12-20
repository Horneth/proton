package processor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"buf-lib-poc/pkg/config"
	"buf-lib-poc/pkg/loader"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Processor struct {
	Loader *loader.SchemaLoader
	Config *config.Config
	Files  []protoreflect.FileDescriptor
}

// ExpandRecursively takes a message and expands its fields according to the config.
func (p *Processor) ExpandRecursively(ctx context.Context, md protoreflect.MessageDescriptor, msg protoreflect.Value) (interface{}, error) {
	if msg.Message() == nil {
		return nil, nil
	}

	// 1. Convert message to JSON map using protojson to get standard behavior
	jsonData, err := protojson.Marshal(msg.Message().Interface())
	if err != nil {
		return nil, err
	}

	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	// 2. If it's a map, walk it and expand
	if m, ok := data.(map[string]interface{}); ok {
		return p.expandMap(ctx, md, m)
	}
	return data, nil
}

func (p *Processor) expandMap(ctx context.Context, md protoreflect.MessageDescriptor, data map[string]interface{}) (map[string]interface{}, error) {
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		jsonName := fd.JSONName()
		val, ok := data[jsonName]
		if !ok {
			continue
		}

		// Check for mapping
		var mapped *config.Mapping
		for _, m := range p.Config.Mappings {
			if m.Type == string(md.FullName()) && m.Field == string(fd.Name()) {
				mapped = &m
				break
			}
		}

		if mapped != nil && fd.Kind() == protoreflect.BytesKind {
			// Field is a nested message in bytes
			str, ok := val.(string)
			if !ok {
				continue // Should be base64 string from protojson
			}
			bytes, err := base64.StdEncoding.DecodeString(str)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 field %s: %v", jsonName, err)
			}

			expanded, err := p.expandBytes(ctx, bytes, mapped)
			if err != nil {
				return nil, err
			}
			data[jsonName] = expanded
		} else if fd.Kind() == protoreflect.MessageKind {
			// Nested message - if it's a map, recurse
			if subMap, ok := val.(map[string]interface{}); ok {
				expanded, err := p.expandMap(ctx, fd.Message(), subMap)
				if err != nil {
					return nil, err
				}
				data[jsonName] = expanded
			} else if subList, ok := val.([]interface{}); ok && fd.IsList() {
				for j, item := range subList {
					if itemMap, ok := item.(map[string]interface{}); ok {
						expanded, err := p.expandMap(ctx, fd.Message(), itemMap)
						if err != nil {
							return nil, err
						}
						subList[j] = expanded
					}
				}
			}
		}
	}
	return data, nil
}

func (p *Processor) expandBytes(ctx context.Context, data []byte, m *config.Mapping) (interface{}, error) {
	binaryData := data
	if m.Versioned {
		wrapperFiles, err := p.Loader.LoadSchema(ctx, "untyped_versioned_message.proto")
		if err != nil {
			return nil, err
		}
		wrapperMsgDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
		if wrapperMsgDesc == nil {
			return nil, fmt.Errorf("wrapper descriptor not found")
		}
		wrapperMsg := dynamicpb.NewMessage(wrapperMsgDesc)
		if err := proto.Unmarshal(binaryData, wrapperMsg); err != nil {
			return nil, err
		}
		binaryData = wrapperMsg.Get(wrapperMsgDesc.Fields().ByName("data")).Bytes()
	}

	targetDesc := loader.FindMessage(p.Files, m.TargetType)
	if targetDesc == nil {
		return nil, fmt.Errorf("target type %s not found", m.TargetType)
	}
	targetMsg := dynamicpb.NewMessage(targetDesc)
	if err := proto.Unmarshal(binaryData, targetMsg); err != nil {
		return nil, err
	}

	return p.ExpandRecursively(ctx, targetDesc, protoreflect.ValueOfMessage(targetMsg))
}

// CompressRecursively takes a JSON map and compresses fields into bytes according to the config.
func (p *Processor) CompressRecursively(ctx context.Context, md protoreflect.MessageDescriptor, data interface{}) (interface{}, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		jsonName := fd.JSONName()
		val, ok := m[jsonName]
		if !ok {
			continue
		}

		// Check for mapping
		var mapped *config.Mapping
		for _, mapping := range p.Config.Mappings {
			if mapping.Type == string(md.FullName()) && mapping.Field == string(fd.Name()) {
				mapped = &mapping
				break
			}
		}

		if mapped != nil && fd.Kind() == protoreflect.BytesKind {
			// Pre-compress the nested object
			compressedBytes, err := p.compressBytes(ctx, val, mapped)
			if err != nil {
				return nil, err
			}
			// Replace with base64 string so protojson.Unmarshal can handle it
			m[jsonName] = base64.StdEncoding.EncodeToString(compressedBytes)
		} else if fd.Kind() == protoreflect.MessageKind {
			if subMap, ok := val.(map[string]interface{}); ok {
				compressed, err := p.CompressRecursively(ctx, fd.Message(), subMap)
				if err != nil {
					return nil, err
				}
				m[jsonName] = compressed
			} else if subList, ok := val.([]interface{}); ok && fd.IsList() {
				for j, item := range subList {
					compressed, err := p.CompressRecursively(ctx, fd.Message(), item)
					if err != nil {
						return nil, err
					}
					subList[j] = compressed
				}
			}
		}
	}
	return m, nil
}

func (p *Processor) compressBytes(ctx context.Context, data interface{}, m *config.Mapping) ([]byte, error) {
	var binaryData []byte
	var err error

	if bytes, ok := data.([]byte); ok {
		// Data is already binary, use it as is
		binaryData = bytes
	} else if str, ok := data.(string); ok {
		// Data is a string, could be base64-encoded binary
		if decoded, err := base64.StdEncoding.DecodeString(str); err == nil {
			binaryData = decoded
		} else {
			return nil, fmt.Errorf("failed to decode base64 string for mapped field: %v", err)
		}
	} else {
		targetDesc := loader.FindMessage(p.Files, m.TargetType)
		if targetDesc == nil {
			return nil, fmt.Errorf("target type %s not found", m.TargetType)
		}

		// 1. Recursively compress the target data
		var finalData interface{}
		finalData, err = p.CompressRecursively(ctx, targetDesc, data)
		if err != nil {
			return nil, err
		}

		// 2. Marshal to binary
		jsonData, err := json.Marshal(finalData)
		if err != nil {
			return nil, err
		}
		targetMsg := dynamicpb.NewMessage(targetDesc)
		if err := protojson.Unmarshal(jsonData, targetMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON for %s: %v", m.TargetType, err)
		}
		binaryData, err = proto.Marshal(targetMsg)
		if err != nil {
			return nil, err
		}
	}

	// 3. Wrap if versioned
	if m.Versioned {
		wrapperFiles, err := p.Loader.LoadSchema(ctx, "untyped_versioned_message.proto")
		if err != nil {
			return nil, err
		}
		wrapperDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
		if wrapperDesc == nil {
			return nil, fmt.Errorf("wrapper descriptor not found")
		}

		// Check if it's already wrapped to avoid double wrapping
		alreadyWrapped := false
		testMsg := dynamicpb.NewMessage(wrapperDesc)
		if err := proto.Unmarshal(binaryData, testMsg); err == nil {
			// Basic check: if it has data and version fields successfully set, it's likely already wrapped
			if len(testMsg.Get(wrapperDesc.Fields().ByName("data")).Bytes()) > 0 {
				alreadyWrapped = true
			}
		}

		if !alreadyWrapped {
			wrapperMsg := dynamicpb.NewMessage(wrapperDesc)
			wrapperMsg.Set(wrapperDesc.Fields().ByName("data"), protoreflect.ValueOfBytes(binaryData))
			wrapperMsg.Set(wrapperDesc.Fields().ByName("version"), protoreflect.ValueOfInt32(m.DefaultVersion))
			binaryData, err = proto.Marshal(wrapperMsg)
			if err != nil {
				return nil, err
			}
		}
	}

	return binaryData, nil
}
