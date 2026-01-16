package main

import (
	"encoding/hex"
	"fmt"
	"log"

	"buf-lib-poc/pkg/canton"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

var (
	isBase64Crypto bool
	signAlgo       string
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

	var signCmd = &cobra.Command{
		Use:   "sign [private-key-file] [data-file]",
		Short: "Sign data using a private key",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			privKeyPath := args[0]
			dataPath := args[1]

			privKey, err := io.ReadData(privKeyPath, isBase64Crypto)
			if err != nil {
				log.Fatalf("failed to read private key: %v", err)
			}

			data, err := io.ReadData(dataPath, isBase64Crypto)
			if err != nil {
				log.Fatalf("failed to read data: %v", err)
			}

			sig, err := canton.Sign(data, privKey, signAlgo)
			if err != nil {
				log.Fatalf("signing failed: %v", err)
			}

			fmt.Print(io.EncodeData(sig, true))
		},
	}
	signCmd.Flags().BoolVarP(&isBase64Crypto, "base64", "b", false, "Is input base64 encoded")
	signCmd.Flags().StringVarP(&signAlgo, "algo", "a", "ed25519", "Signing algorithm (ed25519, ecdsa256, ecdsa384)")

	cryptoCmd.AddCommand(fingerprintCmd)
	cryptoCmd.AddCommand(signCmd)

	var outputFormat string
	var hashNonceCmd = &cobra.Command{
		Use:   "hash-nonce [synchronizer-id] [nonce]",
		Short: "Compute Canton authentication token hash",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			synchronizerId := args[0]
			nonceInput := args[1]

			nonce, err := io.ReadData(nonceInput, isBase64Crypto)
			if err != nil {
				log.Fatalf("failed to read nonce: %v", err)
			}

			hash := canton.ComputeAuthenticationTokenHash(nonce, synchronizerId)
			if outputFormat == "base64" {
				fmt.Print(io.EncodeData(hash, true))
			} else {
				fmt.Println(hex.EncodeToString(hash))
			}
		},
	}
	hashNonceCmd.Flags().BoolVarP(&isBase64Crypto, "base64", "b", false, "Is nonce base64 encoded")
	hashNonceCmd.Flags().StringVarP(&outputFormat, "output", "o", "hex", "Output format (hex, base64)")

	cryptoCmd.AddCommand(hashNonceCmd)
}
