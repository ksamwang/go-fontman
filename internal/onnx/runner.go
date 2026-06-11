package onnx

import (
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

type Runner struct {
	session *ort.DynamicAdvancedSession
	output  string
}

func NewRunner(dllPath, modelPath, inputName, outputName string) (*Runner, error) {
	if dllPath != "" {
		ort.SetSharedLibraryPath(dllPath)
	}
	if err := ort.InitializeEnvironment(ort.WithLogLevelWarning()); err != nil {
		return nil, err
	}
	session, err := ort.NewDynamicAdvancedSession(modelPath, []string{inputName}, []string{outputName}, nil)
	if err != nil {
		_ = ort.DestroyEnvironment()
		return nil, err
	}
	return &Runner{session: session, output: outputName}, nil
}

func (r *Runner) Close() error {
	if r == nil {
		return nil
	}
	if r.session != nil {
		if err := r.session.Destroy(); err != nil {
			return err
		}
	}
	return ort.DestroyEnvironment()
}

func (r *Runner) Embed(input []float32, shape []int) ([]float32, error) {
	if len(shape) != 4 {
		return nil, fmt.Errorf("expected 4D input shape, got %v", shape)
	}
	ortShape := ort.NewShape(int64(shape[0]), int64(shape[1]), int64(shape[2]), int64(shape[3]))
	tensor, err := ort.NewTensor(ortShape, input)
	if err != nil {
		return nil, err
	}
	defer tensor.Destroy()

	outputs := []ort.Value{nil}
	if err := r.session.Run([]ort.Value{tensor}, outputs); err != nil {
		return nil, err
	}
	defer outputs[0].Destroy()

	output, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type for %s", r.output)
	}
	data := output.GetData()
	embedding := make([]float32, len(data))
	copy(embedding, data)
	return embedding, nil
}
