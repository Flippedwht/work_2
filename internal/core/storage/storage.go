package storage

type StorageInterface interface {
	GetBlockByNumber(num uint64) ([]byte, error)
	//GetBlockByTxHash(txID string) ([]byte, error) // 新增方法
	DBPath() string
	Close() error
}
