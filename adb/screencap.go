package adb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
)

type PixFmt uint32

const (
	A_8          PixFmt = 0x00000008
	JPEG         PixFmt = 0x00000100
	LA_88        PixFmt = 0x0000000a
	L_8          PixFmt = 0x00000009
	OPAQUE       PixFmt = 0xffffffff
	RGBA_1010102 PixFmt = 0x0000002b
	RGBA_4444    PixFmt = 0x00000007
	RGBA_5551    PixFmt = 0x00000006
	RGBA_8888    PixFmt = 0x00000001
	RGBA_F16     PixFmt = 0x00000016
	RGBX_8888    PixFmt = 0x00000002
	RGB_332      PixFmt = 0x0000000b
	RGB_565      PixFmt = 0x00000004
	RGB_888      PixFmt = 0x00000003
	TRANSLUCENT  PixFmt = 0xfffffffd
	TRANSPARENT  PixFmt = 0xfffffffe
	YCbCr_420_SP PixFmt = 0x00000011
	YCbCr_422_I  PixFmt = 0x00000014
	YCbCr_422_SP PixFmt = 0x00000010
)

func (adb *ADB) Screencap() (*image.NRGBA, error) {
	img := bytes.NewBuffer(nil)
	if err := adb.Run("screencap", img, nil); err != nil {
		return nil, err
	}
	return decodeImage(img.Bytes())
}

func decodeImage(input []byte) (*image.NRGBA, error) {
	_w := binary.LittleEndian.Uint32(input[0+0 : 0+4])
	_h := binary.LittleEndian.Uint32(input[0+4 : 4+4])
	_p := binary.LittleEndian.Uint32(input[0+8 : 4+8])

	// unknown byte
	// _u := binary.LittleEndian.Uint32(input[0+12 : 4+12])

	w, h, p := int(_w), int(_h), PixFmt(_p)
	switch p {
	case RGBA_8888:
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		img.Pix = input[16:]
		return img, nil
	}

	return nil, fmt.Errorf("pixel format 0x%x not implemented", p)
}
