package main

import (
	"fmt"
	"log"

	"buf-lib-poc/pkg/canton"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

var (
	isBase64Crypto bool
)

func initCryptoCommands(cryptoCmd *cobra.Command) {
	var fingerprintCmd = &cobra.Command{
		Use:   "fingerprint [public-key-file]",
		Short: "Compute Canton fingerprint of a public key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]
			data, err := io.ReadData(input, isBase64Crypto)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			fmt.Println(canton.Fingerprint(data))
		},
	}
	fingerprintCmd.Flags().BoolVarP(&isBase64Crypto, "base64", "b", false, "Is input base64 encoded")

	cryptoCmd.AddCommand(fingerprintCmd)
}
