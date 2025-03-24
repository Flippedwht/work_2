package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
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

var db *leveldb.DB

func main() {
	var err error
	dbPath := "E:/VScode_project/work_2/bench_data_basic"
	db, err = leveldb.OpenFile(dbPath, nil)
	if err != nil {
		log.Fatalf("无法打开 LevelDB: %v", err)
	}
	defer db.Close()

	log.Println("启动 HTTP 服务器（基础版）...")

	// 设置路由
	router := mux.NewRouter()
	router.HandleFunc("/block/{blockNumber}", getBlockHandler).Methods("GET")                                    // 查询单个区块
	router.HandleFunc("/blocks", getBlocksHandler).Methods("GET")                                                // 批量查询区块
	router.HandleFunc("/tx/by-id/{txID}", getTransactionByIDHandler).Methods("GET")                              // 查询交易（通过交易ID）
	router.HandleFunc("/tx/by-block/{blockNumber}/{index}", getTransactionByBlockAndIndexHandler).Methods("GET") // 查询交易（通过区块号和顺序编号）

	// 启动 HTTP 服务器
	port := ":8081"
	log.Printf("HTTP 服务器运行在 http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, router))
}

// 查询单个区块
func getBlockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	blockNumber, err := strconv.ParseUint(vars["blockNumber"], 10, 64)
	if err != nil {
		http.Error(w, "无效的区块号", http.StatusBadRequest)
		return
	}

	// 先查 blockIndex:{区块号} 获取区块哈希
	blockIndexKey := fmt.Sprintf("blockIndex:%d", blockNumber)
	blockHashBytes, err := db.Get([]byte(blockIndexKey), nil)
	if err != nil {
		http.Error(w, "区块编号不存在", http.StatusNotFound)
		return
	}
	blockHash := string(blockHashBytes)

	// 再查 block:{区块哈希} 获取区块数据
	blockKey := fmt.Sprintf("block:%s", blockHash)
	blockData, err := db.Get([]byte(blockKey), nil)
	if err != nil {
		http.Error(w, "区块数据不存在", http.StatusNotFound)
		return
	}

	var block Block
	json.Unmarshal(blockData, &block)

	// 日志记录
	log.Printf("[查询区块] 区块号: %d, 区块哈希: %s, 交易数: %d", blockNumber, blockHash, len(block.Transactions))

	// 返回数据
	w.Header().Set("Content-Type", "application/json")
	w.Write(blockData)
}

