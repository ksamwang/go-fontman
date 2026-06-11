package imageprep

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
)

type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

func TensorFromFile(path string, box Box, width, height int) ([]float32, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return Tensor(img, box, width, height)
}

func Tensor(img image.Image, box Box, width, height int) ([]float32, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid target size %dx%d", width, height)
	}
	cropped, err := crop(img, box)
	if err != nil {
		return nil, err
	}
	fitted := fitCanvas(cropped, width, height)
	return toNCHW(fitted), nil
}

func crop(img image.Image, box Box) (image.Image, error) {
	bounds := img.Bounds()
	if box.W <= 0 || box.H <= 0 {
		return nil, fmt.Errorf("invalid box %+v", box)
	}
	rect := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H).Intersect(bounds)
	if rect.Empty() {
		return nil, fmt.Errorf("box %+v is outside image bounds %v", box, bounds)
	}
	out := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(out, out.Bounds(), img, rect.Min, draw.Src)
	return out, nil
}

func fitCanvas(img image.Image, width, height int) *image.RGBA {
	sourceBounds := img.Bounds()
	scale := math.Min(float64(width)/float64(sourceBounds.Dx()), float64(height)/float64(sourceBounds.Dy()))
	newWidth := max(1, int(math.Round(float64(sourceBounds.Dx())*scale)))
	newHeight := max(1, int(math.Round(float64(sourceBounds.Dy())*scale)))

	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), img, sourceBounds, xdraw.Over, nil)

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	x := (width - newWidth) / 2
	y := (height - newHeight) / 2
	draw.Draw(canvas, image.Rect(x, y, x+newWidth, y+newHeight), resized, image.Point{}, draw.Src)
	return canvas
}

func toNCHW(img *image.RGBA) []float32 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	tensor := make([]float32, 3*width*height)
	channelSize := width * height
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			i := y*width + x
			tensor[i] = normalize(img.Pix[offset])
			tensor[channelSize+i] = normalize(img.Pix[offset+1])
			tensor[channelSize*2+i] = normalize(img.Pix[offset+2])
		}
	}
	return tensor
}

func normalize(value byte) float32 {
	return (float32(value)/255.0 - 0.5) / 0.5
}
