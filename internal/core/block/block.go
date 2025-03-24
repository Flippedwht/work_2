package block

import "encoding/json"

type Block struct {
	Number       uint64         `json:"block_number"`
	Timestamp    uint64         `json:"timestamp"`
	Transactions []Transaction  `json:"transactions"`
	TxIndex      map[string]int `json:"tx_index"` // 内部索引，记录交易在Transactions中的位置
}

type Transaction struct {
	TXID     string `json:"txid"`
	From     string `json:"from"`
	To       string `json:"to"`
	Gas      uint64 `json:"gas"`
	GasPrice uint64 `json:"gas_price"`
	Amount   uint64 `json:"amount"`
}

func FromJSON(data []byte) (*Block, error) {
	var b Block
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *Block) Size() int {
	size := 20 + len(b.Transactions)*64 // 64 字节交易哈希的估算
	return size
}
