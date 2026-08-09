package aravis

import (
	"image"
	"image/color"
)

type BayerRG struct {
	// Pix holds the image's pixels, in bayer order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

func NewBayerRG(r image.Rectangle) *BayerRG {
	w, h := r.Dx(), r.Dy()
	pix := make([]uint8, w*h)

	return &BayerRG{pix, w, r}
}

func (p *BayerRG) ColorModel() color.Model { return color.RGBAModel }

func (p *BayerRG) Bounds() image.Rectangle { return p.Rect }

// sample returns the raw sensor value at (x, y), clamping the coordinates to the
// image bounds (edge replication). Clamping keeps the neighbor lookups used by
// At from indexing outside Pix on the first/last row and column, which would
// otherwise panic.
func (p *BayerRG) sample(x, y int) uint8 {
	if x < p.Rect.Min.X {
		x = p.Rect.Min.X
	} else if x >= p.Rect.Max.X {
		x = p.Rect.Max.X - 1
	}

	if y < p.Rect.Min.Y {
		y = p.Rect.Min.Y
	} else if y >= p.Rect.Max.Y {
		y = p.Rect.Max.Y - 1
	}

	i := (y-p.Rect.Min.Y)*p.Stride + (x - p.Rect.Min.X)
	if i < 0 || i >= len(p.Pix) {
		return 0
	}

	return p.Pix[i]
}

// At returns an RGBA pixel with simple nearest-neighbor debayering.
func (p *BayerRG) At(x, y int) color.Color {
	if x&1 == 0 && y&1 == 0 {
		// top-left: red
		return color.RGBA{
			p.sample(x, y),
			p.sample(x+1, y),
			p.sample(x+1, y+1),
			0xff,
		}
	} else if x&1 == 1 && y&1 == 0 {
		// top-right: green
		return color.RGBA{
			p.sample(x-1, y),
			p.sample(x, y),
			p.sample(x, y+1),
			0xff,
		}
	} else if x&1 == 0 && y&1 == 1 {
		// bottom-left: green
		return color.RGBA{
			p.sample(x, y-1),
			p.sample(x, y),
			p.sample(x+1, y),
			0xff,
		}
	} else {
		// bottom-right: blue
		return color.RGBA{
			p.sample(x-1, y-1),
			p.sample(x, y-1),
			p.sample(x, y),
			0xff,
		}
	}
}
