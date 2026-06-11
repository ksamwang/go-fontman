package matcher

import (
	"path/filepath"

	"github.com/ksamwang/go-fontman/internal/imageprep"
	"github.com/ksamwang/go-fontman/internal/index"
	"github.com/ksamwang/go-fontman/internal/manifest"
	"github.com/ksamwang/go-fontman/internal/metadata"
	"github.com/ksamwang/go-fontman/internal/onnx"
)

type Matcher struct {
	manifest *manifest.Manifest
	index    *index.Index
	metadata *metadata.Store
	runner   *onnx.Runner
}

type Match struct {
	Rank  int     `json:"rank"`
	Index int     `json:"index"`
	Score float32 `json:"score"`
	Font  FontRef `json:"font"`
}

type FontRef struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	FilePath string `json:"filePath,omitempty"`

	FaceIndex int    `json:"faceIndex"`
	FaceName  string `json:"faceName"`

	FamilyName  string `json:"familyName"`
	WeightName  string `json:"weightName"`
	StyleGroup  string `json:"styleGroup"`
	ScriptScope string `json:"scriptScope"`
	Category    string `json:"category"`

	IsItalic bool `json:"isItalic"`
	IsTTC    bool `json:"isTTC"`
}

func New(manifestPath, dllPath string) (*Matcher, error) {
	mf, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, err
	}
	idx, err := index.Load(resolve(manifestPath, mf.Index.Path))
	if err != nil {
		return nil, err
	}
	meta, err := metadata.Load(resolve(manifestPath, mf.Metadata.Path))
	if err != nil {
		return nil, err
	}
	runner, err := onnx.NewRunner(dllPath, resolve(manifestPath, mf.Model.Path), mf.Model.InputName, mf.Model.OutputName)
	if err != nil {
		return nil, err
	}
	return &Matcher{manifest: mf, index: idx, metadata: meta, runner: runner}, nil
}

func (m *Matcher) Close() error {
	if m == nil || m.runner == nil {
		return nil
	}
	return m.runner.Close()
}

func (m *Matcher) MatchImage(imagePath string, box imageprep.Box, topK int) ([]Match, error) {
	width := m.manifest.Preprocess.ImageWidth
	height := m.manifest.Preprocess.ImageHeight
	tensor, err := imageprep.TensorFromFile(imagePath, box, width, height)
	if err != nil {
		return nil, err
	}
	embedding, err := m.runner.Embed(tensor, m.manifest.Model.InputShape)
	if err != nil {
		return nil, err
	}
	results, err := m.index.TopK(embedding, topK)
	if err != nil {
		return nil, err
	}
	matches := make([]Match, 0, len(results))
	for rank, result := range results {
		font := m.metadata.FontAt(result.Index)
		matches = append(matches, Match{
			Rank:  rank + 1,
			Index: result.Index,
			Score: result.Score,
			Font: FontRef{
				Name:        font.Name,
				Path:        font.Path,
				FilePath:    font.FilePath,
				FaceIndex:   font.FaceIndex,
				FaceName:    font.Name,
				FamilyName:  font.FamilyName,
				WeightName:  font.WeightName,
				StyleGroup:  font.StyleGroup,
				ScriptScope: font.ScriptScope,
				Category:    font.Category,
				IsItalic:    font.IsItalic,
				IsTTC:       font.IsTTC,
			},
		})
	}
	return matches, nil
}

func resolve(manifestPath, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	clean := filepath.Clean(value)
	if _, err := filepath.Rel(filepath.Dir(manifestPath), clean); err == nil {
		// Manifest paths are exported relative to the repository/runtime working
		// directory. Prefer them as-is when present.
		if abs, err := filepath.Abs(clean); err == nil {
			return abs
		}
	}
	return filepath.Join(filepath.Dir(manifestPath), clean)
}
