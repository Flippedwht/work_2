package bench_test

import (
	"os"
	"path/filepath"
	"testing"

	"work_2/internal/core/storage"
	"work_2/test/bench/testutil"
)

var chunkStorage *storage.ChunkStorage

func init() {
	dbPath := filepath.Join("testdata", "chunkdb")
	os.RemoveAll(dbPath)

	var err error
	chunkStorage, err = storage.NewChunkStorage(dbPath)
	if err != nil {
		panic(err)
	}
}

func BenchmarkChunkWrite(b *testing.B) {
	blocks := testutil.LoadCustomBlocks(filepath.Join("test", "bench", "data"))
	batchSize := 100
	b.ResetTimer()

	for i := 0; i < b.N; i += batchSize {
		end := i + batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		chunkStorage.WriteBlocks(blocks[i:end])
	}
}
