package main

import (
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/config"
	"buf-lib-poc/pkg/engine"

	"github.com/spf13/cobra"
)

var (
	e *engine.Engine
)

func main() {
	var configPath string

	var rootCmd = &cobra.Command{
		Use:   "proton",
		Short: "Proton: Universal Protobuf & Canton Toolkit",
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

	// --- Command Groups ---

	var protoCmd = &cobra.Command{
		Use:   "proto",
		Short: "Generic Protobuf utility commands",
	}

	var cantonCmd = &cobra.Command{
		Use:   "canton",
		Short: "Canton specialized commands",
	}

	var cryptoCmd = &cobra.Command{
		Use:   "crypto",
		Short: "Cryptographic utility commands",
	}

	// --- Initialize Subcommands ---
	initProtoCommands(protoCmd)
	initCantonCommands(cantonCmd)
	initCryptoCommands(cryptoCmd)
	initDamlCommands(rootCmd)

	// --- Add to Root ---
	rootCmd.AddCommand(protoCmd)
	rootCmd.AddCommand(cantonCmd)
	rootCmd.AddCommand(cryptoCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// resolveSchemaArgs is a helper shared across command files
func resolveSchemaArgs(args []string) (string, []string, error) {
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
