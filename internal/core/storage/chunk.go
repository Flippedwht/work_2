package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"work_2/internal/core/block"
	"work_2/internal/core/metrics"
	"work_2/internal/core/skiplist"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	ChunkSizeThreshold = 2 * 1024 * 1024 // 2MB
	CacheSize          = 128             // 缓存128个最近访问的chunk
	BlockCacheSize     = 1024            // 额外缓存1024个单独的区块
	TxCacheSize        = 2048            // 额外缓存2048个交易
)

// **缓存交易和所在区块**
type TxCacheEntry struct {
	Tx    *block.Transaction
	Block *block.Block
}

// 从交易 ID 到它所在区块号以及在区块内的索引的映射 flush的时候用到
// 即保存交易在 Chunk 内的位置信息
type TxOffset struct {
	BlockNumber uint64 // 交易所在的区块号
	TxIndex     int    // 在该区块 Transactions 数组中的索引
}

type ChunkStorage struct {
	db         *leveldb.DB
	path       string
	mu         sync.RWMutex
	currentBuf []*block.Block //当前内存缓冲区
	chunkIDSeq uint64
	cache      *lru.Cache[uint64, *Chunk]        // 使用LRU缓存最近访问的chunk
	blockCache *lru.Cache[uint64, *block.Block]  // LRU缓存最近访问的Block
	txCache    *lru.Cache[string, *TxCacheEntry] // 缓存交易
	skipList   *skiplist.SkipList                // 跳表
}

type Chunk struct {
	ID         uint64                  `json:"id"`
	StartBlock uint64                  `json:"start"`      // 包含的起始区块号
	EndBlock   uint64                  `json:"end"`        // 结束区块号
	CreatedAt  int64                   `json:"created"`    // 时间戳
	Data       []byte                  `json:"data"`       // 序列化的区块数据
	Index      map[uint64]int          `json:"index"`      // 区块号->数据偏移量
	Blocks     map[uint64]*block.Block `json:"blocks"`     // 存储每个区块 JSON
	TxOffsets  map[string]TxOffset     `json:"tx_offsets"` // 交易索引映射
}

// 初始化存储系统
func NewChunkStorage(path string) (*ChunkStorage, error) {
	fullPath := filepath.Join(path, "chunks")
	db, err := leveldb.OpenFile(fullPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb: %v", err)
	}

	chunkCache, err := lru.New[uint64, *Chunk](CacheSize)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create chunk cache: %v", err)
	}

	blockCache, err := lru.New[uint64, *block.Block](BlockCacheSize)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create block cache: %v", err)
	}

	txCache, err := lru.New[string, *TxCacheEntry](TxCacheSize)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tx cache: %v", err)
	}

	return &ChunkStorage{
		db:         db,
		path:       fullPath,
		currentBuf: make([]*block.Block, 0),
		chunkIDSeq: 0,
		cache:      chunkCache,
		blockCache: blockCache,
		txCache:    txCache,
		skipList:   skiplist.NewSkipList(16, 0.5),
	}, nil
}

// ======================== 核心存储逻辑 ========================
func (cs *ChunkStorage) AddBlock(b *block.Block) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.currentBuf = append(cs.currentBuf, b)

	if cs.shouldFlush() {
		return cs.Flush()
	}
	return nil
}

func (cs *ChunkStorage) shouldFlush() bool {
	totalSize := 0
	for _, b := range cs.currentBuf {
		totalSize += b.Size()
	}
	return totalSize >= ChunkSizeThreshold
}

