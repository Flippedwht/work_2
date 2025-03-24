// test/bench/read_test.go
package bench_test

import (
	"fmt"
	"math/rand"
	"testing"
	"work_2/internal/core/storage"
)

var (
	testStorage *storage.ChunkStorage
	testTxIDs   []string
)

func init() {
	var err error
	testStorage, err = storage.NewChunkStorage("bench_data")
	if err != nil {
		panic(err)
	}

	// 预加载测试交易ID
	testTxIDs = make([]string, 1000)
	for i := range testTxIDs {
		testTxIDs[i] = fmt.Sprintf("tx%d", i*100)
	}
}

func BenchmarkBlockQuery(b *testing.B) {
	b.ReportAllocs() // 确保报告内存分配
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		num := uint64(rand.Intn(100000) + 1)
		_, _ = testStorage.GetBlockByNumber(num)
	}
}

func BenchmarkTxQuery(b *testing.B) {
	b.ReportAllocs() // 确保报告内存分配
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := testTxIDs[rand.Intn(len(testTxIDs))]
		_, _ = testStorage.GetBlockByTxHash(tx)
	}
}
