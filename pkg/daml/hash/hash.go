package hash

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	apiv2 "buf-lib-poc/pkg/daml/proto/com/daml/ledger/api/v2"
	"buf-lib-poc/pkg/daml/proto/com/daml/ledger/api/v2/interactive"
	transactionv1 "buf-lib-poc/pkg/daml/proto/com/daml/ledger/api/v2/interactive/transaction/v1"
)

const (
	PreparedTransactionHashPurpose = "\x00\x00\x00\x30"
	HashingSchemeVersionByte       = "\x02"
	NodeEncodingVersion            = "\x01"
)

// HashPreparedTransaction computes the V2 SHA256 hash of a PreparedTransaction message.
func HashPreparedTransaction(tx *interactive.PreparedTransaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}

	damlTx := tx.Transaction
	nodesMap, seedsMap := buildNodesAndSeedsMap(damlTx)

	txHash := hashTransaction(damlTx, nodesMap, seedsMap)
	metaHash := hashMetadata(tx.Metadata, nodesMap, seedsMap)

	h := sha256.New()
	h.Write([]byte(PreparedTransactionHashPurpose))
	h.Write([]byte(HashingSchemeVersionByte))
	h.Write(txHash)
	h.Write(metaHash)
	return h.Sum(nil), nil
}

func buildNodesAndSeedsMap(damlTx *interactive.DamlTransaction) (map[string]*interactive.DamlTransaction_Node, map[string][]byte) {
	nodesMap := make(map[string]*interactive.DamlTransaction_Node)
	if damlTx != nil {
		for _, node := range damlTx.Nodes {
			nodesMap[node.NodeId] = node
		}
	}
	seedsMap := make(map[string][]byte)
	if damlTx != nil {
		for _, seed := range damlTx.NodeSeeds {
			seedsMap[fmt.Sprintf("%d", seed.NodeId)] = seed.Seed
		}
	}
	return nodesMap, seedsMap
}

func hashTransaction(tx *interactive.DamlTransaction, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	encoded := encodeTransaction(tx, nodesMap, seedsMap)
	h := sha256.New()
	h.Write([]byte(PreparedTransactionHashPurpose))
	h.Write(encoded)
	return h.Sum(nil)
}

func encodeTransaction(tx *interactive.DamlTransaction, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	res := encodeString(tx.Version)
	roots := encodeRepeated(tx.Roots, func(rootID string) []byte {
		if node, ok := nodesMap[rootID]; ok {
			return sha256Sum(encodeNode(node, nodesMap, seedsMap))
		}
		return make([]byte, 32)
	})
	return append(res, roots...)
}

func encodeNode(node *interactive.DamlTransaction_Node, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	switch n := node.VersionedNode.(type) {
	case *interactive.DamlTransaction_Node_V1:
		return encodeNodeV1(n.V1, node.NodeId, nodesMap, seedsMap)
	default:
		return []byte{}
	}
}

func encodeNodeV1(v1Node *transactionv1.Node, nodeID string, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	switch t := v1Node.NodeType.(type) {
	case *transactionv1.Node_Create:
		return encodeCreateNode(t.Create, nodeID, seedsMap)
	case *transactionv1.Node_Exercise:
		return encodeExerciseNode(t.Exercise, nodeID, nodesMap, seedsMap)
	case *transactionv1.Node_Fetch:
		return encodeFetchNode(t.Fetch, nodeID)
	case *transactionv1.Node_Rollback:
		return encodeRollbackNode(t.Rollback, nodeID, nodesMap, seedsMap)
	default:
		return []byte{}
	}
}

func encodeCreateNode(create *transactionv1.Create, nodeID string, seedsMap map[string][]byte) []byte {
	res := []byte(NodeEncodingVersion)
	res = append(res, encodeString(create.LfVersion)...)
	res = append(res, 0x00) // Create node tag

	seed, ok := seedsMap[nodeID]
	if ok {
		res = append(res, 0x01)
		res = append(res, seed...)
	} else {
		res = append(res, 0x00)
	}

	res = append(res, encodeHexString(create.ContractId)...)
	res = append(res, encodeString(create.PackageName)...)
	res = append(res, encodeIdentifier(create.TemplateId)...)
	res = append(res, encodeValue(create.Argument)...)
	res = append(res, encodeRepeated(create.Signatories, encodeString)...)
	res = append(res, encodeRepeated(create.Stakeholders, encodeString)...)
	return res
}

