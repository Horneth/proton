package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/loader"
	"buf-lib-poc/pkg/template"

	"github.com/spf13/cobra"
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

	rootCmd.AddCommand(templateCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
