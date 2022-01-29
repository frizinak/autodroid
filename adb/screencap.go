package adb

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
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
	r, w := io.Pipe()
	var gerr error
	go func() {
		gerr = adb.Run("screencap", w, nil)
		w.Close()
	}()
	img, err := decodeImageReader(r)
	if gerr != nil {
		return nil, gerr
	}
	return img, err
}

func (adb *ADB) ScreencapContinuous(cb func(*image.NRGBA) error) error {
	var img *image.NRGBA
	err, reconnect := func() (error, bool) {
		var err error
		for {
			_, err = fmt.Fprintln(adb.stdin, "screencap")
			if err != nil {
				return err, true
			}
			img, err = decodeImageReader(adb.stdout)
			if err != nil {
				return err, true
			}
			if err = cb(img); err != nil {
				return err, false
			}
		}
	}()

	if err != nil && reconnect {
		_ = adb.Close()
		_ = adb.Init()
	}

	return err
}

func decodeImageReader(r io.Reader) (*image.NRGBA, error) {
	var err error
	buf := make([]byte, 16)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	_w := binary.LittleEndian.Uint32(buf[0:4])
	_h := binary.LittleEndian.Uint32(buf[4:8])
	_p := binary.LittleEndian.Uint32(buf[8:12])
	// +unknown byte

	if err != nil {
		return nil, err
	}

	w, h, p := int(_w), int(_h), PixFmt(_p)
	switch p {
	case RGBA_8888:
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		o := 0
		for {
			n, err := r.Read(img.Pix[o:])
			o += n
			if o == len(img.Pix) {
				break
			}
			if err != nil {
				return nil, err
			}
		}
		return img, nil
	}

	return nil, fmt.Errorf("pixel format 0x%x not implemented", p)
}
