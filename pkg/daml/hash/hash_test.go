package hash

import (
	"encoding/hex"
	"testing"

	interactive "buf-lib-poc/pkg/daml/proto/com/daml/ledger/api/v2/interactive"
)

func TestHashPreparedTransaction_Deterministic(t *testing.T) {
	tx := &interactive.PreparedTransaction{
		Transaction: &interactive.DamlTransaction{
			Version: "1",
			Roots:   []string{"0"},
			Nodes: []*interactive.DamlTransaction_Node{
				{
					NodeId: "0",
				},
			},
		},
		Metadata: &interactive.Metadata{
			SubmitterInfo: &interactive.Metadata_SubmitterInfo{
				ActAs:     []string{"party1"},
				CommandId: "cmd1",
			},
			TransactionUuid: "uuid1",
			SynchronizerId:  "sync1",
		},
	}

	h1, err := HashPreparedTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to hash: %v", err)
	}

	h2, err := HashPreparedTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to hash: %v", err)
	}

	if hex.EncodeToString(h1) != hex.EncodeToString(h2) {
		t.Errorf("Hash is not deterministic: %x != %x", h1, h2)
	}

	t.Logf("Hash: %x", h1)
}