// 将缓冲区数据持久化为Chunk
// 1、创建Chunk结构并填充元数据
// 2、遍历缓冲区中的区块：
//
//	建立区块号到ChunkID的索引
//	构建交易ID到区块位置的映射
//
// 3、序列化Chunk并存入LevelDB
// 4、更新chunkIDSeq序列号
// 5、清空currentBuf
func (cs *ChunkStorage) Flush() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if len(cs.currentBuf) == 0 {
		return nil
	}

	chunk := Chunk{
		ID:         cs.chunkIDSeq,
		StartBlock: cs.currentBuf[0].Number,
		EndBlock:   cs.currentBuf[len(cs.currentBuf)-1].Number,
		CreatedAt:  time.Now().Unix(),
		Blocks:     make(map[uint64]*block.Block),
		TxOffsets:  make(map[string]TxOffset),
	}

	for _, b := range cs.currentBuf {
		chunk.Blocks[b.Number] = b
		blockIndexKey := []byte(fmt.Sprintf("block_index:%d", b.Number))
		cs.db.Put(blockIndexKey, []byte(fmt.Sprintf("%d", chunk.ID)), nil)

		// 遍历区块中的交易，构建交易索引
		for i, tx := range b.Transactions {
			// 存储 LevelDB 的 tx_index 仍可以保留，但这里我们同时记录更详细的信息到 chunk.TxOffsets
			txKey := []byte(fmt.Sprintf("tx_index:%s", tx.TXID))
			cs.db.Put(txKey, []byte(fmt.Sprintf("%d", chunk.ID)), nil)

			chunk.TxOffsets[tx.TXID] = TxOffset{
				BlockNumber: b.Number,
				TxIndex:     i,
			}
		}
	}

	chunkData, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal chunk failed: %w", err)
	}

	chunkKey := []byte(fmt.Sprintf("chunk:%d", chunk.ID))
	if err := cs.db.Put(chunkKey, chunkData, nil); err != nil {
		return fmt.Errorf("store chunk failed: %w", err)
	}

	cs.currentBuf = nil
	cs.chunkIDSeq++
	return nil
}

// ======================== 查询接口 ========================
// GetChunkIDSeq 返回当前的 chunkIDSeq 值
func (cs *ChunkStorage) GetChunkIDSeq() uint64 {
	return cs.chunkIDSeq
}

// GetDB 返回当前的数据库实例
func (cs *ChunkStorage) GetDB() *leveldb.DB {
	return cs.db
}

// 添加以下方法到 ChunkStorage 结构体

// GetChunkCount 获取已生成的大块总数
func (cs *ChunkStorage) GetChunkCount() uint64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.chunkIDSeq
}

// GetBlockByNumber 通过区块号查询原始数据
func (cs *ChunkStorage) GetBlockByNumber(num uint64) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	metrics.LevelDBReadOps.Inc()
	// **0. 优先查询跳表（热点区块）**
	if node := cs.skipList.Search(num); node != nil {
		// 跳表命中，返回该节点存储的区块
		return json.Marshal(node.Block)
	}

	// **1. 先查 blockCache**
	if block, ok := cs.blockCache.Get(num); ok {
		// 同时将查到的 Block 插入跳表，以便下次直接命中
		cs.skipList.Insert(num, block)
		return json.Marshal(block)
	}

	// **2. 查询LevelDB的区块索引获取ChunkID**
	indexKey := []byte(fmt.Sprintf("block_index:%d", num))
	chunkIDBytes, err := cs.db.Get(indexKey, nil)
	if err != nil {
		return nil, fmt.Errorf("block index lookup failed: %w", err)
	}
	chunkID, _ := strconv.ParseUint(string(chunkIDBytes), 10, 64)

	// **3. 加载 Chunk**
	chunk, err := cs.loadChunk(chunkID)
	if err != nil {
		return nil, err
	}

	// **4. 提取 Block 并缓存**
	block, exists := chunk.Blocks[num]
	if !exists {
		return nil, errors.New("block not found in chunk")
	}
	cs.blockCache.Add(num, block) // 加入 block 缓存
	// 同时将该 Block 插入跳表
	cs.skipList.Insert(num, block)

	return json.Marshal(block)
}

