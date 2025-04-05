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
	UserHotThreshold   = 10              // 设定查询次数超过10次的用户为热用户
)

// UserProfile 结构体
type UserProfile struct {
	UserID        string
	QueryCount    int             // 总查询次数（历史累计）
	LastQueryTime time.Time       // 最近一次查询时间
	QueryHistory  map[string]int  // 滑动窗口（小时统计）
	DailyPattern  map[int]float64 // 过去 7 天每小时平均查询次数（SMA）
	QuerySequence []string        // 记录用户最近查询的交易 ID
}

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

	// **1. 查询 TxCache（LRU 缓存）**
	if entry, ok := cs.txCache.Get(txHash); ok {
		cs.UpdateUserProfile(entry.Tx.From, txHash)

		blockData, err := json.Marshal(entry.Block)
		if err != nil {
			return nil, nil, fmt.Errorf("block marshal failed: %w", err)
		}
		txData, err := json.Marshal(entry.Tx)
		if err != nil {
			return nil, nil, fmt.Errorf("tx marshal failed: %w", err)
		}
		return blockData, txData, nil
	}

	// **2. 查询 LevelDB 获取 ChunkID**
	txKey := []byte(fmt.Sprintf("tx_index:%s", txHash))
	chunkIDBytes, err := cs.db.Get(txKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("tx index lookup failed: %w", err)
	}
	chunkID, _ := strconv.ParseUint(string(chunkIDBytes), 10, 64)

	// **3. 加载 Chunk**
	chunk, err := cs.loadChunk(chunkID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load chunk: %w", err)
	}

	// **4. 利用 Chunk 内的交易偏移索引，直接查找交易**
	offset, ok := chunk.TxOffsets[txHash]
	if !ok {
		return nil, nil, errors.New("transaction not found in tx_offsets")
	}

	// **5. 先查询跳表，看看是否已经缓存了这个区块**
	if node := cs.skipList.Search(offset.BlockNumber); node != nil {
		// **跳表命中**
		blockData, err := json.Marshal(node.Block)
		if err != nil {
			return nil, nil, fmt.Errorf("block marshal failed: %w", err)
		}
		tx := node.Block.Transactions[offset.TxIndex]
		txData, err := json.Marshal(tx)
		if err != nil {
			return nil, nil, fmt.Errorf("tx marshal failed: %w", err)
		}
		// **缓存交易**
		cs.txCache.Add(txHash, &TxCacheEntry{Tx: &tx, Block: node.Block})
		return blockData, txData, nil
	}

	// **6. 跳表未命中，从 Chunk 加载区块**
	targetBlock, exists := chunk.Blocks[offset.BlockNumber]
	if !exists {
		return nil, nil, errors.New("block not found in chunk")
	}

	// **7. 校验交易索引**
	if offset.TxIndex < 0 || offset.TxIndex >= len(targetBlock.Transactions) {
		return nil, nil, errors.New("invalid transaction index")
	}
	tx := targetBlock.Transactions[offset.TxIndex]

	// **8. 缓存查找到的交易 & 区块**
	cs.txCache.Add(txHash, &TxCacheEntry{Tx: &tx, Block: targetBlock})
	cs.skipList.Insert(targetBlock.Number, targetBlock) // **插入跳表**

	// **9. 更新用户查询行为**
	cs.UpdateUserProfile(tx.From, txHash)

	// **10. 触发 Lookahead 预加载**
	if cs.IsHotUser(tx.From) || cs.IsCyclicHotUser(tx.From) {
		cs.PreloadUserTransactions(tx.From)
	}

	// **11. 预测用户下一次可能查询的交易**
	if predictedTx := cs.PredictNextQuery(tx.From); predictedTx != "" {
		fmt.Printf("预测用户 %s 可能查询: %s\n", tx.From, predictedTx)
		cs.PreloadUserTransactions(predictedTx) // 预加载预测的交易
	}

	// **12. 返回交易 & 区块数据**
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

// 更新用户查询行为
func (cs *ChunkStorage) UpdateUserProfile(userID, txHash string) {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))

	data, err := cs.db.Get(key, nil)
	var profile UserProfile
	if err == nil {
		_ = json.Unmarshal(data, &profile)
	} else {
		profile = UserProfile{
			UserID:        userID,
			QueryCount:    0,
			QueryHistory:  make(map[string]int),
			DailyPattern:  make(map[int]float64),
			QuerySequence: []string{},
		}
	}

	// **记录当前小时查询次数**
	currentHour := time.Now().Format("2006-01-02 15")
	profile.QueryHistory[currentHour]++
	profile.QueryCount++
	profile.LastQueryTime = time.Now()

	// **移除超出滑动窗口范围的数据**
	windowSize := 24 // 24 小时
	profile = pruneOldHistory(profile, windowSize)

	// **计算过去 7 天的 SMA 识别周期性查询**
	profile = computeDailyPattern(profile, 7)

	// **更新用户查询序列（最多存 10 次，FIFO 方式）**
	profile.QuerySequence = append(profile.QuerySequence, txHash)
	if len(profile.QuerySequence) > 10 {
		profile.QuerySequence = profile.QuerySequence[1:] // 只保留最近 10 条查询
	}

	// **存入 LevelDB**
	data, _ = json.Marshal(profile)
	_ = cs.db.Put(key, data, nil)

	// **调用马尔可夫预测，看看是否能预测下一个查询**
	if predictedTx := cs.PredictNextQuery(userID); predictedTx != "" {
		fmt.Printf("🔮 预测用户 %s 可能查询: %s\n", userID, predictedTx)
	}
}

