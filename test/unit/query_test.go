package storage_test

import (
	"encoding/json"
	"testing"
	"work_2/internal/core/block"
	"work_2/internal/core/storage"
)

func TestTxQuery(t *testing.T) {
	// 初始化存储
	tmpDir := t.TempDir()
	s, _ := storage.NewChunkStorage(tmpDir)
	defer s.Close()

	// 准备测试数据
	txHash := "tx_123"
	testBlock := &block.Block{
		Number: 1,
		Transactions: []block.Transaction{
			{TXID: txHash, Amount: 100},
		},
	}

	// 写入数据
	if err := s.AddBlock(testBlock); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	// 测试查询
	data, err := s.GetBlockByTxHash(txHash)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	// 验证数据
	var result block.Block
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Number != 1 || len(result.Transactions) != 1 {
		t.Errorf("数据不一致: %+v", result)
	}
}
