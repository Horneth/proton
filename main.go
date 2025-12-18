package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/io"
	"buf-lib-poc/pkg/loader"
	"buf-lib-poc/pkg/template"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "buf-poc",
		Short: "Buf PoC CLI tool",
	}

	var templateCmd = &cobra.Command{
		Use:   "template [input-file] [message-name]",
		Short: "Generate a JSON template for a given protobuf message (supports .proto and Buf images)",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			inputFile := args[0]
			messageName := args[1]

			l := &loader.SchemaLoader{}
			files, err := l.LoadSchema(ctx, inputFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}

			foundMsg := loader.FindMessage(files, messageName)
			if foundMsg == nil {
				log.Fatalf("could not find message: %s", messageName)
			}

			tmpl := template.GenerateJSONTemplate(foundMsg)
			templateJSON, _ := json.MarshalIndent(tmpl, "", "  ")
			fmt.Println(string(templateJSON))
		},
	}

	var data string
	var isBase64 bool

	var decodeCmd = &cobra.Command{
		Use:   "decode [schema-file] [message-name] ([data])",
		Short: "Decode binary protobuf data to JSON",
		Long: `Decode binary protobuf data to JSON format.
Data can be provided as a positional argument, via the --data flag, or piped from stdin.
Use @path to read from a file. Use - to explicitly read from stdin.`,
		Args: cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			schemaFile := args[0]
			messageName := args[1]

			// Handle input data source
			input := data
			if input == "" {
				if len(args) > 2 {
					input = args[2]
				} else {
					input = "-" // Default to stdin if no data arg provided
				}
			}

			// 1. Load schema
			l := &loader.SchemaLoader{}
			files, err := l.LoadSchema(ctx, schemaFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}

			foundMsg := loader.FindMessage(files, messageName)
			if foundMsg == nil {
				log.Fatalf("could not find message: %s", messageName)
			}

			// 2. Read input data
			binaryData, err := io.ReadData(input, isBase64)
			if err != nil {
				log.Fatalf("failed to read input data: %v", err)
			}

			// 3. Decode
			msg := dynamicpb.NewMessage(foundMsg)
			if err := proto.Unmarshal(binaryData, msg); err != nil {
				log.Fatalf("failed to unmarshal binary data: %v", err)
			}

			// 4. Print JSON
			output, err := protojson.Marshal(msg)
			if err != nil {
				log.Fatalf("failed to marshal to JSON: %v", err)
			}
			fmt.Println(string(output))
		},
	}

	decodeCmd.Flags().StringVarP(&data, "data", "d", "", "Input data (binary or base64)")
	decodeCmd.Flags().BoolVarP(&isBase64, "base64", "b", false, "Interpret input data as base64")

	var outputBase64 bool
	var generateCmd = &cobra.Command{
		Use:   "generate [schema-file] [message-name] ([json-data])",
		Short: "Serialize JSON to binary protobuf",
		Long: `Serialize JSON data to binary protobuf format.
JSON data can be provided as a positional argument, via the --data flag, or piped from stdin.
Use @path to read from a file. Use - to explicitly read from stdin.`,
		Args: cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			schemaFile := args[0]
			messageName := args[1]

			// Handle input data source
			input := data
			if input == "" {
				if len(args) > 2 {
					input = args[2]
				} else {
					input = "-" // Default to stdin if no data arg provided
				}
			}

			// 1. Load schema
			l := loader.SchemaLoader{}
			files, err := l.LoadSchema(ctx, schemaFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}

			foundMsg := loader.FindMessage(files, messageName)
			if foundMsg == nil {
				log.Fatalf("could not find message: %s", messageName)
			}

			// 2. Read JSON data
			jsonData, err := io.ReadData(input, false)
			if err != nil {
				log.Fatalf("failed to read JSON data: %v", err)
			}

			// 3. Serialize
			msg := dynamicpb.NewMessage(foundMsg)
			if err := protojson.Unmarshal(jsonData, msg); err != nil {
				log.Fatalf("failed to unmarshal JSON: %v", err)
			}

			binaryData, err := proto.Marshal(msg)
			if err != nil {
				log.Fatalf("failed to marshal to binary: %v", err)
			}

			// 4. Output
			if outputBase64 {
				fmt.Println(base64.StdEncoding.EncodeToString(binaryData))
			} else {
				os.Stdout.Write(binaryData)
			}
		},
	}

	generateCmd.Flags().StringVarP(&data, "data", "d", "", "Input JSON data")
	generateCmd.Flags().BoolVarP(&outputBase64, "base64", "b", false, "Output base64 encoded binary")

	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(generateCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
