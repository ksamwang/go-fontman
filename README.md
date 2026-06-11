# go-fontman

`go-fontman` 是一个本地字体识别运行时。它消费已经训练好的 Find Fontman 字体向量模型和字体索引，在 Go 中完成图片文字区域的字体匹配。

本项目不负责训练。训练、字体库扫描、索引生成仍由外部训练项目完成；本项目只负责把训练产物转换成桌面端可用的 ONNX runtime 资产，并提供 CLI 和本地 HTTP 服务给主程序调用。

## 当前能力

- 加载 ONNX 字体 embedding 模型。
- 加载字体向量索引 `font_index.f32bin`。
- 读取字体元信息 `font_index_meta.json`。
- 对图片中的指定文字区域进行裁切、缩放、归一化。
- 返回 Top-K 字体候选。
- 返回关键字体元信息：
  - `faceIndex`
  - `faceName`
  - `familyName`
  - `weightName`
  - `styleGroup`
  - `scriptScope`
  - `isTTC`
  - `isItalic`
- 提供两个入口：
  - `fontman-cli`：命令行调试工具。
  - `fontman-service`：本地常驻 HTTP 服务，并内嵌测试网页。

## 目录结构

```text
cmd/
  fontman-cli/          # 命令行调试入口
  fontman-service/      # 本地 HTTP 服务 + 内嵌测试网页

internal/
  imageprep/            # 图片裁切、缩放、归一化
  index/                # 字体向量索引读取和 Top-K 检索
  manifest/             # runtime manifest 读取
  matcher/              # 字体匹配主流程
  metadata/             # 字体元信息读取
  onnx/                 # ONNX Runtime 封装

runtime/
  font_ai/
    manifest.json
    model/
      font_embedding.onnx
      font_embedding.onnx.data
    data/
      font_index.f32bin
      font_index_meta.json
  onnxruntime/
    win-x64/
      onnxruntime.dll
      onnxruntime_providers_shared.dll

tools/
  export_runtime.py
  requirements-export.txt

artifacts/
  font_ai/
    source/             # 本地转换输入，默认不提交
```

## 运行资产

Go 运行时需要以下目录：

```text
runtime/font_ai/
runtime/onnxruntime/win-x64/
```

`runtime/font_ai` 包含模型、索引和元信息。`runtime/onnxruntime/win-x64` 包含 ONNX Runtime DLL。默认情况下 CLI 和服务都会从这些路径加载运行资产。

当前 Go ONNX 绑定版本固定为：

```text
github.com/yalue/onnxruntime_go v1.23.0
```

它匹配 ONNX Runtime 1.23.x。后续如果替换 `onnxruntime.dll` 版本，需要同步调整 Go 依赖版本。

## 模型输入约定

模型输入固定为：

```text
NCHW float32 [1, 3, 128, 384]
```

预处理流程：

```text
原图
  -> 按 box 裁切文字区域
  -> 转 RGB
  -> 等比缩放到 384x128 内
  -> 白底居中
  -> ((pixel / 255.0) - 0.5) / 0.5
  -> NCHW float32
  -> ONNX 模型
```

匹配方式：

```text
embedding dot font_index_vector
```

索引向量和模型输出都按 L2 归一化处理，所以 dot product 等价于 cosine similarity。

## box 参数说明

`box` 使用原图像素坐标：

```json
{"x": 0, "y": 0, "w": 90, "h": 43}
```

含义：

```text
x: 裁切区域左上角 X
y: 裁切区域左上角 Y
w: 裁切区域宽度
h: 裁切区域高度
```

建议只框选文字本体，避免包含 logo、边框、大面积背景或反光区域。如果图片本身就是纯文字裁切图，可以使用整图尺寸作为 box。

## 命令行使用

运行 CLI：

```powershell
go run ./cmd/fontman-cli `
  -image D:\path\to\sample.png `
  -box 100,80,420,120 `
  -top-k 5
```

默认会加载：

```text
runtime/font_ai/manifest.json
runtime/onnxruntime/win-x64/onnxruntime.dll
```

也可以显式指定：

```powershell
go run ./cmd/fontman-cli `
  -manifest runtime\font_ai\manifest.json `
  -onnxruntime-dll runtime\onnxruntime\win-x64\onnxruntime.dll `
  -image D:\path\to\sample.png `
  -box 100,80,420,120 `
  -top-k 5
```

`-onnxruntime-dll` 也可以通过环境变量覆盖：

