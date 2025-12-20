package main

import (
	"context"
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
		Use:   "canton-proto",
		Short: "Specialized Canton Protobuf tool",
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
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration")

	var isBase64 bool
	var inspectCmd = &cobra.Command{
		Use:   "inspect [message-name] [data]",
		Short: "Inspect a Canton message with auto-detection",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			msgName := args[0]
			input := args[1]

			schemaFile := os.Getenv("PROTO_IMAGE")
			if schemaFile == "" {
				log.Fatal("PROTO_IMAGE must be set for specialized tool")
			}

			binaryData, err := io.ReadData(input, isBase64)
			if err != nil {
				log.Fatalf("failed to read data: %v", err)
			}

			// Try versioned decoding by default as it's common in Canton
			out, err := e.Decode(context.Background(), schemaFile, msgName, binaryData, true)
			if err != nil {
				// Fallback to non-versioned if it fails
				out, err = e.Decode(context.Background(), schemaFile, msgName, binaryData, false)
				if err != nil {
					log.Fatalf("failed to decode: %v", err)
				}
			}

			outputJSON, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(outputJSON))
		},
	}

	inspectCmd.Flags().BoolVarP(&isBase64, "base64", "b", false, "Is input base64 encoded")
	rootCmd.AddCommand(inspectCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