func encodeExerciseNode(exercise *transactionv1.Exercise, nodeID string, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	res := []byte(NodeEncodingVersion)
	res = append(res, encodeString(exercise.LfVersion)...)
	res = append(res, 0x01) // Exercise node tag

	res = append(res, seedsMap[nodeID]...)

	res = append(res, encodeHexString(exercise.ContractId)...)
	res = append(res, encodeString(exercise.PackageName)...)
	res = append(res, encodeIdentifier(exercise.TemplateId)...)
	res = append(res, encodeRepeated(exercise.Signatories, encodeString)...)
	res = append(res, encodeRepeated(exercise.Stakeholders, encodeString)...)
	res = append(res, encodeRepeated(exercise.ActingParties, encodeString)...)
	res = append(res, encodeOptional(exercise.InterfaceId != nil, func() []byte { return encodeIdentifier(exercise.InterfaceId) })...)
	res = append(res, encodeString(exercise.ChoiceId)...)
	res = append(res, encodeValue(exercise.ChosenValue)...)
	res = append(res, encodeBool(exercise.Consuming)...)
	res = append(res, encodeOptional(exercise.ExerciseResult != nil, func() []byte { return encodeValue(exercise.ExerciseResult) })...)
	res = append(res, encodeRepeated(exercise.ChoiceObservers, encodeString)...)

	children := encodeRepeated(exercise.Children, func(childID string) []byte {
		if node, ok := nodesMap[childID]; ok {
			return sha256Sum(encodeNode(node, nodesMap, seedsMap))
		}
		return make([]byte, 32)
	})
	res = append(res, children...)
	return res
}

func encodeFetchNode(fetch *transactionv1.Fetch, nodeID string) []byte {
	res := []byte(NodeEncodingVersion)
	res = append(res, encodeString(fetch.LfVersion)...)
	res = append(res, 0x02) // Fetch node tag
	res = append(res, encodeHexString(fetch.ContractId)...)
	res = append(res, encodeString(fetch.PackageName)...)
	res = append(res, encodeIdentifier(fetch.TemplateId)...)
	res = append(res, encodeRepeated(fetch.Signatories, encodeString)...)
	res = append(res, encodeRepeated(fetch.Stakeholders, encodeString)...)
	res = append(res, encodeOptional(fetch.InterfaceId != nil, func() []byte { return encodeIdentifier(fetch.InterfaceId) })...)
	res = append(res, encodeRepeated(fetch.ActingParties, encodeString)...)
	return res
}

func encodeRollbackNode(rollback *transactionv1.Rollback, nodeID string, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	res := []byte(NodeEncodingVersion)
	res = append(res, 0x03) // Rollback node tag
	children := encodeRepeated(rollback.Children, func(childID string) []byte {
		if node, ok := nodesMap[childID]; ok {
			return sha256Sum(encodeNode(node, nodesMap, seedsMap))
		}
		return make([]byte, 32)
	})
	res = append(res, children...)
	return res
}

func hashMetadata(metadata *interactive.Metadata, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	encoded := encodeMetadata(metadata, nodesMap, seedsMap)
	h := sha256.New()
	h.Write([]byte(PreparedTransactionHashPurpose))
	h.Write(encoded)
	return h.Sum(nil)
}

func encodeMetadata(metadata *interactive.Metadata, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	res := []byte{0x01}
	if metadata.SubmitterInfo != nil {
		res = append(res, encodeRepeated(metadata.SubmitterInfo.ActAs, encodeString)...)
		res = append(res, encodeString(metadata.SubmitterInfo.CommandId)...)
	} else {
		res = append(res, encodeInt32(0)...)
		res = append(res, encodeString("")...)
	}
	res = append(res, encodeString(metadata.TransactionUuid)...)
	res = append(res, encodeInt32(int32(metadata.MediatorGroup))...)
	res = append(res, encodeString(metadata.SynchronizerId)...)

	res = append(res, encodeOptional(metadata.MinLedgerEffectiveTime != nil, func() []byte {
		return encodeInt64(int64(*metadata.MinLedgerEffectiveTime))
	})...)
	res = append(res, encodeOptional(metadata.MaxLedgerEffectiveTime != nil, func() []byte {
		return encodeInt64(int64(*metadata.MaxLedgerEffectiveTime))
	})...)

	res = append(res, encodeInt64(int64(metadata.PreparationTime))...)
	res = append(res, encodeRepeated(metadata.InputContracts, func(c *interactive.Metadata_InputContract) []byte {
		return encodeInputContract(c, nodesMap, seedsMap)
	})...)
	return res
}

func encodeInputContract(contract *interactive.Metadata_InputContract, nodesMap map[string]*interactive.DamlTransaction_Node, seedsMap map[string][]byte) []byte {
	res := encodeInt64(int64(contract.CreatedAt))
	var encodedNode []byte
	switch c := contract.Contract.(type) {
	case *interactive.Metadata_InputContract_V1:
		encodedNode = encodeCreateNode(c.V1, "unused_node_id", map[string][]byte{})
	}
	res = append(res, sha256Sum(encodedNode)...)
	return res
}

