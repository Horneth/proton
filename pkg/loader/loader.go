package loader

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// SchemaLoader defines the interface for loading protobuf schemas
type SchemaLoader struct {
	ImportPaths []string
}

// LoadSchema loads a schema from a file (proto, binary image, or JSON image)
func (l *SchemaLoader) LoadSchema(ctx context.Context, path string) ([]protoreflect.FileDescriptor, error) {
	if strings.HasSuffix(path, ".proto") {
		return l.loadFromProto(ctx, path)
	}

	// Try loading as a Buf image (FileDescriptorSet)
	return l.loadFromImage(path)
}

func (l *SchemaLoader) loadFromProto(ctx context.Context, path string) ([]protoreflect.FileDescriptor, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absPath)
	file := filepath.Base(absPath)

	importPaths := append([]string{dir}, l.ImportPaths...)

	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			ImportPaths: importPaths,
		},
	}
	files, err := compiler.Compile(ctx, file)
	if err != nil {
		return nil, err
	}

	var result []protoreflect.FileDescriptor
	for _, f := range files {
		result = append(result, f)
	}
	return result, nil
}

func (l *SchemaLoader) loadFromImage(path string) ([]protoreflect.FileDescriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Decompress if gzipped (magic number 0x1f 0x8b)
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gr.Close()
		data, err = io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip content: %v", err)
		}
	}

	fds := &descriptorpb.FileDescriptorSet{}

	// Try binary first
	err = proto.Unmarshal(data, fds)
	if err != nil || len(fds.File) == 0 {
		// Try JSON
		unmarshalOptions := protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}
		err = unmarshalOptions.Unmarshal(data, fds)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image as binary or JSON: %v", err)
		}
	}

	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("failed to create file registry: %v", err)
	}

	var slice []protoreflect.FileDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		slice = append(slice, fd)
		return true
	})

	return slice, nil
}

// FindMessage searches for a message by fully qualified name in the given files
func FindMessage(files []protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	for _, f := range files {
		if m := findInDescriptor(f, name); m != nil {
			return m
		}
	}
	return nil
}

func findInDescriptor(f protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	msgs := f.Messages()
	for i := 0; i < msgs.Len(); i++ {
		m := msgs.Get(i)
		if string(m.FullName()) == name {
			return m
		}
		if nested := findNested(m, name); nested != nil {
			return nested
		}
	}
	return nil
}

func findNested(m protoreflect.MessageDescriptor, name string) protoreflect.MessageDescriptor {
	msgs := m.Messages()
	for i := 0; i < msgs.Len(); i++ {
		nested := msgs.Get(i)
		if string(nested.FullName()) == name {
			return nested
		}
		if res := findNested(nested, name); res != nil {
			return res
		}
	}
	return nil
}
