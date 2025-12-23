package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"buf-lib-poc/pkg/canton"
	"buf-lib-poc/pkg/io"
	"buf-lib-poc/pkg/loader"
	"buf-lib-poc/pkg/patch"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
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
	revokeFlag    bool
	serialFlag    int64
	restrictions  string
	inputPath     string
	pubKeyPaths   []string
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

			// 3. Build Transaction JSON using Patching Logic
			tx := make(map[string]interface{})

			// Operation & Serial
			op := "TOPOLOGY_CHANGE_OP_ADD_REPLACE"
			if revokeFlag {
				op = "TOPOLOGY_CHANGE_OP_REMOVE"
			}
			patch.Set(tx, "operation", op)
			patch.Set(tx, "serial", serialFlag)

			// Shared Delegation Fields
			prefix := "mapping.namespaceDelegation"
			patch.Set(tx, prefix+".namespace", fingerprint)
			patch.Set(tx, prefix+".targetKey.format", info.Format)
			patch.Set(tx, prefix+".targetKey.publicKey", targetData)
			patch.Set(tx, prefix+".targetKey.usage", []string{"SIGNING_KEY_USAGE_NAMESPACE"})
			patch.Set(tx, prefix+".targetKey.keySpec", info.KeySpec)

			// Restrictions
			switch restrictions {
			case "all":
				patch.Set(tx, prefix+".canSignAllMappings", map[string]interface{}{})
			case "all-but-delegation":
				patch.Set(tx, prefix+".canSignAllButNamespaceDelegations", map[string]interface{}{})
			default:
				// Comma-separated list of mapping codes
				codes := strings.Split(restrictions, ",")
				patch.Set(tx, prefix+".canSignSpecificMapings.mappings", codes)
			}

			jsonData, _ := json.Marshal(tx)

			// 4. Generate Binary Prep File
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

			// 5. Compute and Write Hash
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
	delegationCmd.Flags().BoolVar(&revokeFlag, "revoke", false, "Revoke the transaction (operation = REMOVE)")
	delegationCmd.Flags().Int64Var(&serialFlag, "serial", 1, "Transaction serial number")
	delegationCmd.Flags().StringVar(&restrictions, "restrictions", "all", "Signing restrictions (all, all-but-delegation, or comma-separated mapping codes)")

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
			prepData, err := io.ReadData(prepFilePath, false)
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

	var verifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify signatures in a SignedTopologyTransaction",
		Run: func(cmd *cobra.Command, args []string) {
			if inputPath == "" || len(pubKeyPaths) == 0 {
				log.Fatal("missing required flags: --input, --public-key")
			}

			schemaFile := os.Getenv("PROTO_IMAGE")
			if schemaFile == "" {
				log.Fatal("PROTO_IMAGE must be set to point to Canton topology image")
			}

			// 1. Load Public Keys and compute fingerprints
			keys := make(map[string][]byte)
			for _, p := range pubKeyPaths {
				data, err := io.ReadData(p, false)
				if err != nil {
					log.Fatalf("failed to read public key %s: %v", p, err)
				}
				fp := canton.Fingerprint(data)
				keys[fp] = data
				fmt.Printf("Loaded key for fingerprint: %s\n", fp)
			}

			inputData, err := io.ReadData(inputPath, false)
			if err != nil {
				log.Fatalf("failed to read input file: %v", err)
			}

			// 2. Handle Version Wrapping
			// Try to unwrap as UntypedVersionedMessage
			// We load it from the same schemaFile (it's now bundled in the image)
			wrapperFiles, err := e.Loader.LoadSchema(context.Background(), schemaFile)
			if err == nil {
				wrapperDesc := loader.FindMessage(wrapperFiles, "com.digitalasset.canton.version.v1.UntypedVersionedMessage")
				if wrapperDesc != nil {
					wrapperMsg := dynamicpb.NewMessage(wrapperDesc)
					if err := proto.Unmarshal(inputData, wrapperMsg); err == nil {
						// Check if data field is present and non-empty
						innerData := wrapperMsg.Get(wrapperDesc.Fields().ByName("data")).Bytes()
						if len(innerData) > 0 {
							// It was a valid wrapper, use inner data
							inputData = innerData
						}
					}
				}
			}

			// 3. Load Schema and SignedTopologyTransaction
			files, err := e.Loader.LoadSchema(context.Background(), schemaFile)
			if err != nil {
				log.Fatalf("failed to load schema: %v", err)
			}
			foundMsg := loader.FindMessage(files, "com.digitalasset.canton.protocol.v30.SignedTopologyTransaction")
			if foundMsg == nil {
				log.Fatal("could not find SignedTopologyTransaction in schema")
			}

			// 4. Unmarshal SignedTopologyTransaction
			// Crucial: Use dynamicpb to preserve raw bytes, NO recursive expansion
			signedTx := dynamicpb.NewMessage(foundMsg)
			if err := proto.Unmarshal(inputData, signedTx); err != nil {
				log.Fatalf("failed to unmarshal SignedTopologyTransaction: %v", err)
			}

			// 5. Extract Transaction Bytes & Compute Hash
			txField := foundMsg.Fields().ByName("transaction")
			rawTx := signedTx.Get(txField).Bytes()
			if len(rawTx) == 0 {
				log.Fatal("transaction field is empty")
			}

			// Canton Hash Purpose 11 = Topology Transaction
			txHash := canton.ComputeHash(rawTx, 11)
			fmt.Printf("Computed transaction hash: %x\n", txHash)

			// 6. Verify Signatures
			sigField := foundMsg.Fields().ByName("signatures")
			sigsList := signedTx.Get(sigField).List()

			allValid := true
			for i := 0; i < sigsList.Len(); i++ {
				sigVal := sigsList.Get(i).Message()
				sigDesc := sigVal.Descriptor()

				fp := sigVal.Get(sigDesc.Fields().ByName("signed_by")).String()
				// Handle both snake_case and camelCase for robustness, though standard should be one.
				if fp == "" {
					// try camelCase just in case dynamicpb behaves oddly
					fp = sigVal.Get(sigDesc.Fields().ByName("signedBy")).String()
				}

				algoEnumVal := sigVal.Get(sigDesc.Fields().ByName("signing_algorithm_spec")).Enum()
				// if default 0, might look empty? UNSPECIFIED = 0.

				algoName := string(sigDesc.Fields().ByName("signing_algorithm_spec").Enum().Values().ByNumber(algoEnumVal).Name())

				sigData := sigVal.Get(sigDesc.Fields().ByName("signature")).Bytes()

				fmt.Printf("Checking signature %d by %s (%s)...\n", i, fp, algoName)
				pubKey, ok := keys[fp]
				if !ok {
					fmt.Printf("  WARNING: Public key for fingerprint %s not provided\n", fp)
					allValid = false
					continue
				}

				valid, err := canton.VerifySignature(txHash, sigData, pubKey, algoName)
				if err != nil {
					fmt.Printf("  ERROR: %v\n", err)
					allValid = false
				} else if valid {
					fmt.Printf("  SUCCESS: Signature is valid\n")
				} else {
					fmt.Printf("  FAILURE: Signature is INVALID\n")
					allValid = false
				}
			}

			if !allValid {
				os.Exit(1)
			}
		},
	}
	verifyCmd.Flags().StringVar(&inputPath, "input", "", "Path to SignedTopologyTransaction binary")
	verifyCmd.Flags().StringSliceVar(&pubKeyPaths, "public-key", nil, "Path(s) to public key(s) for verification")

	topologyCmd.AddCommand(prepareCmd)
	topologyCmd.AddCommand(assembleCmd)
	topologyCmd.AddCommand(verifyCmd)

	cantonCmd.AddCommand(topologyCmd)
}
