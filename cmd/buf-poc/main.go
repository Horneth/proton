package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/config"
	"buf-lib-poc/pkg/engine"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

func main() {
	var configPath string
	var e *engine.Engine

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
			var cfg *config.Config
			if configPath != "" {
				var err error
				cfg, err = config.LoadConfig(configPath)
				if err != nil {
					log.Printf("warning: failed to load config: %v", err)
				}
			}
			e = engine.NewEngine(cfg)
		},
	}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to nesting configuration JSON (defaults to ~/.proto.config.json)")

	// Helper to resolve schema file and remaining args
	resolveSchemaArgs := func(args []string) (string, []string, error) {
		envImage := os.Getenv("PROTO_IMAGE")
		if len(args) > 0 {
			if _, err := os.Stat(args[0]); err == nil {
				return args[0], args[1:], nil
			}
			if envImage != "" {
				return envImage, args, nil
			}
			return "", nil, fmt.Errorf("schema file %s not found and PROTO_IMAGE not set", args[0])
		}
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
			messageName := remaining[0]

			tmpl, err := e.Template(context.Background(), schemaFile, messageName)
			if err != nil {
				log.Fatalf("failed to generate template: %v", err)
			}

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
			messageName := remaining[0]

			input := data
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}

			binaryData, err := io.ReadData(input, isBase64)
			if err != nil {
				log.Fatalf("failed to read input data: %v", err)
			}

			out, err := e.Decode(context.Background(), schemaFile, messageName, binaryData, versioned)
			if err != nil {
				log.Fatalf("failed to decode: %v", err)
			}

			outputJSON, _ := json.MarshalIndent(out, "", "  ")
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
			messageName := remaining[0]

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

			var vPtr *int32
			if cmd.Flags().Changed("versioned") {
				vPtr = &versionNum
			}

			binaryData, err := e.Generate(context.Background(), schemaFile, messageName, jsonData, vPtr)
			if err != nil {
				log.Fatalf("failed to generate: %v", err)
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