// 批量查询区块
func getBlocksHandler(w http.ResponseWriter, r *http.Request) {
	startID, err1 := strconv.ParseUint(r.URL.Query().Get("start"), 10, 64)
	endID, err2 := strconv.ParseUint(r.URL.Query().Get("end"), 10, 64)

	if err1 != nil || err2 != nil || startID > endID {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	log.Printf("[批量查询区块] %d - %d", startID, endID)

	var blocks []Block
	for i := startID; i <= endID; i++ {
		// 先查 blockIndex:{区块号} 获取区块哈希
		blockIndexKey := fmt.Sprintf("blockIndex:%d", i)
		blockHashBytes, err := db.Get([]byte(blockIndexKey), nil)
		if err != nil {
			continue
		}
		blockHash := string(blockHashBytes)

		// 再查 block:{区块哈希} 获取区块数据
		blockKey := fmt.Sprintf("block:%s", blockHash)
		blockData, err := db.Get([]byte(blockKey), nil)
		if err != nil {
			continue
		}

		var block Block
		json.Unmarshal(blockData, &block)
		blocks = append(blocks, block)
	}

	response, _ := json.Marshal(blocks)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// 查询交易（通过交易ID）
func getTransactionByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txID := vars["txID"]

	// 先查 txIndex:{交易ID} 获取区块哈希和顺序编号
	txIndexKey := fmt.Sprintf("txIndex:%s", txID)
	blockHashBytes, err := db.Get([]byte(txIndexKey), nil)
	if err != nil {
		http.Error(w, "交易不存在", http.StatusNotFound)
		return
	}
	blockHash := string(blockHashBytes)

	// 再查 block:{区块哈希} 获取区块数据
	blockKey := fmt.Sprintf("block:%s", blockHash)
	blockData, err := db.Get([]byte(blockKey), nil)
	if err != nil {
		http.Error(w, "区块数据丢失", http.StatusInternalServerError)
		return
	}

	var block Block
	json.Unmarshal(blockData, &block)

	// 查找该区块中的目标交易
	var foundTx *Transaction
	for _, tx := range block.Transactions {
		if tx.TXID == txID {
			foundTx = &tx
			break
		}
	}

	if foundTx == nil {
		http.Error(w, "交易数据丢失", http.StatusInternalServerError)
		return
	}

	// 日志记录
	log.Printf("[查询交易] 交易ID: %s, 交易所在区块哈希: %s", txID, blockHash)

	// 返回交易数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(foundTx)
}

// 查询交易（通过区块号和顺序编号）
func getTransactionByBlockAndIndexHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	blockNumber, err := strconv.ParseUint(vars["blockNumber"], 10, 64)
	if err != nil {
		http.Error(w, "无效的区块号", http.StatusBadRequest)
		return
	}

	index, err := strconv.Atoi(vars["index"])
	if err != nil {
		http.Error(w, "无效的顺序编号", http.StatusBadRequest)
		return
	}

	// 构造区块号文件路径
	blockchainDir := "E:/VScode_project/work_2/blockchain"
	blockNumberDir := filepath.Join(blockchainDir, "block_number")
	blockNumberFilePath := filepath.Join(blockNumberDir, "block_number.csv")

	// 检查区块号文件是否存在
	if _, err := os.Stat(blockNumberFilePath); os.IsNotExist(err) {
		http.Error(w, "区块号文件不存在", http.StatusNotFound)
		return
	}

	// 读取区块号文件，找到对应的区块头哈希
	blockHash := ""
	file, err := os.Open(blockNumberFilePath)
	if err != nil {
		http.Error(w, "无法打开区块号文件", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) != 2 {
			continue
		}
		num, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			continue
		}
		if num == blockNumber {
			blockHash = record[1]
			break
		}
	}

	if blockHash == "" {
		http.Error(w, "区块头哈希不存在", http.StatusNotFound)
		return
	}

	// 构造区块头哈希文件路径
	blockHeaderDir := filepath.Join(blockchainDir, "block_header")
	blockHeaderFilePath := filepath.Join(blockHeaderDir, fmt.Sprintf("%s.csv", blockHash))

	// 检查区块头哈希文件是否存在
	if _, err := os.Stat(blockHeaderFilePath); os.IsNotExist(err) {
		http.Error(w, "区块头哈希文件不存在", http.StatusNotFound)
		return
	}

	// 读取区块头哈希文件，找到对应的交易哈希
	txID := ""
	file, err = os.Open(blockHeaderFilePath)
	if err != nil {
		http.Error(w, "无法打开区块头哈希文件", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	reader = csv.NewReader(file)
	for i := 0; ; i++ {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if i == index {
			txID = record[0]
			break
		}
	}

	if txID == "" {
		http.Error(w, "交易不存在", http.StatusNotFound)
		return
	}

	// 用交易ID查询交易数据
	txKey := fmt.Sprintf("tx:%s", txID)
	txData, err := db.Get([]byte(txKey), nil)
	if err != nil {
		http.Error(w, "交易数据不存在", http.StatusNotFound)
		return
	}

	var tx Transaction
	json.Unmarshal(txData, &tx)

	// 日志记录
	log.Printf("[查询交易] 区块号: %d, 顺序编号: %d, 交易ID: %s", blockNumber, index, txID)

	// 返回交易数据
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tx)
}
