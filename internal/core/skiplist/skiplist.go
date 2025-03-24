// skiplist.go
package skiplist

import (
	"math/rand"
	"time"
	"work_2/internal/core/block"
)

// SkipListNode 表示跳表中的一个节点
type SkipListNode struct {
	BlockNumber uint64
	Block       *block.Block    // 指向区块数据
	AccessCount int             // 访问计数
	level       int             // 当前节点的层数（1~maxLevel）
	forward     []*SkipListNode // 每一层的前向指针
}

// SkipList 表示跳表
type SkipList struct {
	header      *SkipListNode // 头节点
	level       int           // 当前跳表最高层数
	maxLevel    int           // 跳表允许的最大层数
	probability float64       // 基础提升概率（例如0.5）
	fMax        int           // 当前全局访问计数最大值
}

// NewSkipList 创建一个新的跳表
func NewSkipList(maxLevel int, probability float64) *SkipList {
	header := &SkipListNode{
		forward: make([]*SkipListNode, maxLevel),
		level:   1,
	}
	return &SkipList{
		header:      header,
		level:       1,
		maxLevel:    maxLevel,
		probability: probability,
		fMax:        1,
	}
}

// randomLevel 根据节点的访问计数动态生成一个层级
// 公式：p = min(0.3 + 0.4 * (accessCount / fMax), 0.7)
// 然后以 p 为概率生成新层级
func (sl *SkipList) randomLevel(accessCount int) int {
	// 更新全局最大访问计数
	if accessCount > sl.fMax {
		sl.fMax = accessCount
	}
	p := 0.3 + 0.4*float64(accessCount)/float64(sl.fMax)
	if p > 0.7 {
		p = 0.7
	}
	level := 1
	// 用随机数生成层级
	rand.Seed(time.Now().UnixNano())
	for rand.Float64() < p && level < sl.maxLevel {
		level++
	}
	return level
}

// Insert 插入或更新一个节点，如果已存在则更新访问计数并可能提升层级
func (sl *SkipList) Insert(blockNumber uint64, blk *block.Block) {
	update := make([]*SkipListNode, sl.maxLevel)
	x := sl.header
	// 从最高层向下寻找插入位置
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].BlockNumber < blockNumber {
			x = x.forward[i]
		}
		update[i] = x
	}
	x = x.forward[0]
	if x != nil && x.BlockNumber == blockNumber {
		// 节点已存在，增加访问计数
		x.AccessCount++
		// 计算期望的新层级
		newLevel := sl.randomLevel(x.AccessCount)
		if newLevel > x.level {
			// 需要提升节点：先移除，再重新插入
			sl.remove(blockNumber)
			sl.insertNew(blockNumber, blk, x.AccessCount, newLevel)
		}
		return
	}
	// 新节点插入时，访问计数设为1
	level := sl.randomLevel(1)
	if level > sl.level {
		for i := sl.level; i < level; i++ {
			update[i] = sl.header
		}
		sl.level = level
	}
	newNode := &SkipListNode{
		BlockNumber: blockNumber,
		Block:       blk,
		AccessCount: 1,
		level:       level,
		forward:     make([]*SkipListNode, level),
	}
	for i := 0; i < level; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}
}

// insertNew 是辅助函数，用于以指定的访问计数和层级插入一个新节点
func (sl *SkipList) insertNew(blockNumber uint64, blk *block.Block, accessCount int, level int) {
	update := make([]*SkipListNode, sl.maxLevel)
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].BlockNumber < blockNumber {
			x = x.forward[i]
		}
		update[i] = x
	}
	if level > sl.level {
		for i := sl.level; i < level; i++ {
			update[i] = sl.header
		}
		sl.level = level
	}
	newNode := &SkipListNode{
		BlockNumber: blockNumber,
		Block:       blk,
		AccessCount: accessCount,
		level:       level,
		forward:     make([]*SkipListNode, level),
	}
	for i := 0; i < level; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}
}

// remove 从跳表中删除指定 blockNumber 的节点
func (sl *SkipList) remove(blockNumber uint64) {
	update := make([]*SkipListNode, sl.maxLevel)
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].BlockNumber < blockNumber {
			x = x.forward[i]
		}
		update[i] = x
	}
	x = x.forward[0]
	if x != nil && x.BlockNumber == blockNumber {
		for i := 0; i < sl.level; i++ {
			if update[i].forward[i] != x {
				break
			}
			update[i].forward[i] = x.forward[i]
		}
		// 调整跳表当前层数
		for sl.level > 1 && sl.header.forward[sl.level-1] == nil {
			sl.level--
		}
	}
}

// Search 查找给定 blockNumber 的节点，若命中则更新其访问计数和可能提升层级
func (sl *SkipList) Search(blockNumber uint64) *SkipListNode {
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].BlockNumber < blockNumber {
			x = x.forward[i]
		}
	}
	x = x.forward[0]
	if x != nil && x.BlockNumber == blockNumber {
		// 更新访问计数
		x.AccessCount++
		newLevel := sl.randomLevel(x.AccessCount)
		if newLevel > x.level {
			sl.remove(blockNumber)
			sl.insertNew(blockNumber, x.Block, x.AccessCount, newLevel)
			return sl.Search(blockNumber)
		}
		return x
	}
	return nil
}
