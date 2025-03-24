package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"work_2/internal/core/storage"
)

func TestNewStorage(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "testdb")
	defer os.RemoveAll(tmpDir)

	s, err := storage.NewChunkStorage(tmpDir)
	if err != nil {
		t.Errorf("创建存储失败: %v", err)
	}
	s.Close()
}

func TestWriteChunk(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "chunk-test")
	defer os.RemoveAll(tmpDir)

	s, err := storage.NewChunkStorage(tmpDir)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer s.Close()

	if err := s.WriteChunk([]byte("test data")); err != nil {
		t.Errorf("写入失败: %v", err)
	}
}
