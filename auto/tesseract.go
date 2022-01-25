//go:build tesseract

package auto

import (
	"bytes"
	"image"

	"golang.org/x/image/tiff"
)

func TextSupported() bool { return true }

func (i *ImageSearch) Text(region image.Rectangle, minConfidence float64) ([]Word, error) {
	sub := image.NewGray(image.Rect(0, 0, region.Dx(), region.Dy()))
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			o := i.c.PixOffset(x, y)
			v := i.c.Pix[o+0]/3 + i.c.Pix[o+1]/3 + i.c.Pix[o+2]/3
			o = sub.PixOffset(x-region.Min.X, y-region.Min.Y)
			sub.Pix[o] = v
		}
	}

	region = sub.Bounds()
	var val uint8
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			o := sub.PixOffset(x, y)
			v := sub.Pix[o]
			val = 0
			if v > 127 {
				val = 255
			}
			sub.Pix[o] = val
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(sub.Pix)+200))
	err := tiff.Encode(buf, sub, &tiff.Options{})
	if err != nil {
		return nil, err
	}

	if err := i.tess.SetImageFromBytes(buf.Bytes()); err != nil {
		return nil, err
	}
	bb, err := i.tess.GetBoundingBoxes(3)
	if err != nil {
		return nil, err
	}

	n := make([]Word, 0, len(bb))
	for _, b := range bb {
		if b.Confidence < minConfidence {
			continue
		}
		n = append(n, Word{b.Box, b.Word, b.Confidence})
	}

	return n, nil
}
