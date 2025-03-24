// test/bench/write_test.go
package bench_test

import (
	"fmt"
	"testing"
	"work_2/internal/core/block"
	"work_2/internal/core/storage"
)

func BenchmarkWriteBlocks(b *testing.B) {
	s, err := storage.NewChunkStorage("bench_write")
	if err != nil {
		panic(err)
	}
	defer s.Close()

	b.ReportAllocs() // 确保报告内存分配
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := &block.Block{
			Number: uint64(i),
			Transactions: []block.Transaction{
				{TXID: fmt.Sprintf("tx%d", i), Amount: 100},
			},
		}
		_ = s.AddBlock(block)
	}
	_ = s.Flush() // 确保刷新到存储
}
