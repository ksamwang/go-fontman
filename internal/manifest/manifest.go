package manifest

import (
	"encoding/json"
	"os"
)

type Manifest struct {
	Model      Model      `json:"model"`
	Preprocess Preprocess `json:"preprocess"`
	Index      Index      `json:"index"`
	Metadata   Metadata   `json:"metadata"`
}

type Model struct {
	Path             string `json:"path"`
	InputName        string `json:"inputName"`
	InputShape       []int  `json:"inputShape"`
	OutputName       string `json:"outputName"`
	EmbeddingDim     int    `json:"embeddingDim"`
	OutputNormalized bool   `json:"outputNormalized"`
}

type Preprocess struct {
	ImageWidth  int    `json:"imageWidth"`
	ImageHeight int    `json:"imageHeight"`
	ColorMode   string `json:"colorMode"`
	Layout      string `json:"layout"`
}

type Index struct {
	Path              string `json:"path"`
	Count             int    `json:"count"`
	Dim               int    `json:"dim"`
	Metric            string `json:"metric"`
	VectorsNormalized bool   `json:"vectorsNormalized"`
}

type Metadata struct {
	Path string `json:"path"`
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out Manifest
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
