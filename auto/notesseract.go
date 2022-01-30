//go:build !tesseract

package auto

import (
	"errors"
	"image"
)

func newTess() interface{}                   { return nil }
func (i *ImageSearch) closeTesseract() error { return nil }

func TextSupported() bool { return false }

func (i *ImageSearch) Text(region image.Rectangle, minConfidence float64) ([]Word, error) {
	return nil, errors.New("non tesseract build")
}
