package utils

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x + 1), G: uint8(y + 1), B: uint8(w*y + x), A: 255})
		}
	}

	return img
}

func assertMapping(t *testing.T, src, got image.Image, wantW, wantH int, mapping func(u, v int) (x, y int)) {
	t.Helper()

	require.NotNil(t, got)
	b := got.Bounds()
	require.Equal(t, wantW, b.Dx(), "output width")
	require.Equal(t, wantH, b.Dy(), "output height")

	for v := range wantH {
		for u := range wantW {
			x, y := mapping(u, v)
			wantR, wantG, wantB, wantA := src.At(x, y).RGBA()
			gotR, gotG, gotB, gotA := got.At(b.Min.X+u, b.Min.Y+v).RGBA()
			assert.Equal(t,
				[4]uint32{wantR, wantG, wantB, wantA},
				[4]uint32{gotR, gotG, gotB, gotA},
				"dst(%d,%d) should hold src(%d,%d)", u, v, x, y,
			)
		}
	}
}

func TestApplyOrientation(t *testing.T) {
	t.Parallel()

	const (
		srcW = 4
		srcH = 3
	)

	tests := []struct {
		name        string
		orientation uint16
		wantW       int
		wantH       int
		mapping     func(u, v int) (x, y int)
	}{
		{
			name:        "1 top-left is returned untouched",
			orientation: 1,
			wantW:       srcW,
			wantH:       srcH,
			mapping:     func(u, v int) (int, int) { return u, v },
		},
		{
			name:        "2 mirrors horizontally",
			orientation: 2,
			wantW:       srcW,
			wantH:       srcH,
			mapping:     func(u, v int) (int, int) { return srcW - 1 - u, v },
		},
		{
			name:        "3 rotates 180 degrees",
			orientation: 3,
			wantW:       srcW,
			wantH:       srcH,
			mapping:     func(u, v int) (int, int) { return srcW - 1 - u, srcH - 1 - v },
		},
		{
			name:        "4 mirrors vertically",
			orientation: 4,
			wantW:       srcW,
			wantH:       srcH,
			mapping:     func(u, v int) (int, int) { return u, srcH - 1 - v },
		},
		{
			name:        "5 transposes across the top-left diagonal",
			orientation: 5,
			wantW:       srcH,
			wantH:       srcW,
			mapping:     func(u, v int) (int, int) { return v, u },
		},
		{
			name:        "6 rotates 90 degrees clockwise",
			orientation: 6,
			wantW:       srcH,
			wantH:       srcW,
			mapping:     func(u, v int) (int, int) { return v, srcH - 1 - u },
		},
		{
			name:        "7 transposes across the bottom-right diagonal",
			orientation: 7,
			wantW:       srcH,
			wantH:       srcW,
			mapping:     func(u, v int) (int, int) { return srcW - 1 - v, srcH - 1 - u },
		},
		{
			name:        "8 rotates 270 degrees clockwise",
			orientation: 8,
			wantW:       srcH,
			wantH:       srcW,
			mapping:     func(u, v int) (int, int) { return srcW - 1 - v, u },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := newTestImage(srcW, srcH)
			got := ApplyOrientation(src, tt.orientation)

			assertMapping(t, src, got, tt.wantW, tt.wantH, tt.mapping)
		})
	}
}

func TestApplyOrientationRotationsAreNotIdentity(t *testing.T) {
	t.Parallel()

	src := newTestImage(3, 3)

	for orientation := uint16(2); orientation <= 8; orientation++ {
		t.Run(fmt.Sprintf("orientation %d", orientation), func(t *testing.T) {
			t.Parallel()

			got := ApplyOrientation(src, orientation)
			require.NotNil(t, got)

			differs := false

			for y := range 3 {
				for x := range 3 {
					if got.At(x, y) != src.At(x, y) {
						differs = true
					}
				}
			}

			assert.True(t, differs, "orientation %d must transform the image", orientation)
		})
	}
}

func TestApplyOrientationNilImage(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ApplyOrientation(nil, 1))
	assert.Nil(t, ApplyOrientation(nil, 6))
	assert.Nil(t, ApplyOrientation(nil, 0))
}

func TestApplyOrientationOutOfRangeIsANoOp(t *testing.T) {
	t.Parallel()

	for _, orientation := range []uint16{0, 9, 100, 65535} {
		t.Run(fmt.Sprintf("orientation %d", orientation), func(t *testing.T) {
			t.Parallel()

			src := newTestImage(4, 3)
			got := ApplyOrientation(src, orientation)

			assert.Same(t, src, got, "out-of-range orientation must return the original image")
		})
	}
}

func TestApplyOrientationIdentityDoesNotCopy(t *testing.T) {
	t.Parallel()

	src := newTestImage(4, 3)

	assert.Same(t, src, ApplyOrientation(src, 1), "orientation 1 must not allocate a copy")
}

// TestApplyOrientationOverwideImageIsSkipped requires transforms to return
// images wider than `10,000` pixels unchanged.
func TestApplyOrientationOverwideImageIsSkipped(t *testing.T) {
	t.Parallel()

	src := newTestImage(10001, 1)

	for _, orientation := range []uint16{2, 4, 6} {
		t.Run(fmt.Sprintf("orientation %d", orientation), func(t *testing.T) {
			t.Parallel()

			got := ApplyOrientation(src, orientation)

			assert.Same(t, src, got, "oversized image must be returned as-is")
			assert.Equal(t, 10001, got.Bounds().Dx())
			assert.Equal(t, 1, got.Bounds().Dy())
		})
	}
}

// TestApplyOrientationOvertallImageIsSkipped requires transforms to return
// images taller than `10,000` pixels unchanged.
func TestApplyOrientationOvertallImageIsSkipped(t *testing.T) {
	t.Parallel()

	src := newTestImage(1, 10001)

	for _, orientation := range []uint16{2, 4, 6} {
		t.Run(fmt.Sprintf("orientation %d", orientation), func(t *testing.T) {
			t.Parallel()

			got := ApplyOrientation(src, orientation)

			assert.Same(t, src, got, "oversized image must be returned as-is")
			assert.Equal(t, 1, got.Bounds().Dx())
			assert.Equal(t, 10001, got.Bounds().Dy())
		})
	}
}

// TestApplyOrientationNonZeroOriginBounds requires coordinate transforms to
// account for non-zero image-bound origins.
func TestApplyOrientationNonZeroOriginBounds(t *testing.T) {
	t.Parallel()

	full := newTestImage(4, 4)
	sub, ok := full.SubImage(image.Rect(2, 2, 4, 4)).(*image.RGBA)
	require.True(t, ok)
	require.Equal(t, image.Pt(2, 2), sub.Bounds().Min)

	t.Run("flip horizontal honours the offset origin", func(t *testing.T) {
		t.Parallel()

		got := ApplyOrientation(sub, 2)

		assertMapping(t, sub, got, 2, 2, func(u, v int) (int, int) { return 3 - u, 2 + v })
	})

	t.Run("flip vertical honours the offset origin", func(t *testing.T) {
		t.Parallel()

		got := ApplyOrientation(sub, 4)

		assertMapping(t, sub, got, 2, 2, func(u, v int) (int, int) { return 2 + u, 3 - v })
	})

	t.Run("rotate 90 honours the offset origin", func(t *testing.T) {
		t.Parallel()

		got := ApplyOrientation(sub, 6)
		assertMapping(t, sub, got, 2, 2, func(u, v int) (int, int) { return 2 + v, 3 - u })
	})
}
