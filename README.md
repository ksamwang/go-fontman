# go-fontman

Go runtime for Find Fontman inference. Training stays outside this repo; this
project consumes trained `font_ai` artifacts and exports a desktop-friendly
runtime bundle.

## Artifact Flow

Copy trained files into `artifacts/font_ai/source/`:

- `font_embedding.pt`
- `font_index.npz`
- `font_index_meta.json`
- `train_config.json`

The current source artifact location is:

```powershell
D:\Wanglv_Dev\GolangCodeSource\IdBadge\workers\image-replica\tools\find_fontman\data\font_ai
```

Export ONNX and the Go-friendly vector index:

```powershell
python tools\export_runtime.py
```

Outputs:

- `runtime/font_ai/model/font_embedding.onnx`
- `runtime/font_ai/model/font_embedding.onnx.data`
- `runtime/font_ai/manifest.json`
- `runtime/font_ai/data/font_index.f32bin`
- `runtime/font_ai/data/font_index_meta.json`

The export script is a conversion tool only. The final Go runtime should not
depend on Python, PyTorch, or NumPy.

## Model Contract

- Input: `image`, float32, `NCHW`, shape `[1, 3, 128, 384]`
- Preprocess: RGB, keep-aspect thumbnail into `384x128`, white canvas,
  normalize with `((pixel / 255.0) - 0.5) / 0.5`
- Output: `embedding`, float32, 512 dimensions, L2 normalized
- Match metric: dot product against normalized index vectors

## CLI Probe

The first Go-side probe is `cmd/fontman-cli`. It loads the exported runtime,
runs ONNX inference, and returns Top-K font candidates.

```powershell
go run ./cmd/fontman-cli `
  -manifest runtime\font_ai\manifest.json `
  -image D:\path\to\sample.png `
  -box 100,80,420,120 `
  -top-k 5
```

By default the CLI loads:

```text
runtime/onnxruntime/win-x64/onnxruntime.dll
```

Use `-onnxruntime-dll` or `ONNXRUNTIME_DLL` to override it.

The current Go binding is pinned to `github.com/yalue/onnxruntime_go v1.23.0`,
which matches ONNX Runtime 1.23.x. If the DLL version changes, update the Go
binding version in lockstep.
