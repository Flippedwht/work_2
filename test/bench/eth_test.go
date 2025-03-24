package bench_test

import (
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"

	"work_2/test/bench/testutil"
)

var ethDB *leveldb.Database

func init() {
	dbPath := filepath.Join("testdata", "ethdb")
	var err error
	ethDB, err = leveldb.New(dbPath, 256, 0, "", false)
	if err != nil {
		panic(err)
	}
}

func BenchmarkEthWrite(b *testing.B) {
	blocks := testutil.LoadEthBlocks(filepath.Join("test", "bench", "data"))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		block := blocks[i%len(blocks)]
		batch := ethDB.NewBatch()
		rawdb.WriteBlock(batch, block)
		rawdb.WriteCanonicalHash(batch, block.Hash(), block.NumberU64())
		batch.Write()
	}
}
