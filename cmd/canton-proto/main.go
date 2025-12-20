package main

import (
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/canton"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "canton-proto",
		Short: "Specialized Canton Protobuf tool",
	}

	var isBase64 bool
	var fingerprintCmd = &cobra.Command{
		Use:   "fingerprint [public-key-file]",
		Short: "Compute Canton fingerprint of a public key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]
			data, err := io.ReadData(input, isBase64)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			fmt.Println(canton.Fingerprint(data))
		},
	}

	fingerprintCmd.Flags().BoolVarP(&isBase64, "base64", "b", false, "Is input base64 encoded")
	rootCmd.AddCommand(fingerprintCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
