package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/syndtr/goleveldb/leveldb"
)

// 交易结构
type Transaction struct {
	TXID   string `json:"txid"`
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Gas    uint64 `json:"gas"`
}

// 区块头结构
type BlockHeader struct {
	ParentHash       string `json:"parent_hash"`
	BlockNumber      uint64 `json:"block_number"`
	Timestamp        uint64 `json:"timestamp"`
	TransactionsRoot string `json:"transactions_root"`
}

// 区块结构
type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
}

const (
	NumBlocks   = 10000 // 生成 10 万个区块
	FlushPeriod = 1000  // 每 1000 个区块输出进度
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// 创建 CPU Profile 文件
	cpuProfile, err := os.Create("cpu_basic.prof")
	if err != nil {
		log.Fatal("无法创建 CPU Profile 文件:", err)
	}
	defer cpuProfile.Close()

	// 开启 CPU Profiling
	pprof.StartCPUProfile(cpuProfile)
	defer pprof.StopCPUProfile()

	// 打开 LevelDB
	dbPath := "E:/VScode_project/work_2/bench_data_basic"
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		log.Fatalf("无法打开 LevelDB: %v", err)
	}
	defer db.Close()

	// 创建存储目录
	blockchainDir := "E:/VScode_project/work_2/blockchain"
	os.MkdirAll(blockchainDir, os.ModePerm)
	blockNumberDir := filepath.Join(blockchainDir, "block_number")
	os.MkdirAll(blockNumberDir, os.ModePerm)
	blockHeaderDir := filepath.Join(blockchainDir, "block_header")
	os.MkdirAll(blockHeaderDir, os.ModePerm)

	// **记录初始内存占用**
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)
	startMemory := float64(startMem.Alloc) / (1024 * 1024)

	// **开始记录时间**
	startTime := time.Now()
	writeStartTime := time.Now()
	totalSize := int64(0)

	fmt.Println("开始生成基础版本测试数据...")

	// 启动 CPU 监控协程
	go monitorCPUUsage()
	go monitorMemoryUsage()

	var parentHash string = "0x0"

	// 生成 NumBlocks 个区块
	for i := uint64(1); i <= NumBlocks; i++ {
		txCount := 100
		txs := make([]Transaction, txCount)
		txHashes := make([]string, txCount)

		// 创建一个文件来保存当前区块的交易哈希
		var blockHeaderFile *os.File
		var blockHeaderFilePath string

		for j := 0; j < txCount; j++ {
			tx := Transaction{
				From:   fmt.Sprintf("user_%d", rand.Intn(100)),
				To:     fmt.Sprintf("user_%d", rand.Intn(100)),
				Amount: uint64(rand.Intn(10000)),
				Gas:    uint64(rand.Intn(1000)),
			}

			// **计算交易哈希**
			txHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%d", tx.From, tx.To, tx.Amount, i)))
			txHashStr := hex.EncodeToString(txHash[:])
			tx.TXID = txHashStr
			txs[j] = tx
			txHashes[j] = txHashStr

			// **存储交易数据**
			txData, _ := json.Marshal(tx)
			txKey := fmt.Sprintf("tx:%s", txHashStr)
			db.Put([]byte(txKey), txData, nil)

			// **存储交易索引**
			txIndexKey := fmt.Sprintf("txIndex:%s", txHashStr)
			txIndexValue := fmt.Sprintf("%s:%d", parentHash, j)
			db.Put([]byte(txIndexKey), []byte(txIndexValue), nil)
		}

		// 计算交易根哈希
		transactionsRoot := sha256.Sum256([]byte(fmt.Sprintf("%v", txHashes)))

		// 生成区块头
		header := BlockHeader{
			ParentHash:       parentHash,
			BlockNumber:      i,
			Timestamp:        uint64(time.Now().Unix()),
			TransactionsRoot: hex.EncodeToString(transactionsRoot[:]),
		}

		// 生成区块
		b := Block{
			Header:       header,
			Transactions: txs,
		}

		// **计算区块大小**
		blockData, _ := json.Marshal(b)
		blockSize := int64(len(blockData))
		totalSize += blockSize

		// **计算区块哈希**
		blockHash := sha256.Sum256(blockData)
		blockHashStr := hex.EncodeToString(blockHash[:])

		// **存入 LevelDB**
		blockKey := fmt.Sprintf("block:%s", blockHashStr)
		db.Put([]byte(blockKey), blockData, nil)

		// **存储区块索引**
		db.Put([]byte(fmt.Sprintf("blockIndex:%d", i)), []byte(blockHashStr), nil)
		// **将区块号和区块头哈希写入文件**
		blockNumberFilePath := filepath.Join(blockNumberDir, "block_number.csv")
		file, err := os.OpenFile(blockNumberFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("无法打开区块号文件: %v", err)
		}
		defer file.Close()

		fmt.Fprintf(file, "%d,%s\n", i, blockHashStr)

		// **将交易哈希写入以区块头哈希命名的文件**
		blockHeaderFilePath = filepath.Join(blockHeaderDir, fmt.Sprintf("%s.csv", blockHashStr))
		blockHeaderFile, err = os.Create(blockHeaderFilePath)
		if err != nil {
			log.Fatalf("无法创建区块头文件: %v", err)
		}
		defer blockHeaderFile.Close()

		for _, txHash := range txHashes {
			fmt.Fprintf(blockHeaderFile, "%s\n", txHash)
		}

		// 记录最新区块哈希
		parentHash = blockHashStr

		if i%FlushPeriod == 0 {
			fmt.Printf("已生成 %d 个区块\n", i)
		}
	}

	// **统计结果**
	writeElapsed := time.Since(writeStartTime)
	elapsed := time.Since(startTime)
	tps := float64(NumBlocks) / elapsed.Seconds()
	throughput := float64(totalSize) / (1024 * 1024) / elapsed.Seconds() // MB/s
	cpuUsage := getCPUUsage()

	// **记录最终内存占用**
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)
	endMemory := float64(endMem.Alloc) / (1024 * 1024)

	fmt.Println("\n==================== 基础版本测试结果 ====================")
	fmt.Printf(" 总运行时间: %v\n", elapsed)
	fmt.Printf(" 数据写入时间: %v\n", writeElapsed)
	fmt.Printf(" 数据总大小: %.2f MB\n", float64(totalSize)/(1024*1024))
	fmt.Printf(" 吞吐量: %.2f MB/S，%.2f TPS\n", throughput, tps)
	fmt.Printf(" CPU 使用率: %.2f%%\n", cpuUsage)
	fmt.Printf(" 内存占用（开始）: %.2f MB\n", startMemory)
	fmt.Printf(" 内存占用（结束）: %.2f MB\n", endMemory)
	fmt.Printf(" GC 触发次数: %d\n", endMem.NumGC)
	fmt.Println("=================================================")

	// 结束 CPU Profiling
	pprof.StopCPUProfile()
}

// **监控 CPU 使用率**
func monitorCPUUsage() {
	ticker := time.NewTicker(5 * time.Second) // 每 5 秒采样一次 CPU 使用率
	defer ticker.Stop()

	for range ticker.C {
		cpuUsage := getCPUUsage()
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		fmt.Printf("[监控] CPU 使用率: %.2f%%，Goroutine: %d，内存占用: %.2f MB\n",
			cpuUsage, runtime.NumGoroutine(), float64(memStats.Alloc)/(1024*1024))
	}
}

// **获取 CPU 使用率**
func getCPUUsage() float64 {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		fmt.Println("获取 CPU 使用率失败:", err)
		return 0.00
	}
	return percentages[0] // 返回 CPU 总使用率
}

// **监控内存占用**
func monitorMemoryUsage() {
	ticker := time.NewTicker(5 * time.Second) // 每 5 秒记录一次内存占用
	defer ticker.Stop()

	for range ticker.C {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		fmt.Printf("[内存监控] 当前内存占用: %.2f MB, GC 次数: %d\n",
			float64(memStats.Alloc)/(1024*1024), memStats.NumGC)
	}
}
