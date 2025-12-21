package main

import (
	"context"
	"encoding/base64"
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

	// --- Helper to resolve schema file and remaining args ---
	resolveSchemaArgs := func(args []string) (string, []string, error) {
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

	// --- Generic Utility Commands ---

	var templateCmd = &cobra.Command{
		Use:   "template [schema-file] [message-name]",
		Short: "Generate a JSON template from Protobuf message",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := remaining[0]

			tmpl, err := e.Template(context.Background(), schemaFile, messageName)
			if err != nil {
				log.Fatalf("failed to generate template: %v", err)
			}

			templateJSON, _ := json.MarshalIndent(tmpl, "", "  ")
			fmt.Println(string(templateJSON))
		},
	}

	var dataFlag string
	var isBase64Flag bool
	var versionedFlag bool

	var decodeCmd = &cobra.Command{
		Use:   "decode [schema-file] [message-name] ([data])",
		Short: "Decode binary Protobuf data to JSON",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := remaining[0]

			input := dataFlag
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}

			binaryData, err := io.ReadData(input, isBase64Flag)
			if err != nil {
				log.Fatalf("failed to read input data: %v", err)
			}

			out, err := e.Decode(context.Background(), schemaFile, messageName, binaryData, versionedFlag)
			if err != nil {
				log.Fatalf("failed to decode: %v", err)
			}

			outputJSON, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(outputJSON))
		},
	}
	decodeCmd.Flags().StringVarP(&dataFlag, "data", "d", "", "Input data (binary or base64)")
	decodeCmd.Flags().BoolVarP(&isBase64Flag, "base64", "b", false, "Interpret input data as base64")
	decodeCmd.Flags().BoolVarP(&versionedFlag, "versioned", "V", false, "Unwrap from UntypedVersionedMessage")

	var outputBase64Flag bool
	var versionNumFlag int32
	var generateCmd = &cobra.Command{
		Use:   "generate [schema-file] [message-name] ([json-data])",
		Short: "Serialize JSON to binary Protobuf",
		Args:  cobra.RangeArgs(1, 3),
		Run: func(cmd *cobra.Command, args []string) {
			schemaFile, remaining, err := resolveSchemaArgs(args)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			if len(remaining) == 0 {
				log.Fatal("missing message name")
			}
			messageName := remaining[0]

			input := dataFlag
			if input == "" {
				if len(remaining) > 1 {
					input = remaining[1]
				} else {
					input = "-"
				}
			}
			jsonData, err := io.ReadData(input, false)
			if err != nil {
				log.Fatalf("failed to read JSON data: %v", err)
			}

			var vPtr *int32
			if cmd.Flags().Changed("versioned") {
				vPtr = &versionNumFlag
			}

			binaryData, err := e.Generate(context.Background(), schemaFile, messageName, jsonData, vPtr)
			if err != nil {
				log.Fatalf("failed to generate: %v", err)
			}

			if outputBase64Flag {
				fmt.Println(base64.StdEncoding.EncodeToString(binaryData))
			} else {
				os.Stdout.Write(binaryData)
			}
		},
	}
	generateCmd.Flags().StringVarP(&dataFlag, "data", "d", "", "Input JSON data")
	generateCmd.Flags().BoolVarP(&outputBase64Flag, "base64", "b", false, "Output base64 encoded binary")
	generateCmd.Flags().Int32VarP(&versionNumFlag, "versioned", "V", 0, "Wrap in UntypedVersionedMessage with this version")

	// --- Canton Specialized Commands ---

	var isBase64Canton bool
	var fingerprintCmd = &cobra.Command{
		Use:   "fingerprint [public-key-file]",
		Short: "Compute Canton fingerprint of a public key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input := args[0]
			data, err := io.ReadData(input, isBase64Canton)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			fmt.Println(canton.Fingerprint(data))
		},
	}
	fingerprintCmd.Flags().BoolVarP(&isBase64Canton, "base64", "b", false, "Is input base64 encoded")

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
				"namespaceDelegation": map[string]interface{}{
					"namespace": fingerprint,
					"targetKey": map[string]interface{}{
						"format":    info.Format,
						"publicKey": targetData, // Engine.Generate will handle base64 encoding for the JSON
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

	var prepFilePath string
	var signaturePath string
	var signatureAlgo string
	var signedBy string
	var finalOutput string

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
				"transaction": prepData, // Engine.Generate handles []byte to base64
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

	delegationCmd.Flags().BoolVar(&isRoot, "root", false, "Is this a self-signed root delegation")
	delegationCmd.Flags().StringVar(&rootKeyPath, "root-key", "", "Path to root public key")
	delegationCmd.Flags().StringVar(&targetKeyPath, "target-key", "", "Path to target public key")
	delegationCmd.Flags().StringVar(&outputPrefix, "output", "", "Output prefix")

	assembleCmd.Flags().StringVar(&prepFilePath, "prepared-transaction", "", "Path to prepared transaction (.prep)")
	assembleCmd.Flags().StringVar(&signaturePath, "signature", "", "Path to signature file")
	assembleCmd.Flags().StringVar(&signatureAlgo, "signature-algorithm", "", "Signature algorithm (ed25519, ecdsa256, ecdsa384)")
	assembleCmd.Flags().StringVar(&signedBy, "signed-by", "", "Fingerprint of the signer")
	assembleCmd.Flags().StringVar(&finalOutput, "output", "", "Output path")

	prepareCmd.AddCommand(delegationCmd)

	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(decodeCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(fingerprintCmd)
	rootCmd.AddCommand(prepareCmd)
	rootCmd.AddCommand(assembleCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
