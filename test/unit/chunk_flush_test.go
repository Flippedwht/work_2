package storage_test

import (
	"path/filepath"
	"testing"
	"work_2/internal/core/block"
	"work_2/internal/core/storage"
)

func TestChunkFlushing(t *testing.T) {
	// Setup
	tmpDir := filepath.Join(t.TempDir(), "storage")
	cs, err := storage.NewChunkStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer cs.Close()

	// Test data
	blocks := []*block.Block{
		{Number: 1, Transactions: []block.Transaction{{TXID: "tx1", Amount: 100}}},
		{Number: 2, Transactions: []block.Transaction{{TXID: "tx2", Amount: 200}}},
		{Number: 3, Transactions: []block.Transaction{{TXID: "tx3", Amount: 300}}},
	}

	// Add blocks (total size ~120 bytes)
	for _, b := range blocks {
		if err := cs.AddBlock(b); err != nil {
			t.Fatalf("AddBlock failed: %v", err)
		}
	}

	// Force flush
	if err := cs.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify chunk count
	if cs.GetChunkIDSeq() != 1 {
		t.Errorf("Expected 1 chunk, got %d", cs.GetChunkIDSeq())
	}

	// Verify chunk data
	iter := cs.GetDB().NewIterator(nil, nil)
	defer iter.Release()

	chunkCount := 0
	for iter.Next() {
		key := iter.Key()
		if string(key)[0:6] == "chunk:" {
			chunkCount++
		}
	}
	if chunkCount != 1 {
		t.Errorf("Expected 1 chunk in DB, found %d", chunkCount)
	}
}