func encodeIdentifier(id *apiv2.Identifier) []byte {
	if id == nil {
		return []byte{}
	}
	res := encodeString(id.PackageId)
	res = append(res, encodeRepeated(splitParts(id.ModuleName), encodeString)...)
	res = append(res, encodeRepeated(splitParts(id.EntityName), encodeString)...)
	return res
}

func splitParts(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ".")
}

func encodeValue(v *apiv2.Value) []byte {
	if v == nil {
		return []byte{}
	}
	switch s := v.Sum.(type) {
	case *apiv2.Value_Unit:
		return []byte{0x00}
	case *apiv2.Value_Bool:
		return append([]byte{0x01}, encodeBool(s.Bool)...)
	case *apiv2.Value_Int64:
		return append([]byte{0x02}, encodeInt64(s.Int64)...)
	case *apiv2.Value_Numeric:
		return append([]byte{0x03}, encodeString(s.Numeric)...)
	case *apiv2.Value_Timestamp:
		return append([]byte{0x04}, encodeInt64(s.Timestamp)...)
	case *apiv2.Value_Date:
		return append([]byte{0x05}, encodeInt32(s.Date)...)
	case *apiv2.Value_Party:
		return append([]byte{0x06}, encodeString(s.Party)...)
	case *apiv2.Value_Text:
		return append([]byte{0x07}, encodeString(s.Text)...)
	case *apiv2.Value_ContractId:
		return append([]byte{0x08}, encodeHexString(s.ContractId)...)
	case *apiv2.Value_Optional:
		return append([]byte{0x09}, encodeOptional(s.Optional.Value != nil, func() []byte {
			return encodeValue(s.Optional.Value)
		})...)
	case *apiv2.Value_List:
		return append([]byte{0x0a}, encodeRepeated(s.List.Elements, encodeValue)...)
	case *apiv2.Value_TextMap:
		return append([]byte{0x0b}, encodeRepeated(s.TextMap.Entries, func(e *apiv2.TextMap_Entry) []byte {
			return append(encodeString(e.Key), encodeValue(e.Value)...)
		})...)
	case *apiv2.Value_Record:
		res := []byte{0x0c}
		res = append(res, encodeOptional(s.Record.RecordId != nil, func() []byte {
			return encodeIdentifier(s.Record.RecordId)
		})...)
		res = append(res, encodeRepeated(s.Record.Fields, func(f *apiv2.RecordField) []byte {
			labelPrefix := []byte{0x00}
			if f.Label != "" {
				labelPrefix = append([]byte{0x01}, encodeString(f.Label)...)
			}
			return append(labelPrefix, encodeValue(f.Value)...)
		})...)
		return res
	case *apiv2.Value_Variant:
		res := []byte{0x0d}
		res = append(res, encodeOptional(s.Variant.VariantId != nil, func() []byte {
			return encodeIdentifier(s.Variant.VariantId)
		})...)
		res = append(res, encodeString(s.Variant.Constructor)...)
		res = append(res, encodeValue(s.Variant.Value)...)
		return res
	case *apiv2.Value_Enum:
		res := []byte{0x0e}
		res = append(res, encodeOptional(s.Enum.EnumId != nil, func() []byte {
			return encodeIdentifier(s.Enum.EnumId)
		})...)
		res = append(res, encodeString(s.Enum.Constructor)...)
		return res
	case *apiv2.Value_GenMap:
		return append([]byte{0x0f}, encodeRepeated(s.GenMap.Entries, func(e *apiv2.GenMap_Entry) []byte {
			return append(encodeValue(e.Key), encodeValue(e.Value)...)
		})...)
	}
	return []byte{}
}

// --- Helpers ---

func encodeBytes(v []byte) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(v)))
	return append(b, v...)
}

func encodeString(v string) []byte {
	return encodeBytes([]byte(v))
}

func encodeInt32(v int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(v))
	return b
}

func encodeInt64(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func encodeBool(v bool) []byte {
	if v {
		return []byte{0x01}
	}
	return []byte{0x00}
}

func encodeHexString(v string) []byte {
	b, _ := hex.DecodeString(v)
	return encodeBytes(b)
}

func encodeRepeated[T any](items []T, encodeFn func(T) []byte) []byte {
	var res []byte
	for _, item := range items {
		res = append(res, encodeFn(item)...)
	}
	return append(encodeInt32(int32(len(items))), res...)
}

func encodeOptional(exists bool, encodeFn func() []byte) []byte {
	if exists {
		return append([]byte{0x01}, encodeFn()...)
	}
	return []byte{0x00}
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
