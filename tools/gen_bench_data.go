// tools/gen_bench_data.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"work_2/internal/core/block"
	"work_2/internal/core/storage"

	"github.com/shirou/gopsutil/cpu"
)

const (
	NumBlocks   = 100000 // 生成 5 万个区块
	FlushPeriod = 3      // 每 50 个区块合并写入
)

// 用户特征
type UserProfile struct {
	UserID        string
	QueryCount    int
	LastQueryTime time.Time
	QueryHistory  map[string]int  // 按小时统计查询次数 ("YYYY-MM-DD HH" -> count)
	DailyPattern  map[int]float64 // 24 小时模式 ("小时" -> 平均查询次数)
	// 计算过去 7 天每小时的平均查询次数，找出用户的查询高峰
	QuerySequence []string // 记录用户最近查询的交易 ID
}

func main() {
	rand.Seed(time.Now().UnixNano()) // 确保每次运行生成不同数据

	// 创建 CPU Profile 文件
	cpuProfile, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal("无法创建 CPU Profile 文件:", err)
	}
	defer cpuProfile.Close()

	// 开启 CPU Profiling
	pprof.StartCPUProfile(cpuProfile)
	defer pprof.StopCPUProfile()

	// 初始化 Chunk 存储
	s, err := storage.NewChunkStorage("bench_data")
	if err != nil {
		panic(fmt.Sprintf("无法初始化存储: %v", err))
	}
	defer s.Close()

	// **记录初始内存占用**
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)
	startMemory := float64(startMem.Alloc) / (1024 * 1024)

	// **记录测试开始时间**
	startTime := time.Now()
	writeStartTime := time.Now()
	totalSize := int64(0)
	fmt.Println(" 开始生成测试数据...")

	// 启动 CPU 和内存监控协程
	go monitorCPUUsage()
	go monitorMemoryUsage()

	blocks := []*block.Block{}            // **额外维护区块列表**
	transactions := []map[string]string{} // **单独记录交易数据**

	// 生成 NumBlocks 个区块
	for i := uint64(1); i <= NumBlocks; i++ {
		txCount := 100
		txs := make([]block.Transaction, txCount)

		for j := 0; j < txCount; j++ {
			fromUser := generateUser()
			txHash := fmt.Sprintf("tx%d_%d", i, j) // 交易哈希

			txs[j] = block.Transaction{
				TXID:   txHash,
				From:   fromUser,
				To:     fmt.Sprintf("user_%d", rand.Intn(100)),
				Amount: uint64(rand.Intn(10000)),
				Gas:    uint64(rand.Intn(1000)),
			}

			updateUserProfile(s, fromUser, txHash) // 传入 userID 和 txHash

			// **记录交易信息**
			transactions = append(transactions, map[string]string{
				"txHash": txHash,
				"from":   fromUser,
			})
		}

		// 创建区块
		b := &block.Block{
			Number:       i,
			Timestamp:    uint64(time.Now().Unix()),
			Transactions: txs,
		}

		// 添加区块到存储
		s.AddBlock(b)
		blocks = append(blocks, b) // **记录区块数据**
		//统计一下大小
		blockBytes, _ := json.Marshal(b) // 序列化区块
		blockSize := int64(len(blockBytes))
		totalSize += blockSize // 累加数据大小

		// 每 FlushPeriod 个区块强制写入一次数据库
		if i%FlushPeriod == 0 {
			s.Flush()
			fmt.Printf("已生成 %d 个区块，数据写入中...\n", i)
		}
	}

	// **最后一次 Flush，确保数据完整**
	s.Flush()
	writeElapsed := time.Since(writeStartTime)
	// **保存交易数据**
	saveTransactionData(transactions, "E:\\VScode_project\\work_2\\transactions.json")

	// **统计吞吐量**
	elapsed := time.Since(startTime)
	tps := float64(NumBlocks) / elapsed.Seconds()
	throughput := float64(totalSize) / (1024 * 1024) / elapsed.Seconds() // MB/s

	// **获取 CPU & 内存 使用率**
	cpuUsage := getCPUUsage()

	// **记录最终内存占用**
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)
	endMemory := float64(endMem.Alloc) / (1024 * 1024)

	// **打印统计结果**
	fmt.Println("\n====================测试结果 ====================")
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
		fmt.Printf("[CPU 监控] CPU 使用率: %.2f%%\n", cpuUsage)
	}
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

// **获取 CPU 使用率**
func getCPUUsage() float64 {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		fmt.Println("获取 CPU 使用率失败:", err)
		return 0.00
	}
	return percentages[0] // 返回 CPU 总使用率
}

func generateUser() string {
	if rand.Float64() < 0.3 {
		return fmt.Sprintf("user_%d", rand.Intn(10)) // 30% 生成热用户
	}
	return fmt.Sprintf("user_%d", rand.Intn(90)+10) // 70% 生成冷用户
}

func updateUserProfile(cs *storage.ChunkStorage, userID, txHash string) {
	key := []byte(fmt.Sprintf("user_profile:%s", userID))
	data, err := cs.GetDB().Get(key, nil) // 直接调用 GetDB() 访问 LevelDB
	var profile UserProfile

	if err == nil {
		_ = json.Unmarshal(data, &profile)
	} else {
		profile = UserProfile{
			UserID:        userID,
			QueryCount:    0, //  生成测试数据时，不增加查询次数
			QueryHistory:  make(map[string]int),
			DailyPattern:  make(map[int]float64),
			QuerySequence: []string{},
		}
	}

	// **只记录交易，不增加查询次数**
	profile.QuerySequence = append(profile.QuerySequence, txHash)
	if len(profile.QuerySequence) > 10 {
		profile.QuerySequence = profile.QuerySequence[1:] // 只保留最近 10 条查询
	}

	// 存入 LevelDB
	data, _ = json.Marshal(profile)
	if err := cs.GetDB().Put(key, data, nil); err != nil {
		fmt.Printf("[ERROR] 用户数据写入失败: %v\n", err)
	} // 直接操作 LevelDB
}

func saveTransactionData(transactions []map[string]string, outputPath string) {
	data, err := json.MarshalIndent(transactions, "", "  ")
	if err != nil {
		fmt.Printf("交易数据序列化失败: %v\n", err)
		return
	}

	err = os.WriteFile(outputPath, data, 0644)
	if err != nil {
		fmt.Printf("交易数据写入失败: %v\n", err)
		return
	}

}
