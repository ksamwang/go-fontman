package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ksamwang/go-fontman/internal/imageprep"
	"github.com/ksamwang/go-fontman/internal/matcher"
)

func main() {
	var manifestPath string
	var dllPath string
	var imagePath string
	var boxText string
	var topK int
	flag.StringVar(&manifestPath, "manifest", "artifacts/font_ai/runtime/manifest.json", "runtime manifest path")
	flag.StringVar(&dllPath, "onnxruntime-dll", defaultONNXRuntimeDLL(), "onnxruntime.dll path")
	flag.StringVar(&imagePath, "image", "", "input image path")
	flag.StringVar(&boxText, "box", "", "crop box as x,y,w,h")
	flag.IntVar(&topK, "top-k", 5, "number of font candidates")
	flag.Parse()

	if imagePath == "" || boxText == "" {
		fail("usage: fontman-cli -image path -box x,y,w,h [-manifest path] [-onnxruntime-dll path] [-top-k 5]")
	}
	box, err := parseBox(boxText)
	if err != nil {
		fail(err.Error())
	}
	m, err := matcher.New(manifestPath, dllPath)
	if err != nil {
		fail(err.Error())
	}
	defer m.Close()

	results, err := m.MatchImage(imagePath, box, topK)
	if err != nil {
		fail(err.Error())
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		fail(err.Error())
	}
}

func parseBox(value string) (imageprep.Box, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return imageprep.Box{}, fmt.Errorf("box must be x,y,w,h")
	}
	values := make([]int, 4)
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return imageprep.Box{}, fmt.Errorf("invalid box value %q", part)
		}
		values[i] = n
	}
	return imageprep.Box{X: values[0], Y: values[1], W: values[2], H: values[3]}, nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func defaultONNXRuntimeDLL() string {
	if value := strings.TrimSpace(os.Getenv("ONNXRUNTIME_DLL")); value != "" {
		return value
	}
	path := filepath.Join("runtime", "onnxruntime", "win-x64", "onnxruntime.dll")
	if _, err := os.Stat(path); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return path
	}
	return ""
}
