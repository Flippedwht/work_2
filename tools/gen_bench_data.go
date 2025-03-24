// tools/gen_bench_data.go
package main

import (
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
	NumBlocks   = 50000 // 生成 5 万个区块
	FlushPeriod = 50    // 每 50 个区块合并写入
)

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

	// 生成 NumBlocks 个区块
	for i := uint64(1); i <= NumBlocks; i++ {
		txCount := 30
		txs := make([]block.Transaction, txCount)

		for j := 0; j < txCount; j++ {
			txs[j] = block.Transaction{
				TXID:   fmt.Sprintf("tx%d_%d", i, j), // 交易ID格式: tx{区块号}_{交易序号}
				From:   fmt.Sprintf("user_%d", rand.Intn(100)),
				To:     fmt.Sprintf("user_%d", rand.Intn(100)),
				Amount: uint64(rand.Intn(10000)),
				Gas:    uint64(rand.Intn(1000)),
			}
		}

		// 创建区块
		b := &block.Block{
			Number:       i,
			Timestamp:    uint64(time.Now().Unix()),
			Transactions: txs,
		}

		// 添加区块到当前 Chunk 缓冲区
		s.AddBlock(b)

		// 计算数据大小（近似计算）
		blockSize := int64(100 + len(txs)*64) // 近似计算一个区块的大小（假设区块头 100 字节，每笔交易 64 字节）
		totalSize += blockSize

		// 每 FlushPeriod 个区块强制写入一次数据库
		if i%FlushPeriod == 0 {
			s.Flush()
			fmt.Printf("✅ 已生成 %d 个区块，数据写入中...\n", i)
		}
	}

	// **最后一次 Flush，确保数据完整**
	s.Flush()
	writeElapsed := time.Since(writeStartTime)

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
