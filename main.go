package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/config"
	"buf-lib-poc/pkg/io"
	"buf-lib-poc/pkg/loader"
	"buf-lib-poc/pkg/processor"
	"buf-lib-poc/pkg/template"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	var configPath string
	var cfg *config.Config
	var rootCmd = &cobra.Command{
		Use:   "buf-poc",
		Short: "Buf PoC CLI tool",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if configPath == "" {
				home, _ := os.UserHomeDir()
				defaultConfig := home + "/.proto.config.json"
				if _, err := os.Stat(defaultConfig); err == nil {
					configPath = defaultConfig
				}
			}
			if configPath != "" {
				c, err := config.LoadConfig(configPath)
				if err == nil {
					cfg = c
				}
			}
		},
	}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to nesting configuration JSON (defaults to ~/.proto.config.json)")

	// Helper to resolve schema file and remaining args
	resolveSchemaArgs := func(args []string) (string, []string, error) {
		envImage := os.Getenv("PROTO_IMAGE")
		if len(args) > 0 {
			// Check if first arg is a file that exists
			if _, err := os.Stat(args[0]); err == nil {
				return args[0], args[1:], nil
			}
			// If it doesn't exist, but we have env var, use env var
			if envImage != "" {
				return envImage, args, nil
			}
			return "", nil, fmt.Errorf("schema file %s not found and PROTO_IMAGE not set", args[0])
		}

		// No args provided
		if envImage != "" {
			return envImage, nil, nil
		}
		return "", nil, fmt.Errorf("missing schema file and PROTO_IMAGE not set")
	}

	var templateCmd = &cobra.Command{
		Use:   "template [schema-file] [message-name]",
		Short: "Generate a JSON template (supports .proto and Buf images)",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := cfg.ResolveAlias(remaining[0])

			l := &loader.SchemaLoader{}
			files, err := l.LoadSchema(context.Background(), schemaFile)
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
	var versioned bool

	var decodeCmd = &cobra.Command{
		Use:   "decode [schema-file] [message-name] ([data])",
		Short: "Decode binary protobuf data to JSON",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := cfg.ResolveAlias(remaining[0])

			input := data
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}

			l := &loader.SchemaLoader{}
			binaryData, err := io.ReadData(input, isBase64)
			if err != nil {
				log.Fatalf("failed to read input data: %v", err)
			}

			if versioned {
				wrapperFiles, err := l.LoadSchema(context.Background(), "untyped_versioned_message.proto")
				if err != nil {
					log.Fatalf("failed to load wrapper schema: %v", err)
				}
				wrapperMsgDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
				wrapperMsg := dynamicpb.NewMessage(wrapperMsgDesc)
				if err := proto.Unmarshal(binaryData, wrapperMsg); err != nil {
					log.Fatalf("failed to unmarshal versioned wrapper: %v", err)
				}
				binaryData = wrapperMsg.Get(wrapperMsgDesc.Fields().ByName("data")).Bytes()
			}

			files, err := l.LoadSchema(context.Background(), schemaFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}
			foundMsg := loader.FindMessage(files, messageName)
			if foundMsg == nil {
				log.Fatalf("could not find message: %s", messageName)
			}

			msg := dynamicpb.NewMessage(foundMsg)
			if err := proto.Unmarshal(binaryData, msg); err != nil {
				log.Fatalf("failed to unmarshal binary data: %v", err)
			}

			var outputJSON []byte
			if cfg != nil {
				proc := &processor.Processor{Loader: l, Config: cfg, Files: files}
				expanded, err := proc.ExpandRecursively(context.Background(), foundMsg, protoreflect.ValueOfMessage(msg))
				if err != nil {
					log.Fatalf("failed to expand message: %v", err)
				}
				outputJSON, _ = json.MarshalIndent(expanded, "", "  ")
			} else {
				outputJSON, err = protojson.Marshal(msg)
				if err != nil {
					log.Fatalf("failed to marshal to JSON: %v", err)
				}
			}
			fmt.Println(string(outputJSON))
		},
	}

	decodeCmd.Flags().StringVarP(&data, "data", "d", "", "Input data (binary or base64)")
	decodeCmd.Flags().BoolVarP(&isBase64, "base64", "b", false, "Interpret input data as base64")
	decodeCmd.Flags().BoolVarP(&versioned, "versioned", "V", false, "Unwrap from UntypedVersionedMessage")

	var outputBase64 bool
	var versionNum int32
	var generateCmd = &cobra.Command{
		Use:   "generate [schema-file] [message-name] ([json-data])",
		Short: "Serialize JSON to binary protobuf",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := cfg.ResolveAlias(remaining[0])

			l := &loader.SchemaLoader{}
			files, err := l.LoadSchema(context.Background(), schemaFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}
			foundMsg := loader.FindMessage(files, messageName)
			if foundMsg == nil {
				log.Fatalf("could not find message: %s", messageName)
			}

			input := data
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}
			jsonData, err := io.ReadData(input, false)
			if err != nil {
				log.Fatalf("failed to read JSON data: %v", err)
			}

			if cfg != nil {
				var mapData interface{}
				if err := json.Unmarshal(jsonData, &mapData); err != nil {
					log.Fatalf("failed to parse input JSON: %v", err)
				}

				proc := &processor.Processor{Loader: l, Config: cfg, Files: files}
				compressed, err := proc.CompressRecursively(context.Background(), foundMsg, mapData)
				if err != nil {
					log.Fatalf("failed to compress message: %v", err)
				}
				jsonData, _ = json.Marshal(compressed)
			}

			msg := dynamicpb.NewMessage(foundMsg)
			if err := protojson.Unmarshal(jsonData, msg); err != nil {
				log.Fatalf("failed to unmarshal JSON: %v", err)
			}
			binaryData, err := proto.Marshal(msg)
			if err != nil {
				log.Fatalf("failed to marshal to binary: %v", err)
			}

			if cmd.Flags().Changed("versioned") {
				wrapperFiles, err := l.LoadSchema(context.Background(), "untyped_versioned_message.proto")
				if err != nil {
					log.Fatalf("failed to load wrapper schema: %v", err)
				}
				wrapperDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
				wrapperMsg := dynamicpb.NewMessage(wrapperDesc)
				wrapperMsg.Set(wrapperDesc.Fields().ByName("data"), protoreflect.ValueOfBytes(binaryData))
				wrapperMsg.Set(wrapperDesc.Fields().ByName("version"), protoreflect.ValueOfInt32(versionNum))
				binaryData, _ = proto.Marshal(wrapperMsg)
			}

			if outputBase64 {
				fmt.Println(base64.StdEncoding.EncodeToString(binaryData))
			} else {
				os.Stdout.Write(binaryData)
			}
		},
	}

	generateCmd.Flags().StringVarP(&data, "data", "d", "", "Input JSON data")
	generateCmd.Flags().BoolVarP(&outputBase64, "base64", "b", false, "Output base64 encoded binary")
	generateCmd.Flags().Int32VarP(&versionNum, "versioned", "V", 0, "Wrap in UntypedVersionedMessage with this version")

	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(generateCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