// 通过交易索引查找
func (cs *ChunkStorage) GetBlockByTxHash(txHash string) ([]byte, []byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// 1. 优先查询 txCache
	if entry, ok := cs.txCache.Get(txHash); ok {
		blockData, _ := json.Marshal(entry.Block) // 交易所在区块
		txData, _ := json.Marshal(entry.Tx)       // 交易本身
		return blockData, txData, nil
	}

	// 2. 通过 LevelDB 索引查找该交易所属的 ChunkID
	txKey := []byte(fmt.Sprintf("tx_index:%s", txHash))
	chunkIDBytes, err := cs.db.Get(txKey, nil)
	if err != nil {
		log.Printf("查询交易 %s 的索引失败: %v", txHash, err)
		return nil, nil, fmt.Errorf("tx index lookup failed: %w", err)
	}
	chunkID, _ := strconv.ParseUint(string(chunkIDBytes), 10, 64)

	// 3. 加载对应的 Chunk
	chunk, err := cs.loadChunk(chunkID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load chunk: %w", err)
	}

	// 4. 利用 Chunk 内预构建的交易偏移索引，直接定位目标交易
	offset, ok := chunk.TxOffsets[txHash]
	if !ok {
		return nil, nil, errors.New("transaction not found in tx_offsets")
	}

	targetBlock, exists := chunk.Blocks[offset.BlockNumber]
	if !exists {
		return nil, nil, errors.New("block not found in chunk")
	}

	// 校验交易索引有效性
	if offset.TxIndex < 0 || offset.TxIndex >= len(targetBlock.Transactions) {
		return nil, nil, errors.New("invalid transaction index")
	}
	tx := targetBlock.Transactions[offset.TxIndex]

	// 5. 缓存查找到的交易信息，以便下次直接命中
	cs.txCache.Add(txHash, &TxCacheEntry{
		Tx:    &tx,
		Block: targetBlock,
	})
	// 同时，将目标 Block 插入跳表，方便后续区块查询
	cs.skipList.Insert(targetBlock.Number, targetBlock)

	blockData, _ := json.Marshal(targetBlock)
	txData, _ := json.Marshal(tx)
	return blockData, txData, nil
}

// **加载 Chunk，并将 Block 逐个存入缓存 缓存交易**
func (cs *ChunkStorage) loadChunk(chunkID uint64) (*Chunk, error) {
	// 先检查缓存中的 Chunk
	if cachedChunk, ok := cs.cache.Get(chunkID); ok {
		return cachedChunk, nil
	}

	chunkKey := []byte(fmt.Sprintf("chunk:%d", chunkID))
	chunkData, err := cs.db.Get(chunkKey, nil)
	if err != nil {
		log.Printf("查询 chunk 失败: %v", err)
		return nil, fmt.Errorf("chunk not found: %w", err)
	}

	var chunk Chunk
	if err := json.Unmarshal(chunkData, &chunk); err != nil {
		log.Printf("解析 chunk 失败: %v", err)
		return nil, fmt.Errorf("chunk unmarshal failed: %w", err)
	}

	// 缓存加载的 Chunk
	cs.cache.Add(chunkID, &chunk)

	// 缓存 Chunk 中所有 Block
	for blockNum, blk := range chunk.Blocks {
		cs.blockCache.Add(blockNum, blk)
		cs.skipList.Insert(blockNum, blk)
	}

	// 不再遍历并缓存每个交易到 txCache，查询时直接利用 TxOffsets 定位交易

	return &chunk, nil
}

// ======================== 工具方法 ========================
// Close 释放数据库资源
func (cs *ChunkStorage) Close() error {
	return cs.db.Close()
}

func (cs *ChunkStorage) WriteChunk(data []byte) error {
	return cs.db.Put([]byte("chunk0"), data, nil)
}

// internal/core/storage/chunk.go
func (cs *ChunkStorage) WriteBlock(b *block.Block) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	key := []byte(fmt.Sprintf("block:%d", b.Number))
	fmt.Printf("正在写入区块 %d → 大小 %d 字节\n", b.Number, len(data)) // 添加日志
	return cs.db.Put(key, data, nil)
}

func (cs *ChunkStorage) DBPath() string {
	return cs.path
}

// 批量写入方法
func (cs *ChunkStorage) WriteBlocks(blocks []*block.Block) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 合并到当前缓冲区
	cs.currentBuf = append(cs.currentBuf, blocks...)

	if cs.shouldFlush() {
		return cs.Flush()
	}
	return nil
}

// 测试数据加载方法
func LoadTestBlocksFromDir(dir string) ([]*block.Block, error) {
	files, _ := os.ReadDir(dir)
	var blocks []*block.Block

	for _, f := range files {
		data, _ := os.ReadFile(filepath.Join(dir, f.Name()))
		var b block.Block
		if err := json.Unmarshal(data, &b); err == nil {
			blocks = append(blocks, &b)
		}
	}
	return blocks, nil
}
