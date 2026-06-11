package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ksamwang/go-fontman/internal/imageprep"
	"github.com/ksamwang/go-fontman/internal/matcher"
)

func main() {
	var addr string
	var port int
	var manifestPath string
	var dllPath string
	flag.StringVar(&addr, "addr", "", "listen address, overrides -port when set")
	flag.IntVar(&port, "port", 9092, "listen port on 127.0.0.1")
	flag.StringVar(&manifestPath, "manifest", "runtime/font_ai/manifest.json", "runtime manifest path")
	flag.StringVar(&dllPath, "onnxruntime-dll", defaultONNXRuntimeDLL(), "onnxruntime.dll path")
	flag.Parse()
	if strings.TrimSpace(addr) == "" {
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	m, err := matcher.New(manifestPath, dllPath)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()

	page := template.Must(template.New("index").Parse(indexHTML))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Execute(w, nil)
	})
	mux.HandleFunc("/api/match", func(w http.ResponseWriter, r *http.Request) {
		handleMatch(w, r, m)
	})
	mux.HandleFunc("/match", func(w http.ResponseWriter, r *http.Request) {
		handleMatch(w, r, m)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, map[string]any{
			"status":  "ok",
			"service": "go-fontman",
		})
	})

	log.Printf("fontman service listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleMatch(w http.ResponseWriter, r *http.Request, m *matcher.Matcher) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		handleJSONMatch(w, r, m)
		return
	}
	handleMultipartMatch(w, r, m)
}

type matchRequest struct {
	ImagePath string        `json:"imagePath"`
	Box       imageprep.Box `json:"box"`
	TopK      int           `json:"topK"`
}

func handleJSONMatch(w http.ResponseWriter, r *http.Request, m *matcher.Matcher) {
	defer r.Body.Close()
	var request matchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(request.ImagePath) == "" {
		http.Error(w, "imagePath is required", http.StatusBadRequest)
		return
	}
	if request.Box.W <= 0 || request.Box.H <= 0 || request.Box.X < 0 || request.Box.Y < 0 {
		http.Error(w, "invalid box", http.StatusBadRequest)
		return
	}
	topK := request.TopK
	if topK <= 0 {
		topK = 5
	}
	results, err := m.MatchImage(request.ImagePath, request.Box, topK)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"box":     request.Box,
		"results": results,
	})
}

func handleMultipartMatch(w http.ResponseWriter, r *http.Request, m *matcher.Matcher) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "go-fontman-*"+filepath.Ext(header.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.ReadFrom(file); err != nil {
		_ = tmp.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmp.Close()

	box, err := parseFormBox(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	topK := parseIntDefault(r.FormValue("topK"), 5)
	results, err := m.MatchImage(tmp.Name(), box, topK)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"box":     box,
		"results": results,
	})
}

func parseFormBox(r *http.Request) (imageprep.Box, error) {
	x := parseIntDefault(r.FormValue("x"), -1)
	y := parseIntDefault(r.FormValue("y"), -1)
	w := parseIntDefault(r.FormValue("w"), -1)
	h := parseIntDefault(r.FormValue("h"), -1)
	if x < 0 || y < 0 || w <= 0 || h <= 0 {
		return imageprep.Box{}, fmt.Errorf("invalid selection box")
	}
	return imageprep.Box{X: x, Y: y, W: w, H: h}, nil
}

