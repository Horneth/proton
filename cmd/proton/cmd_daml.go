package main

import (
	"fmt"
	"log"

	"buf-lib-poc/pkg/daml/hash"
	interactive "buf-lib-poc/pkg/daml/proto/com/daml/ledger/api/v2/interactive"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	base64HashFlag bool
)

func initDamlCommands(rootCmd *cobra.Command) {
	var damlCmd = &cobra.Command{
		Use:   "daml",
		Short: "Daml transaction utilities",
	}

	var hashCmd = &cobra.Command{
		Use:   "hash [file]",
		Short: "Compute the V2 secure hash of a prepared transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputPath := args[0]
			data, err := io.ReadData(inputPath, false)
			if err != nil {
				log.Fatalf("failed to read input file: %v", err)
			}

			// Unmarshal into PreparedTransaction
			var preparedTx interactive.PreparedTransaction
			if err := proto.Unmarshal(data, &preparedTx); err != nil {
				log.Fatalf("failed to unmarshal prepared transaction: %v", err)
			}

			// Compute Hash
			h, err := hash.HashPreparedTransaction(&preparedTx)
			if err != nil {
				log.Fatalf("failed to compute hash: %v", err)
			}

			fmt.Printf("%x\n", h)
		},
	}
	// hashCmd.Flags().BoolVarP(&base64HashFlag, "base64", "b", false, "Output hash as base64")

	var decodeCmd = &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a binary PreparedTransaction into JSON",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputPath := args[0]
			data, err := io.ReadData(inputPath, false)
			if err != nil {
				log.Fatalf("failed to read input file: %v", err)
			}

			var preparedTx interactive.PreparedTransaction
			if err := proto.Unmarshal(data, &preparedTx); err != nil {
				log.Fatalf("failed to unmarshal prepared transaction: %v", err)
			}

			// Marshal to JSON using protojson for proper enum/field name handling
			m := protojson.MarshalOptions{
				Multiline:       true,
				Indent:          "  ",
				EmitUnpopulated: false,
			}
			jsonBytes, err := m.Marshal(&preparedTx)
			if err != nil {
				log.Fatalf("failed to marshal to JSON: %v", err)
			}

			fmt.Println(string(jsonBytes))
		},
	}

	damlCmd.AddCommand(hashCmd)
	damlCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(damlCmd)
}
