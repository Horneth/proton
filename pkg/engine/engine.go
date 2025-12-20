package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"buf-lib-poc/pkg/config"
	"buf-lib-poc/pkg/loader"
	"buf-lib-poc/pkg/processor"
	"buf-lib-poc/pkg/template"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Engine struct {
	Loader *loader.SchemaLoader
	Config *config.Config
}

func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		Loader: &loader.SchemaLoader{},
		Config: cfg,
	}
}

func (e *Engine) Template(ctx context.Context, schemaPath, msgName string) (interface{}, error) {
	resolvedMsgName := e.Config.ResolveAlias(msgName)
	files, err := e.Loader.LoadSchema(ctx, schemaPath)
	if err != nil {
		return nil, err
	}
	foundMsg := loader.FindMessage(files, resolvedMsgName)
	if foundMsg == nil {
		return nil, fmt.Errorf("could not find message: %s", resolvedMsgName)
	}
	return template.GenerateJSONTemplate(foundMsg), nil
}

func (e *Engine) Decode(ctx context.Context, schemaPath, msgName string, binaryData []byte, versioned bool) (interface{}, error) {
	resolvedMsgName := e.Config.ResolveAlias(msgName)

	if versioned {
		wrapperFiles, err := e.Loader.LoadSchema(ctx, "untyped_versioned_message.proto")
		if err != nil {
			return nil, fmt.Errorf("failed to load wrapper schema: %v", err)
		}
		wrapperMsgDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
		wrapperMsg := dynamicpb.NewMessage(wrapperMsgDesc)
		if err := proto.Unmarshal(binaryData, wrapperMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal versioned wrapper: %v", err)
		}
		binaryData = wrapperMsg.Get(wrapperMsgDesc.Fields().ByName("data")).Bytes()
	}

	files, err := e.Loader.LoadSchema(ctx, schemaPath)
	if err != nil {
		return nil, err
	}
	foundMsg := loader.FindMessage(files, resolvedMsgName)
	if foundMsg == nil {
		return nil, fmt.Errorf("could not find message: %s", resolvedMsgName)
	}

	msg := dynamicpb.NewMessage(foundMsg)
	if err := proto.Unmarshal(binaryData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal binary data: %v", err)
	}

	if e.Config != nil {
		proc := &processor.Processor{Loader: e.Loader, Config: e.Config, Files: files}
		return proc.ExpandRecursively(ctx, foundMsg, protoreflect.ValueOfMessage(msg))
	}

	// If no config, just return the standard JSON-friendly map
	jsonData, err := protojson.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out interface{}
	err = json.Unmarshal(jsonData, &out)
	return out, err
}

func (e *Engine) Generate(ctx context.Context, schemaPath, msgName string, jsonData []byte, versionNum *int32) ([]byte, error) {
	resolvedMsgName := e.Config.ResolveAlias(msgName)
	files, err := e.Loader.LoadSchema(ctx, schemaPath)
	if err != nil {
		return nil, err
	}
	foundMsg := loader.FindMessage(files, resolvedMsgName)
	if foundMsg == nil {
		return nil, fmt.Errorf("could not find message: %s", resolvedMsgName)
	}

	if e.Config != nil {
		var mapData interface{}
		if err := json.Unmarshal(jsonData, &mapData); err != nil {
			return nil, fmt.Errorf("failed to parse input JSON: %v", err)
		}

		proc := &processor.Processor{Loader: e.Loader, Config: e.Config, Files: files}
		compressed, err := proc.CompressRecursively(ctx, foundMsg, mapData)
		if err != nil {
			return nil, fmt.Errorf("failed to compress message: %v", err)
		}
		jsonData, _ = json.Marshal(compressed)
	}

	msg := dynamicpb.NewMessage(foundMsg)
	if err := protojson.Unmarshal(jsonData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}
	binaryData, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to binary: %v", err)
	}

	if versionNum != nil {
		wrapperFiles, err := e.Loader.LoadSchema(ctx, "untyped_versioned_message.proto")
		if err != nil {
			return nil, fmt.Errorf("failed to load wrapper schema: %v", err)
		}
		wrapperDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
		wrapperMsg := dynamicpb.NewMessage(wrapperDesc)
		wrapperMsg.Set(wrapperDesc.Fields().ByName("data"), protoreflect.ValueOfBytes(binaryData))
		wrapperMsg.Set(wrapperDesc.Fields().ByName("version"), protoreflect.ValueOfInt32(*versionNum))
		binaryData, _ = proto.Marshal(wrapperMsg)
	}

	return binaryData, nil
}
