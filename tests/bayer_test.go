package tests

import (
	"image"
	"image/color"
	"testing"

	aravis "github.com/MeKo-Christian/go-aravis"
)

// makeBayer builds a w x h BayerRG whose raw sample at (x, y) equals y*w+x, so
// the values that At should read for each neighbor are easy to predict.
func makeBayer(w, h int) *aravis.BayerRG {
	img := aravis.NewBayerRG(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}

	return img
}

// TestBayerNoPanicOnEdges guards the out-of-bounds panic that the original At
// hit on the last row/column: it indexed x+1/y+1 without bounds checks. Every
// pixel of several images (including odd dimensions) must be readable.
func TestBayerNoPanicOnEdges(t *testing.T) {
	for _, dim := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {5, 3}, {4, 7}} {
		w, h := dim[0], dim[1]
		img := makeBayer(w, h)

		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				_ = img.At(x, y) // must not panic
			}
		}
	}
}

// TestBayerAlphaOpaque checks that debayered pixels are opaque. The original
// code set alpha to 0, producing a fully transparent image.
func TestBayerAlphaOpaque(t *testing.T) {
	img := makeBayer(4, 4)

	c, ok := img.At(0, 0).(color.RGBA)
	if !ok {
		t.Fatalf("At did not return color.RGBA")
	}

	if c.A != 0xff {
		t.Errorf("alpha = %d, want 255 (opaque)", c.A)
	}
}

// TestBayerEdgePhasePreserved verifies that a neighbor that falls outside the
// image is reflected to a site of the same Bayer color phase, not clamped onto
// the current site. On the last column of an odd-width image, the green channel
// of a red site must come from a green (odd) column, not from the red site.
func TestBayerEdgePhasePreserved(t *testing.T) {
	img := makeBayer(3, 3) // Pix[i] = i, Stride = 3

	// (2,0) is a red site on the last column. Its green neighbor (3,0) is out of
	// bounds: reflection lands on column 1 (value 1); clamping would wrongly land
	// on column 2 (value 2, the red site itself).
	c, ok := img.At(2, 0).(color.RGBA)
	if !ok {
		t.Fatalf("At did not return color.RGBA")
	}

	if want := (color.RGBA{R: 2, G: 1, B: 4, A: 0xff}); c != want {
		t.Errorf("At(2,0) = %+v, want %+v (green must reflect to column 1, not clamp to the red site)", c, want)
	}
}

// makeBayerAt builds a w x h BayerRG with the given origin, filled with the
// same raw samples as makeBayer(w, h), so the two must debayer identically.
func makeBayerAt(originX, originY, w, h int) *aravis.BayerRG {
	img := aravis.NewBayerRG(image.Rect(originX, originY, originX+w, originY+h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}

	return img
}

// TestBayerOddOriginPhase pins the CFA phase to the rectangle's origin rather
// than to absolute coordinates. sample indexes Pix relative to Rect.Min, so a
// rect with an odd Min.X/Min.Y must still read its top-left sample as red.
// Deriving the phase from x&1/y&1 instead swapped red and blue and pulled the
// green samples from the wrong neighbors for every such image.
func TestBayerOddOriginPhase(t *testing.T) {
	const w, h = 4, 4

	origins := [][2]int{{1, 1}, {1, 0}, {0, 1}, {3, 5}, {-1, -1}}

	base := makeBayer(w, h)

	for _, o := range origins {
		img := makeBayerAt(o[0], o[1], w, h)

		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				got := img.At(o[0]+x, o[1]+y)
				want := base.At(x, y)

				if got != want {
					t.Errorf("origin (%d,%d): At(%d,%d) = %+v, want %+v (same raw samples as the zero-origin image)",
						o[0], o[1], o[0]+x, o[1]+y, got, want)
				}
			}
		}
	}
}

// TestBayerOddOriginTopLeftIsRed states the same invariant directly: the first
// sample of an odd-origin image is a red site, so R comes from it, G from the
// sample to its right and B from the diagonal neighbor.
func TestBayerOddOriginTopLeftIsRed(t *testing.T) {
	img := makeBayerAt(1, 1, 4, 4) // Pix[i] = i, Stride = 4

	c, ok := img.At(1, 1).(color.RGBA)
	if !ok {
		t.Fatalf("At did not return color.RGBA")
	}

	if want := (color.RGBA{R: 0, G: 1, B: 5, A: 0xff}); c != want {
		t.Errorf("At(1,1) = %+v, want %+v (Rect.Min is the red site regardless of its parity)", c, want)
	}
}
