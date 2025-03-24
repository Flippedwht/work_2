package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"work_2/internal/core/storage"
)

var optStorage *storage.ChunkStorage

func main() {
	log.Println("启动HTTP服务（优化版本）...")

	// 实例化优化后的存储
	var err error
	optStorage, err = storage.NewChunkStorage("E:/VScode_project/work_2/bench_data")
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}
	defer optStorage.Close()

	http.HandleFunc("/block", handleBlockQuery)   // 查询单个区块
	http.HandleFunc("/blocks", handleBlocksQuery) // **新增批量查询区块**
	http.HandleFunc("/tx", handleTxQuery)         // 查询交易

	log.Println("HTTP 服务器已启动，监听端口 :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 处理**单个区块**查询
func handleBlockQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("id")
	if query == "" {
		http.Error(w, "缺少 id 参数", http.StatusBadRequest)
		return
	}

	blockID, err := strconv.ParseUint(query, 10, 64)
	if err != nil {
		http.Error(w, "无效的区块 ID", http.StatusBadRequest)
		return
	}

	data, err := optStorage.GetBlockByNumber(blockID)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("查询区块: %d, 存储路径: %s", blockID, optStorage.DBPath())
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// **批量查询区块**
func handleBlocksQuery(w http.ResponseWriter, r *http.Request) {
	startID, err1 := strconv.ParseUint(r.URL.Query().Get("start"), 10, 64)
	endID, err2 := strconv.ParseUint(r.URL.Query().Get("end"), 10, 64)

	if err1 != nil || err2 != nil || startID > endID {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	log.Printf("批量查询区块: %d - %d", startID, endID)

	var blocks []map[string]interface{}
	for i := startID; i <= endID; i++ {
		data, err := optStorage.GetBlockByNumber(i)
		if err == nil {
			var block map[string]interface{}
			json.Unmarshal(data, &block)
			blocks = append(blocks, block)
		}
	}

	response, _ := json.Marshal(blocks)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// 处理**交易查询**
func handleTxQuery(w http.ResponseWriter, r *http.Request) {
	txHash := r.URL.Query().Get("tx")
	if txHash == "" {
		http.Error(w, "缺少 tx 参数", http.StatusBadRequest)
		return
	}

	blockData, txData, err := optStorage.GetBlockByTxHash(txHash)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询失败: %v", err), http.StatusInternalServerError)
		return
	}

	var block map[string]interface{}
	if err := json.Unmarshal(blockData, &block); err != nil {
		http.Error(w, "解析区块数据失败", http.StatusInternalServerError)
		return
	}

	var tx map[string]interface{}
	if err := json.Unmarshal(txData, &tx); err != nil {
		http.Error(w, "解析交易数据失败", http.StatusInternalServerError)
		return
	}

	// **返回交易信息 + 所属区块**
	result := map[string]interface{}{
		"block_number": block["block_number"],
		"timestamp":    block["timestamp"],
		"transaction":  tx,
	}

	log.Printf("查询交易: %s, 所属区块: %v, 存储路径: %s", txHash, block["block_number"], optStorage.DBPath())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
