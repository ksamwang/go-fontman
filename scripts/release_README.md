# go-fontman 发布包

本目录是 `go-fontman` 的可运行发布版本。`runtime` 目录必须和 exe 文件保持同级，不能删除或移动。

## 启动服务

默认监听 `127.0.0.1:9092`：

```powershell
.\fontman-service.exe
```

指定端口：

```powershell
.\fontman-service.exe -port 19092
```

指定完整监听地址：

```powershell
.\fontman-service.exe -addr 127.0.0.1:19092
```

## 测试页面

启动服务后打开：

```text
http://127.0.0.1:9092
```

页面支持上传图片、拖拽标记文字区域并查看字体候选。

## 健康检查接口

```http
GET /health
```

示例：

```powershell
Invoke-RestMethod http://127.0.0.1:9092/health
```

返回：

```json
{
  "service": "go-fontman",
  "status": "ok"
}
```

## 字体匹配接口

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

说明：

- `imagePath` 是本机图片路径。
- `box` 是原图像素坐标，表示要识别字体的文字区域。
- `topK` 是返回候选数量。

返回：

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

PowerShell 请求示例：

```powershell
$body = @{
  imagePath = "D:\path\to\image.png"
  box = @{ x = 0; y = 0; w = 90; h = 43 }
  topK = 5
} | ConvertTo-Json -Depth 4

Invoke-RestMethod -Uri http://127.0.0.1:9092/match -Method Post -ContentType "application/json" -Body $body
```

## 命令行调试

不启动服务也可以使用 CLI 直接识别：

```powershell
.\fontman-cli.exe -image D:\path\to\image.png -box 0,0,90,43 -top-k 5
```
