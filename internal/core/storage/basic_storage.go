package storage

import (
	"encoding/json"
	"fmt"

	"work_2/internal/core/block"

	"github.com/syndtr/goleveldb/leveldb"
)

// BasicStorage 实现了最原始的 LevelDB 存储方案。
// 每个区块以单个 key/value 形式存储，数据格式为 JSON。
type BasicStorage struct {
	db   *leveldb.DB
	path string
}

// NewBasicStorage 打开或创建一个 LevelDB 数据库
func NewBasicStorage(dbPath string) (*BasicStorage, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	return &BasicStorage{
		db:   db,
		path: dbPath,
	}, nil
}

// Close 关闭数据库连接
func (s *BasicStorage) Close() error {
	return s.db.Close()
}

// AddBlock 将一个区块数据序列化后写入数据库
func (s *BasicStorage) AddBlock(b *block.Block) error {
	key := []byte(fmt.Sprintf("block_%d", b.Number))
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return s.db.Put(key, data, nil)
}

// GetBlockByNumber 根据区块号获取对应的数据
func (s *BasicStorage) GetBlockByNumber(num uint64) ([]byte, error) {
	key := []byte(fmt.Sprintf("block_%d", num))
	return s.db.Get(key, nil)
}

// DBPath 返回数据库所在的路径
func (s *BasicStorage) DBPath() string {
	return s.path
}
