package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/canton"
	"buf-lib-poc/pkg/io"

	"github.com/spf13/cobra"
)

var (
	isRoot        bool
	rootKeyPath   string
	targetKeyPath string
	outputPrefix  string
	prepFilePath  string
	signaturePath string
	signatureAlgo string
	signedBy      string
	finalOutput   string
)

func initCantonCommands(cantonCmd *cobra.Command) {
	topologyCmd := &cobra.Command{
		Use:   "topology",
		Short: "Canton topology transaction commands",
	}

	var delegationCmd = &cobra.Command{
		Use:   "delegation",
		Short: "Prepare a namespace delegation transaction",
		Run: func(cmd *cobra.Command, args []string) {
			if rootKeyPath == "" || (targetKeyPath == "" && !isRoot) || outputPrefix == "" {
				log.Fatal("missing required flags: --root-key, --target-key (unless --root), --output")
			}

			// 1. Resolve Root Key & Fingerprint
			rootData, err := io.ReadData(rootKeyPath, false)
			if err != nil {
				log.Fatalf("failed to read root key: %v", err)
			}
			fingerprint := canton.Fingerprint(rootData)
			fmt.Printf("Root namespace fingerprint: %s\n", fingerprint)

			// 2. Resolve Target Key Info
			tPath := targetKeyPath
			if isRoot {
				tPath = rootKeyPath
			}
			targetData, err := io.ReadData(tPath, false)
			if err != nil {
				log.Fatalf("failed to read target key: %v", err)
			}
			info, err := canton.InspectPublicKey(targetData)
			if err != nil {
				log.Fatalf("failed to inspect target key: %v", err)
			}

			// 3. Build Mapping JSON
			mapping := map[string]interface{}{
				"namespaceDelegation": map[string]interface{}{
					"namespace": fingerprint,
					"targetKey": map[string]interface{}{
						"format":    info.Format,
						"publicKey": targetData,
						"usage":     []string{"SIGNING_KEY_USAGE_NAMESPACE"},
						"keySpec":   info.KeySpec,
					},
					"canSignAllMappings": map[string]interface{}{},
				},
			}
			if isRoot {
				mapping["namespaceDelegation"].(map[string]interface{})["isRootDelegation"] = true
			}

			// 4. Build Transaction JSON
			tx := map[string]interface{}{
				"operation": "TOPOLOGY_CHANGE_OP_ADD_REPLACE",
				"serial":    1,
				"mapping":   mapping,
			}

			jsonData, _ := json.Marshal(tx)

			// 5. Generate Binary Prep File
			schemaFile := os.Getenv("PROTO_IMAGE")
			if schemaFile == "" {
				log.Fatal("PROTO_IMAGE must be set to point to Canton topology image")
			}

			version := int32(30)
			binaryData, err := e.Generate(context.Background(), schemaFile, "com.digitalasset.canton.protocol.v30.TopologyTransaction", jsonData, &version)
			if err != nil {
				log.Fatalf("failed to generate binary transaction: %v", err)
			}

			prepPath := outputPrefix + ".prep"
			if err := os.WriteFile(prepPath, binaryData, 0644); err != nil {
				log.Fatalf("failed to write .prep file: %v", err)
			}
			fmt.Printf("Namespace delegation Transaction written to %s\n", prepPath)

			// 6. Compute and Write Hash
			hash := canton.ComputeHash(binaryData, 11)
			hashPath := outputPrefix + ".hash"
			if err := os.WriteFile(hashPath, hash, 0644); err != nil {
				log.Fatalf("failed to write .hash file: %v", err)
			}
			fmt.Printf("Namespace delegation Transaction Hash written to %s\n", hashPath)
		},
	}

	delegationCmd.Flags().BoolVar(&isRoot, "root", false, "Is this a self-signed root delegation")
	delegationCmd.Flags().StringVar(&rootKeyPath, "root-key", "", "Path to root public key")
	delegationCmd.Flags().StringVar(&targetKeyPath, "target-key", "", "Path to target public key")
	delegationCmd.Flags().StringVar(&outputPrefix, "output", "", "Output prefix")

	var prepareCmd = &cobra.Command{
		Use:   "prepare",
		Short: "Preparation commands for topology transactions",
	}
	prepareCmd.AddCommand(delegationCmd)

	var assembleCmd = &cobra.Command{
		Use:   "assemble",
		Short: "Assemble a signed topology transaction",
		Run: func(cmd *cobra.Command, args []string) {
			if prepFilePath == "" || signaturePath == "" || signatureAlgo == "" || signedBy == "" || finalOutput == "" {
				log.Fatal("missing required flags: --prepared-transaction, --signature, --signature-algorithm, --signed-by, --output")
			}

			schemaFile := os.Getenv("PROTO_IMAGE")
			if schemaFile == "" {
				log.Fatal("PROTO_IMAGE must be set to point to Canton topology image")
			}

			// 1. Load Prep Data
			prepData, err := os.ReadFile(prepFilePath)
			if err != nil {
				log.Fatalf("failed to read prepared transaction: %v", err)
			}

			// 2. Load Signature
			sigData, err := io.ReadData(signaturePath, false)
			if err != nil {
				log.Fatalf("failed to read signature: %v", err)
			}

			// 3. Get Signature Metadata
			sigMeta, err := canton.GetSignatureMetadata(signatureAlgo)
			if err != nil {
				log.Fatalf("invalid signature algorithm: %v", err)
			}

			// 4. Build Signed Transaction JSON
			signedTx := map[string]interface{}{
				"transaction": prepData,
				"signatures": []interface{}{
					map[string]interface{}{
						"format":               sigMeta.Format,
						"signature":            sigData,
						"signedBy":             signedBy,
						"signingAlgorithmSpec": sigMeta.Algorithm,
					},
				},
				"proposal": false,
			}

			jsonData, _ := json.Marshal(signedTx)

			// 5. Generate Final Binary
			version := int32(30)
			binaryData, err := e.Generate(context.Background(), schemaFile, "com.digitalasset.canton.protocol.v30.SignedTopologyTransaction", jsonData, &version)
			if err != nil {
				log.Fatalf("failed to generate signed transaction: %v", err)
			}

			if err := os.WriteFile(finalOutput, binaryData, 0644); err != nil {
				log.Fatalf("failed to write certificate: %v", err)
			}
			fmt.Printf("Certificate written to %s\n", finalOutput)
		},
	}
	assembleCmd.Flags().StringVar(&prepFilePath, "prepared-transaction", "", "Path to prepared transaction (.prep)")
	assembleCmd.Flags().StringVar(&signaturePath, "signature", "", "Path to signature file")
	assembleCmd.Flags().StringVar(&signatureAlgo, "signature-algorithm", "", "Signature algorithm (ed25519, ecdsa256, ecdsa384)")
	assembleCmd.Flags().StringVar(&signedBy, "signed-by", "", "Fingerprint of the signer")
	assembleCmd.Flags().StringVar(&finalOutput, "output", "", "Output path")

	topologyCmd.AddCommand(prepareCmd)
	topologyCmd.AddCommand(assembleCmd)

	cantonCmd.AddCommand(topologyCmd)
}
