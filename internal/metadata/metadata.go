package metadata

import (
	"encoding/json"
	"os"
)

type Store struct {
	Fonts []Font `json:"fonts"`
}

type Font struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	FilePath    string `json:"file_path"`
	FaceIndex   int    `json:"face_index"`
	ScriptScope string `json:"script_scope"`
	StyleGroup  string `json:"style_group"`
	FamilyName  string `json:"family_name"`
	WeightName  string `json:"weight_name"`
	IsItalic    bool   `json:"is_italic"`
	IsTTC       bool   `json:"is_ttc"`
	Category    string `json:"category"`
}

func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out Store
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) FontAt(index int) Font {
	if s == nil || index < 0 || index >= len(s.Fonts) {
		return Font{}
	}
	return s.Fonts[index]
}
