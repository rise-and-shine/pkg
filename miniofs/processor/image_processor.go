package processor

import (
	"bytes"

	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/rise-and-shine/pkg/miniofs/types"
)

type ImageProcessor struct{}

// New creates a new ImageProcessor
func New() *ImageProcessor {
	return &ImageProcessor{}
}

// ResizeWithFormat resizes an image to different sizes and encodes it with the specified format
func (p *ImageProcessor) ResizeWithFormat(img image.Image, format string) (map[types.ImageSize][]byte, error) {
	m := make(map[types.ImageSize][]byte)
	format = strings.ToLower(format)

	// Original
	orig := new(bytes.Buffer)
	if err := encodeImage(orig, img, format); err != nil {
		return nil, types.ErrResizeFailed
	}
	m[types.Original] = orig.Bytes()

	// Medium
	med := imaging.Resize(img, 800, 0, imaging.Lanczos)
	medBuf := new(bytes.Buffer)
	if err := encodeImage(medBuf, med, format); err != nil {
		return nil, types.ErrResizeFailed
	}
	m[types.Medium] = medBuf.Bytes()

	// Small
	small := imaging.Resize(img, 300, 0, imaging.Lanczos)
	smallBuf := new(bytes.Buffer)
	if err := encodeImage(smallBuf, small, format); err != nil {
		return nil, types.ErrResizeFailed
	}
	m[types.Small] = smallBuf.Bytes()

	return m, nil
}

// encodeImage encodes an image with the specified format
func encodeImage(buf *bytes.Buffer, img image.Image, format string) error {
	switch format {
	case "jpg", "jpeg", ".jpg", ".jpeg":
		return jpeg.Encode(buf, img, nil)
	case "png", ".png":
		return png.Encode(buf, img)
	default:
		return jpeg.Encode(buf, img, nil) // Default to JPEG
	}
}
