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