func parseIntDefault(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return n
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
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

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Go Fontman Test</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1f2933; background: #f6f7f9; }
    .shell { display: grid; grid-template-columns: minmax(420px, 1fr) 420px; gap: 16px; min-height: 100vh; padding: 16px; box-sizing: border-box; }
    .panel { background: #fff; border: 1px solid #d9dee7; border-radius: 8px; min-height: 0; }
    .toolbar { display: flex; align-items: center; gap: 10px; padding: 12px; border-bottom: 1px solid #e5e8ef; }
    button, input[type=file]::file-selector-button { border: 1px solid #b8c2d2; background: #fff; color: #1f2933; border-radius: 6px; padding: 7px 10px; cursor: pointer; }
    button:disabled { opacity: .5; cursor: not-allowed; }
    .canvas-wrap { position: relative; height: calc(100vh - 98px); overflow: auto; padding: 12px; box-sizing: border-box; }
    canvas { display: block; background: #fff; border: 1px solid #cfd6e2; cursor: crosshair; }
    .side { display: flex; flex-direction: column; min-height: 0; }
    .meta { padding: 12px; border-bottom: 1px solid #e5e8ef; font-size: 13px; line-height: 1.6; }
    .results { overflow: auto; padding: 12px; display: grid; gap: 10px; }
    .candidate { border: 1px solid #d9dee7; border-radius: 8px; padding: 10px; background: #fff; }
    .candidate strong { display: block; margin-bottom: 6px; }
    .kv { color: #52606d; font-size: 12px; overflow-wrap: anywhere; line-height: 1.5; }
    .error { color: #b42318; white-space: pre-wrap; }
    @media (max-width: 980px) { .shell { grid-template-columns: 1fr; } .canvas-wrap { height: 58vh; } }
  </style>
</head>
<body>
  <div class="shell">
    <section class="panel">
      <div class="toolbar">
        <input id="file" type="file" accept="image/*" />
        <button id="match" disabled>识别选区</button>
        <button id="clear" disabled>清除选区</button>
      </div>
      <div class="canvas-wrap">
        <canvas id="canvas"></canvas>
      </div>
    </section>
    <aside class="panel side">
      <div class="meta" id="meta">上传图片后，在画布上拖出文字区域。</div>
      <div class="results" id="results"></div>
    </aside>
  </div>
  <script>
    const fileInput = document.getElementById('file')
    const matchButton = document.getElementById('match')
    const clearButton = document.getElementById('clear')
    const canvas = document.getElementById('canvas')
    const ctx = canvas.getContext('2d')
    const meta = document.getElementById('meta')
    const results = document.getElementById('results')
    let file = null
    let image = null
    let selection = null
    let dragStart = null

    fileInput.addEventListener('change', () => {
      file = fileInput.files[0]
      selection = null
      results.innerHTML = ''
      if (!file) return
      const img = new Image()
      img.onload = () => {
        image = img
        canvas.width = img.naturalWidth
        canvas.height = img.naturalHeight
        draw()
        updateMeta()
      }
      img.src = URL.createObjectURL(file)
    })

    canvas.addEventListener('mousedown', event => {
      if (!image) return
      dragStart = point(event)
      selection = { x: dragStart.x, y: dragStart.y, w: 0, h: 0 }
      draw()
    })
    canvas.addEventListener('mousemove', event => {
      if (!dragStart) return
      const p = point(event)
      selection = normalizeRect(dragStart, p)
      draw()
      updateMeta()
    })
    window.addEventListener('mouseup', () => {
      if (!dragStart) return
      dragStart = null
      selection = selection && selection.w > 2 && selection.h > 2 ? selection : null
      draw()
      updateMeta()
    })
    clearButton.addEventListener('click', () => {
      selection = null
      results.innerHTML = ''
      draw()
      updateMeta()
    })
    matchButton.addEventListener('click', async () => {
      if (!file || !selection) return
      matchButton.disabled = true
      results.innerHTML = '<div class="kv">识别中...</div>'
      const form = new FormData()
      form.append('image', file)
      form.append('x', Math.round(selection.x))
      form.append('y', Math.round(selection.y))
      form.append('w', Math.round(selection.w))
      form.append('h', Math.round(selection.h))
      form.append('topK', 8)
      try {
        const response = await fetch('/api/match', { method: 'POST', body: form })
        const text = await response.text()
        if (!response.ok) throw new Error(text)
        renderResults(JSON.parse(text).results || [])
      } catch (err) {
        results.innerHTML = '<div class="error">' + escapeHTML(String(err.message || err)) + '</div>'
      } finally {
        updateMeta()
      }
    })

    function draw() {
      if (!image) return
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.drawImage(image, 0, 0)
      if (selection) {
        ctx.save()
        ctx.strokeStyle = '#0b6bcb'
        ctx.lineWidth = 2
        ctx.fillStyle = 'rgba(11, 107, 203, 0.12)'
        ctx.fillRect(selection.x, selection.y, selection.w, selection.h)
        ctx.strokeRect(selection.x, selection.y, selection.w, selection.h)
        ctx.restore()
      }
    }

    function updateMeta() {
      const imageText = image ? '图片：' + image.naturalWidth + ' x ' + image.naturalHeight : '上传图片后，在画布上拖出文字区域。'
      const boxText = selection ? '选区：' + Math.round(selection.x) + ', ' + Math.round(selection.y) + ', ' + Math.round(selection.w) + ', ' + Math.round(selection.h) : '选区：未选择'
      meta.textContent = image ? imageText + '\n' + boxText : imageText
      matchButton.disabled = !file || !selection
      clearButton.disabled = !selection
    }

    function renderResults(items) {
      if (!items.length) {
        results.innerHTML = '<div class="kv">没有候选结果。</div>'
        return
      }
      results.innerHTML = items.map(item => {
        const font = item.font || {}
        return '<div class="candidate">' +
          '<strong>#' + item.rank + ' ' + escapeHTML(font.name || '') + '</strong>' +
          '<div class="kv">score: ' + Number(item.score || 0).toFixed(4) + '</div>' +
          '<div class="kv">faceIndex: ' + (font.faceIndex ?? '') + ' / ' + escapeHTML(font.faceName || '') + '</div>' +
          '<div class="kv">family: ' + escapeHTML(font.familyName || '') + ' / weight: ' + escapeHTML(font.weightName || '') + '</div>' +
          '<div class="kv">style: ' + escapeHTML(font.styleGroup || '') + ' / scope: ' + escapeHTML(font.scriptScope || '') + '</div>' +
          '<div class="kv">path: ' + escapeHTML(font.path || '') + '</div>' +
        '</div>'
      }).join('')
    }

    function point(event) {
      const rect = canvas.getBoundingClientRect()
      return {
        x: Math.max(0, Math.min(canvas.width, (event.clientX - rect.left) * canvas.width / rect.width)),
        y: Math.max(0, Math.min(canvas.height, (event.clientY - rect.top) * canvas.height / rect.height))
      }
    }

    function normalizeRect(a, b) {
      const x = Math.min(a.x, b.x)
      const y = Math.min(a.y, b.y)
      return { x, y, w: Math.abs(a.x - b.x), h: Math.abs(a.y - b.y) }
    }

    function escapeHTML(value) {
      return value.replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]))
    }
  </script>
</body>
</html>`
