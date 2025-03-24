package integration_test

import (
	"path/filepath"
	"testing"
	"work_2/internal/core/loader"
	"work_2/internal/core/storage"
)

func TestJsonLoading(t *testing.T) {
	// 1. 定义关键路径（基于项目根目录）
	projectRoot := "E:/VScode_project/work_2" // 替换为你的实际路径
	rawBlocksDir := filepath.Join(projectRoot, "data", "raw_blocks")
	storageDir := filepath.Join(projectRoot, "data", "chunk_storage")

	// 2. 加载区块数据
	blocks, err := loader.LoadBlocksFromDir(rawBlocksDir)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	t.Logf("成功加载 %d 个区块", len(blocks))

	// 3. 初始化存储（使用独立的存储目录）
	s, err := storage.NewChunkStorage(storageDir)
	if err != nil {
		t.Fatalf("存储初始化失败: %v", err)
	}
	defer s.Close()

	// 4. 写入所有区块
	successCount := 0
	for _, b := range blocks {
		if err := s.WriteBlock(b); err != nil {
			t.Errorf("写入区块%d失败: %v", b.Number, err)
		} else {
			successCount++
		}
	}
	t.Logf("成功写入 %d/%d 个区块", successCount, len(blocks))
}
