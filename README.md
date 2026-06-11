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

- `artifacts/font_ai/runtime/model/font_embedding.onnx`
- `artifacts/font_ai/runtime/model/font_embedding.onnx.data`
- `artifacts/font_ai/runtime/manifest.json`
- `artifacts/font_ai/runtime/data/font_index.f32bin`

The export script is a conversion tool only. The final Go runtime should not
depend on Python, PyTorch, or NumPy.

## Model Contract

- Input: `image`, float32, `NCHW`, shape `[1, 3, 128, 384]`
- Preprocess: RGB, keep-aspect thumbnail into `384x128`, white canvas,
  normalize with `((pixel / 255.0) - 0.5) / 0.5`
- Output: `embedding`, float32, 512 dimensions, L2 normalized
- Match metric: dot product against normalized index vectors
