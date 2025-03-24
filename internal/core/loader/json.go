package loader

import (
	"io/ioutil"
	"path/filepath"
	"work_2/internal/core/block"
)

func LoadBlocksFromDir(dir string) ([]*block.Block, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var blocks []*block.Block
	for _, f := range files {
		path := filepath.Join(dir, f.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		b, err := block.FromJSON(data)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}
