package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

var (
	dataFlag         string
	isBase64Flag     bool
	versionedFlag    bool
	outputBase64Flag bool
	versionNumFlag   int32
)

func initProtoCommands(protoCmd *cobra.Command) {
	var templateCmd = &cobra.Command{
		Use:   "template [schema-file] [message-name]",
		Short: "Generate a JSON template from Protobuf message",
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

	var decodeCmd = &cobra.Command{
		Use:   "decode [schema-file] [message-name] ([data])",
		Short: "Decode binary Protobuf data to JSON",
		Args:  cobra.RangeArgs(1, 4),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := remaining[0]

			input := dataFlag
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}

			binaryData, err := io.ReadData(input, isBase64Flag)
			if err != nil {
				log.Fatalf("failed to read input data: %v", err)
			}

			out, err := e.Decode(context.Background(), schemaFile, messageName, binaryData, versionedFlag)
			if err != nil {
				log.Fatalf("failed to decode: %v", err)
			}

			outputJSON, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(outputJSON))
		},
	}
	decodeCmd.Flags().StringVarP(&dataFlag, "data", "d", "", "Input data (binary or base64)")
	decodeCmd.Flags().BoolVarP(&isBase64Flag, "base64", "b", false, "Interpret input data as base64")
	decodeCmd.Flags().BoolVarP(&versionedFlag, "versioned", "V", false, "Unwrap from UntypedVersionedMessage")

	var generateCmd = &cobra.Command{
		Use:   "generate [schema-file] [message-name] ([json-data])",
		Short: "Serialize JSON to binary Protobuf",
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

			input := dataFlag
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
				vPtr = &versionNumFlag
			}

			binaryData, err := e.Generate(context.Background(), schemaFile, messageName, jsonData, vPtr)
			if err != nil {
				log.Fatalf("failed to generate: %v", err)
			}

			if outputBase64Flag {
				fmt.Println(base64.StdEncoding.EncodeToString(binaryData))
			} else {
				os.Stdout.Write(binaryData)
			}
		},
	}
	generateCmd.Flags().StringVarP(&dataFlag, "data", "d", "", "Input JSON data")
	generateCmd.Flags().BoolVarP(&outputBase64Flag, "base64", "b", false, "Output base64 encoded binary")
	generateCmd.Flags().Int32VarP(&versionNumFlag, "versioned", "V", 30, "Wrap in UntypedVersionedMessage with this version")

	protoCmd.AddCommand(templateCmd)
	protoCmd.AddCommand(decodeCmd)
	protoCmd.AddCommand(generateCmd)
}
