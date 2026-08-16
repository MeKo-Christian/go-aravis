package aravis

import (
	"image"
	"image/color"
)

// BayerRG is an image.Image view over a raw, single-plane Bayer frame in RGGB
// order: the sample at an even column and even row is red, the two mixed-parity
// sites are green, and the odd/odd site is blue. Pix holds one 8-bit sensor
// sample per pixel; the RGBA values handed out by At are reconstructed on the
// fly by simple nearest-neighbor debayering, so no separate RGB copy is kept.
//
// Missing color channels are taken from the nearest neighboring site of the
// required color, and neighbors that fall outside Rect are mirrored back across
// the edge rather than clamped, which keeps them on the correct CFA phase.
//
// Caveat: At derives the CFA phase from the absolute coordinates (x&1, y&1)
// while sample indexes Pix relative to Rect.Min. The two therefore agree only
// when Rect.Min.X and Rect.Min.Y are both even; for a rect with an odd origin
// the documented RGGB phase does not hold and the reconstructed colors are
// shifted by one site.
type BayerRG struct {
	// Pix holds the image's pixels, in bayer order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

// NewBayerRG returns a new BayerRG with bounds r and a zeroed Pix buffer of
// r.Dx()*r.Dy() bytes, one byte per sensor sample, with Stride set to r.Dx().
// Callers typically overwrite Pix with the raw frame data from the camera.
//
// Because At interprets the CFA phase from absolute coordinates, pass a
// rectangle whose origin is even (for example image.Rect(0, 0, w, h)) if the
// data is to be read as RGGB.
func NewBayerRG(r image.Rectangle) *BayerRG {
	w, h := r.Dx(), r.Dy()
	pix := make([]uint8, w*h)

	return &BayerRG{pix, w, r}
}

// ColorModel returns color.RGBAModel, the model of the debayered pixels
// produced by At. It does not describe the raw single-channel data in Pix.
func (p *BayerRG) ColorModel() color.Model { return color.RGBAModel }

// Bounds returns the image's bounds, Rect.
func (p *BayerRG) Bounds() image.Rectangle { return p.Rect }

// sample returns the raw sensor value at (x, y). Coordinates outside the image
// are reflected back across the nearest edge (mirror), not clamped. Clamping
// would collapse an out-of-bounds neighbor onto the current Bayer site, which
// carries the wrong color phase — e.g. on the last column of an odd-width image
// it would feed a red sample where a green one is needed. Reflection lands on a
// site of the same CFA parity as the requested neighbor, so the color phase is
// preserved, and it also keeps the lookups from indexing outside Pix (which
// would otherwise panic).
func (p *BayerRG) sample(x, y int) uint8 {
	x = reflectCoord(x, p.Rect.Min.X, p.Rect.Max.X)
	y = reflectCoord(y, p.Rect.Min.Y, p.Rect.Max.Y)

	i := (y-p.Rect.Min.Y)*p.Stride + (x - p.Rect.Min.X)
	if i < 0 || i >= len(p.Pix) {
		return 0
	}

	return p.Pix[i]
}

// reflectCoord maps v into the half-open range [lo, hi) by mirroring across the
// edges. Because each reflection reverses a unit step, the parity of the result
// relative to the edge is preserved, which keeps a Bayer neighbor on its color
// phase. A degenerate span (<= 1) has no valid mirror, so it returns lo.
func reflectCoord(v, lo, hi int) int {
	if hi-lo <= 1 {
		return lo
	}

	for v < lo || v >= hi {
		if v < lo {
			v = 2*lo - v
		} else {
			v = 2*(hi-1) - v
		}
	}

	return v
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