```powershell
$env:ONNXRUNTIME_DLL="D:\path\to\onnxruntime.dll"
```

CLI 输出示例：

```json
[
  {
    "rank": 1,
    "index": 2489,
    "score": 0.5327049,
    "font": {
      "name": "ToronoGlitchSerif H4 Heavy Regular",
      "path": "D:\\...\\font.otf",
      "filePath": "D:\\...\\font.otf",
      "faceIndex": 0,
      "faceName": "ToronoGlitchSerif H4 Heavy Regular",
      "familyName": "繁-宋体",
      "weightName": "heavy",
      "styleGroup": "serif",
      "scriptScope": "zh_simplified",
      "category": "zh_simplified",
      "isItalic": false,
      "isTTC": false
    }
  }
]
```

## 本地服务

运行服务：

```powershell
go run ./cmd/fontman-service
```

默认监听：

```text
127.0.0.1:9092
```

也可以指定地址：

```powershell
go run ./cmd/fontman-service -addr 127.0.0.1:9092
```

服务启动后会加载模型和索引，后续请求复用同一个 matcher。主程序集成时建议使用这个服务入口，而不是频繁调用 CLI。

## 测试网页

打开：

```text
http://127.0.0.1:9092
```

页面支持：

- 上传图片。
- 在画布上拖拽标记文字区域。
- 提交识别。
- 查看 Top-K 字体候选。

测试页面使用 multipart 接口：

```text
POST /api/match
```

这个接口主要给页面使用，主程序建议使用 JSON 接口 `/match`。

## 主程序 API

健康检查：

```http
GET /health
```

返回示例：

```json
{
  "service": "go-fontman",
  "status": "ok"
}
```

字体匹配：

```http
POST /match
Content-Type: application/json
```

请求：

```json
{
  "imagePath": "D:\\path\\to\\image.png",
  "box": {"x": 0, "y": 0, "w": 90, "h": 43},
  "topK": 5
}
```

响应：

```json
{
  "box": {"x": 0, "y": 0, "w": 90, "h": 43},
  "results": [
    {
      "rank": 1,
      "index": 2489,
      "score": 0.5327049,
      "font": {
        "name": "ToronoGlitchSerif H4 Heavy Regular",
        "path": "D:\\...\\font.otf",
        "filePath": "D:\\...\\font.otf",
        "faceIndex": 0,
        "faceName": "ToronoGlitchSerif H4 Heavy Regular",
        "familyName": "繁-宋体",
        "weightName": "heavy",
        "styleGroup": "serif",
        "scriptScope": "zh_simplified",
        "category": "zh_simplified",
        "isItalic": false,
        "isTTC": false
      }
    }
  ]
}
```

## 训练产物转换

训练后的原始产物放到：

```text
artifacts/font_ai/source/
```

需要包含：

```text
font_embedding.pt
font_index.npz
font_index_meta.json
train_config.json
```

安装转换依赖：

```powershell
python -m venv .venv-export
.\.venv-export\Scripts\python.exe -m pip install -r tools\requirements-export.txt
```

如果需要代理：

```powershell
.\.venv-export\Scripts\python.exe -m pip install -r tools\requirements-export.txt `
  -i http://pypi.tuna.tsinghua.edu.cn/simple `
  --trusted-host pypi.tuna.tsinghua.edu.cn `
  --proxy http://127.0.0.1:10808
```

导出 runtime：

```powershell
.\.venv-export\Scripts\python.exe tools\export_runtime.py
```

输出到：

```text
runtime/font_ai/
```

导出脚本只是一次性转换工具。最终 Go 服务运行不依赖 Python、PyTorch 或 NumPy。

## 构建

构建 CLI：

```powershell
go build -o dist\fontman-cli.exe ./cmd/fontman-cli
```

构建服务：

```powershell
go build -o dist\fontman-service.exe ./cmd/fontman-service
```

发布时需要同时带上：

```text
runtime/
```

## 注意事项

- `font_index_meta.json` 当前包含训练环境中的字体绝对路径。后续如果要跨机器发布，需要进一步做字体路径便携化。
- 字体识别模型只应该输入文字区域，不适合输入整张胸牌、logo 或复杂背景。
- `topK` 只是候选数量，不代表自动确认字体。建议主程序保留人工选择或校正入口。
- OCR 不在当前 Go 匹配链路中。当前接口假设调用方已经知道文字区域位置。
