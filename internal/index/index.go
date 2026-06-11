package index

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

var magic = [8]byte{'F', 'M', 'I', 'X', 'F', '3', '2', 0}

type Index struct {
	Count   int
	Dim     int
	Vectors []float32
}

type Result struct {
	Index int
	Score float32
}

func Load(path string) (*Index, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var got [8]byte
	if _, err := io.ReadFull(file, got[:]); err != nil {
		return nil, err
	}
	if got != magic {
		return nil, fmt.Errorf("invalid index magic %q", string(got[:]))
	}
	var header [3]uint32
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	count := int(header[1])
	dim := int(header[2])
	if count <= 0 || dim <= 0 {
		return nil, fmt.Errorf("invalid index shape %dx%d", count, dim)
	}
	vectors := make([]float32, count*dim)
	if err := binary.Read(file, binary.LittleEndian, vectors); err != nil {
		return nil, err
	}
	return &Index{Count: count, Dim: dim, Vectors: vectors}, nil
}

func (idx *Index) TopK(query []float32, k int) ([]Result, error) {
	if idx == nil {
		return nil, errors.New("index is nil")
	}
	if len(query) != idx.Dim {
		return nil, fmt.Errorf("query dim %d does not match index dim %d", len(query), idx.Dim)
	}
	if k <= 0 {
		k = 1
	}
	if k > idx.Count {
		k = idx.Count
	}
	best := make([]Result, 0, k)
	for row := 0; row < idx.Count; row++ {
		score := dot(idx.Vectors[row*idx.Dim:(row+1)*idx.Dim], query)
		insertTopK(&best, Result{Index: row, Score: score}, k)
	}
	return best, nil
}

func dot(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	if math.IsNaN(float64(sum)) {
		return -1
	}
	return sum
}

func insertTopK(best *[]Result, candidate Result, k int) {
	items := *best
	pos := len(items)
	for i, item := range items {
		if candidate.Score > item.Score {
			pos = i
			break
		}
	}
	if pos == len(items) && len(items) >= k {
		return
	}
	items = append(items, Result{})
	copy(items[pos+1:], items[pos:])
	items[pos] = candidate
	if len(items) > k {
		items = items[:k]
	}
	*best = items
}