// 计算周期性查询模式 - SMA
func computeDailyPattern(profile UserProfile, days int) UserProfile {
	hourlyCounts := make(map[int][]int) // 记录每个小时的查询次数

	// 遍历历史查询数据
	for timestamp, count := range profile.QueryHistory {
		t, err := time.Parse("2006-01-02 15", timestamp)
		if err == nil {
			hour := t.Hour()
			hourlyCounts[hour] = append(hourlyCounts[hour], count)
		}
	}

	// 计算每个小时的平均查询次数
	for hour, counts := range hourlyCounts {
		total := 0
		for _, c := range counts {
			total += c
		}
		profile.DailyPattern[hour] = float64(total) / float64(len(counts)) // 计算均值
	}

	return profile
}

// 移除超出时间窗口的数据
func pruneOldHistory(profile UserProfile, windowSize int) UserProfile {
	now := time.Now()
	threshold := now.Add(-time.Duration(windowSize) * time.Hour) // 计算窗口起点

	newHistory := make(map[string]int)
	for timestamp, count := range profile.QueryHistory {
		t, err := time.Parse("2006-01-02 15", timestamp)
		if err == nil && t.After(threshold) {
			newHistory[timestamp] = count
		}
	}

	profile.QueryHistory = newHistory
	return profile
}

// 判断短期热用户 - 滑动窗口
func (cs *ChunkStorage) IsHotUser(userID string) bool {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))

	data, err := cs.db.Get(key, nil)
	if err != nil {
		return false
	}

	var profile UserProfile
	_ = json.Unmarshal(data, &profile)

	// 计算最近 `windowSize` 小时内的查询总数
	windowSize := 24
	totalRecentQueries := 0
	now := time.Now()
	threshold := now.Add(-time.Duration(windowSize) * time.Hour)

	for timestamp, count := range profile.QueryHistory {
		t, err := time.Parse("2006-01-02 15", timestamp)
		if err == nil && t.After(threshold) {
			totalRecentQueries += count
		}
	}

	// 如果最近 24 小时内查询次数 >= 10，则判定为热用户
	return totalRecentQueries >= UserHotThreshold
}

// 判断周期性热用户 - SMA
func (cs *ChunkStorage) IsCyclicHotUser(userID string) bool {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))

	data, err := cs.db.Get(key, nil)
	if err != nil {
		return false
	}

	var profile UserProfile
	_ = json.Unmarshal(data, &profile)

	// 获取当前小时
	currentHour := time.Now().Hour()

	// 如果当前小时的平均查询次数高于阈值，则判定为周期性热用户
	cyclicThreshold := 5.0 // 自定义阈值
	return profile.DailyPattern[currentHour] >= cyclicThreshold
}

// PreloadUserTransactions 预加载用户交易 触发 Lookahead 预加载
func (cs *ChunkStorage) PreloadUserTransactions(userID string) {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))

	data, err := cs.db.Get(key, nil)
	if err != nil {
		return
	}

	var profile UserProfile
	_ = json.Unmarshal(data, &profile)

	// **获取当前小时**
	currentHour := time.Now().Hour()

	// **如果当前小时是用户的历史高峰时段，触发 Lookahead**
	if profile.DailyPattern[currentHour] >= 5 {
		iter := cs.db.NewIterator(nil, nil)
		defer iter.Release()

		count := 0 // 限制最多预加载 10 笔交易
		for iter.Next() {
			keyStr := string(iter.Key())
			if len(keyStr) > 9 && keyStr[:9] == "tx_index:" {
				txHash := keyStr[9:]
				txData := cs.GetTransaction(txHash)
				if txData != nil {
					cs.txCache.Add(txHash, txData)
					count++
					if count >= 10 { // 只预加载最近 10 笔交易
						break
					}
				}
			}
		}
	}
}

// GetTransaction 通过交易哈希获取交易
func (cs *ChunkStorage) GetTransaction(txHash string) *TxCacheEntry {
	txKey := []byte(fmt.Sprintf("tx_index:%s", txHash))
	chunkIDBytes, err := cs.db.Get(txKey, nil)
	if err != nil {
		return nil
	}
	chunkID, _ := strconv.ParseUint(string(chunkIDBytes), 10, 64)

	chunk, err := cs.loadChunk(chunkID)
	if err != nil {
		return nil
	}

	offset, ok := chunk.TxOffsets[txHash]
	if !ok {
		return nil
	}

	block := chunk.Blocks[offset.BlockNumber]
	tx := block.Transactions[offset.TxIndex]

	return &TxCacheEntry{Tx: &tx, Block: block}
}

// 马尔科夫链预测
func (cs *ChunkStorage) PredictNextQuery(userID string) string {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))
	data, err := cs.db.Get(key, nil)
	if err != nil {
		return ""
	}

	var profile UserProfile
	_ = json.Unmarshal(data, &profile)

	// **如果查询历史不足 3 次，不进行预测**
	if len(profile.QuerySequence) < 3 {
		return ""
	}

	// **取最近 2 笔查询**
	lastTx2 := profile.QuerySequence[len(profile.QuerySequence)-2]
	lastTx3 := profile.QuerySequence[len(profile.QuerySequence)-3]

	// **遍历 QuerySequence，找 (Tx3, Tx2) → 预测的 TxX**
	for i := 0; i < len(profile.QuerySequence)-3; i++ {
		if profile.QuerySequence[i] == lastTx3 && profile.QuerySequence[i+1] == lastTx2 {
			return profile.QuerySequence[i+2] // 预测的下一个交易
		}
	}

	return ""
}
