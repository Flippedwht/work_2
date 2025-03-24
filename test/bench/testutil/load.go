package testutil

import (
	"encoding/json"

	"log"
	"os"
	"path/filepath"

	"work_2/internal/core/block"

	eth "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// LoadEthBlocks 加载以太坊RLP格式区块
func LoadEthBlocks(dataDir string) []*eth.Block {
	return loadBlocks(filepath.Join(dataDir, "eth"), decodeEthBlock)
}

// LoadCustomBlocks 加载自定义JSON格式区块
func LoadCustomBlocks(dataDir string) []*block.Block {
	return loadBlocks(filepath.Join(dataDir, "custom"), decodeCustomBlock)
}

func loadBlocks[T any](dir string, decoder func([]byte) (T, error)) []T {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	var blocks []T
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}

		if b, err := decoder(data); err == nil {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func decodeEthBlock(data []byte) (*eth.Block, error) {
	var block eth.Block
	if err := rlp.DecodeBytes(data, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func decodeCustomBlock(data []byte) (*block.Block, error) {
	var b block.Block
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
