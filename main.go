package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bufbuild/protocompile"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "buf-poc",
		Short: "Buf PoC CLI tool",
	}

	var templateCmd = &cobra.Command{
		Use:   "template [proto-file] [message-name]",
		Short: "Generate a JSON template for a given protobuf message",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			protoFile := args[0]
			messageName := args[1]

			if err := runTemplate(protoFile, messageName); err != nil {
				log.Fatalf("Error: %v", err)
			}
		},
	}

	rootCmd.AddCommand(templateCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runTemplate(protoFile, messageName string) error {
	ctx := context.Background()

	// 1. Compile the proto file
	absPath, err := filepath.Abs(protoFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	dir := filepath.Dir(absPath)
	file := filepath.Base(absPath)

	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			ImportPaths: []string{dir},
		},
	}
	files, err := compiler.Compile(ctx, file)
	if err != nil {
		return fmt.Errorf("failed to compile: %v", err)
	}

	// 2. Resolve the message descriptor by fully qualified name
	var foundMsg protoreflect.MessageDescriptor
	for _, f := range files {
		// Try to find by full name
		foundMsg = findMessage(f, messageName)
		if foundMsg != nil {
			break
		}
	}

	if foundMsg == nil {
		return fmt.Errorf("could not find message: %s", messageName)
	}

	// 3. Generate and print template
	template := generateJSONTemplate(foundMsg)
	templateJSON, _ := json.MarshalIndent(template, "", "  ")
	fmt.Println(string(templateJSON))

	return nil
}

// findMessage searches for a message descriptor by its fully qualified name
func findMessage(f protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	msgs := f.Messages()
	for i := 0; i < msgs.Len(); i++ {
		m := msgs.Get(i)
		if string(m.FullName()) == name {
			return m
		}
		// Also search nested messages
		if nested := findNestedMessage(m, name); nested != nil {
			return nested
		}
	}
	return nil
}

func findNestedMessage(m protoreflect.MessageDescriptor, name string) protoreflect.MessageDescriptor {
	msgs := m.Messages()
	for i := 0; i < msgs.Len(); i++ {
		nested := msgs.Get(i)
		if string(nested.FullName()) == name {
			return nested
		}
		if res := findNestedMessage(nested, name); res != nil {
			return res
		}
	}
	return nil
}

// generateJSONTemplate recursively creates a map representing a JSON template for a message
func generateJSONTemplate(md protoreflect.MessageDescriptor) map[string]interface{} {
	template := make(map[string]interface{})
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		template[string(fd.Name())] = getExampleValue(fd)
	}
	return template
}

func getExampleValue(fd protoreflect.FieldDescriptor) interface{} {
	if fd.IsList() {
		return []interface{}{getSingleExampleValue(fd)}
	}
	if fd.IsMap() {
		return map[string]interface{}{
			"key": getSingleExampleValue(fd.MapValue()),
		}
	}
	return getSingleExampleValue(fd)
}

func getSingleExampleValue(fd protoreflect.FieldDescriptor) interface{} {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return "example_string"
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return 0
	case protoreflect.BoolKind:
		return false
	case protoreflect.EnumKind:
		return string(fd.Enum().Values().Get(0).Name())
	case protoreflect.MessageKind:
		return generateJSONTemplate(fd.Message())
	default:
		return nil
	}
}
