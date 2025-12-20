package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"buf-lib-poc/pkg/canton"
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

	var isRoot bool
	var rootKeyPath string
	var targetKeyPath string
	var outputPrefix string

	var prepareCmd = &cobra.Command{
		Use:   "prepare",
		Short: "Preparation commands for topology transactions",
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
				"namespace_delegation": map[string]interface{}{
					"namespace": fingerprint,
					"target_key": map[string]interface{}{
						"format":     info.Format,
						"public_key": targetData, // Engine.Generate will handle base64 encoding for the JSON
						"usage":      []string{"SIGNING_KEY_USAGE_NAMESPACE"},
						"key_spec":   info.KeySpec,
					},
					"can_sign_all_mappings": map[string]interface{}{},
				},
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

	var prepPath string
	var signaturePath string
	var signatureAlgo string
	var finalOutput string

	var assembleCmd = &cobra.Command{
		Use:   "assemble",
		Short: "Assembly commands for topology transactions",
	}

	var assembleDelegationCmd = &cobra.Command{
		Use:   "delegation",
		Short: "Assemble a signed namespace delegation transaction",
		Run: func(cmd *cobra.Command, args []string) {
			if prepPath == "" || signaturePath == "" || signatureAlgo == "" || finalOutput == "" {
				log.Fatal("missing required flags: --prepared-transaction, --signature, --signature-algorithm, --output")
			}

			schemaFile := os.Getenv("PROTO_IMAGE")
			if schemaFile == "" {
				log.Fatal("PROTO_IMAGE must be set to point to Canton topology image")
			}

			// 1. Load Prep Data & Extract Fingerprint
			prepData, err := os.ReadFile(prepPath)
			if err != nil {
				log.Fatalf("failed to read prepared transaction: %v", err)
			}

			// Decode as versioned to get the inner data
			out, err := e.Decode(context.Background(), schemaFile, "com.digitalasset.canton.protocol.v30.TopologyTransaction", prepData, true)
			if err != nil {
				log.Fatalf("failed to decode prepared transaction: %v", err)
			}

			// Extract fingerprint
			mapping, ok := out.(map[string]interface{})["mapping"].(map[string]interface{})
			if !ok {
				log.Fatal("invalid transaction structure: missing mapping")
			}
			nsDelegation, ok := mapping["namespaceDelegation"].(map[string]interface{})
			if !ok {
				log.Fatal("invalid transaction structure: missing namespaceDelegation")
			}
			fingerprint, _ := nsDelegation["namespace"].(string)
			if fingerprint == "" {
				log.Fatal("invalid transaction structure: missing namespace")
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
				"transaction": prepData, // Engine.Generate handles []byte to base64
				"signatures": []interface{}{
					map[string]interface{}{
						"format":                 sigMeta.Format,
						"signature":              sigData,
						"signed_by":              fingerprint,
						"signing_algorithm_spec": sigMeta.Algorithm,
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

	delegationCmd.Flags().BoolVar(&isRoot, "root", false, "Is this a self-signed root delegation")
	delegationCmd.Flags().StringVar(&rootKeyPath, "root-key", "", "Path to root public key")
	delegationCmd.Flags().StringVar(&targetKeyPath, "target-key", "", "Path to target public key")
	delegationCmd.Flags().StringVar(&outputPrefix, "output", "", "Output prefix")

	assembleDelegationCmd.Flags().StringVar(&prepPath, "prepared-transaction", "", "Path to prepared transaction (.prep)")
	assembleDelegationCmd.Flags().StringVar(&signaturePath, "signature", "", "Path to signature file")
	assembleDelegationCmd.Flags().StringVar(&signatureAlgo, "signature-algorithm", "", "Signature algorithm (ed25519, ecdsa256, ecdsa384)")
	assembleDelegationCmd.Flags().StringVar(&finalOutput, "output", "", "Output path")

	prepareCmd.AddCommand(delegationCmd)
	assembleCmd.AddCommand(assembleDelegationCmd)
	rootCmd.AddCommand(fingerprintCmd)
	rootCmd.AddCommand(prepareCmd)
	rootCmd.AddCommand(assembleCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
