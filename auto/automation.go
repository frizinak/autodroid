package auto

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"

	"github.com/otiai10/gosseract/v2"
)

func ToGray(img *image.NRGBA) *image.Gray {
	b := img.Bounds()
	gray := image.NewGray(b)
	for i := range gray.Pix {
		gray.Pix[i] = img.Pix[i*4+0]/3 +
			img.Pix[i*4+1]/3 +
			img.Pix[i*4+2]/3
	}
	return gray
}

func diff(a, b uint8) uint8 {
	if b > a {
		return b - a
	}
	return a - b
}

type Word struct {
	Position   image.Rectangle
	Word       string
	Confidence float64
}

type SearchResult struct {
	image.Rectangle
	Diff int
}

func (s SearchResult) String() string {
	return fmt.Sprintf("%dx%d+%d+%d", s.Dx(), s.Dy(), s.Min.X, s.Min.Y)
}

type SearchResults []SearchResult

func (s SearchResults) Len() int           { return len(s) }
func (s SearchResults) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SearchResults) Less(i, j int) bool { return s[i].Diff < s[j].Diff }

func searchGray(haystack, needle *image.Gray, interval float64, maxDiff uint8) SearchResults {
	h := haystack.Bounds()
	n := needle.Bounds()
	dx, dy := n.Dx(), n.Dy()
	ivx, ivy := int(interval*float64(dx)), int(interval*float64(dy))
	if ivx < 1 || ivy < 1 {
		ivx, ivy = 1, 1
	}
	results := make(SearchResults, 0, 1)
	for y := h.Min.Y; y <= h.Max.Y-dy; y++ {
	inner:
		for x := h.Min.X; x <= h.Max.X-dx; x++ {
			totalDiff := 0
			for ys := n.Min.Y; ys < n.Max.Y; ys += ivy {
				for xs := n.Min.X; xs < n.Max.X; xs += ivx {
					o := haystack.PixOffset(x+xs-n.Min.X, y+ys-n.Min.Y)
					v := haystack.Pix[o]
					o = needle.PixOffset(xs, ys)
					vs := needle.Pix[o]
					diff := diff(v, vs)
					if diff > maxDiff {
						continue inner
					}
					totalDiff += int(diff)
				}
			}
			nr := image.Rect(x, y, x+dx, y+dy)
			ix := len(results) - 1
			if len(results) != 0 && results[ix].Overlaps(nr) {
				results[ix].Rectangle = results[ix].Union(nr)
				continue
			}
			results = append(results, SearchResult{nr, totalDiff})
		}
	}

	return results
}

func search(haystack, needle *image.NRGBA, interval float64, maxDiff uint8) SearchResults {
	h := haystack.Bounds()
	n := needle.Bounds()
	dx, dy := n.Dx(), n.Dy()
	ivx, ivy := int(interval*float64(dx)), int(interval*float64(dy))
	if ivx < 1 || ivy < 1 {
		ivx, ivy = 1, 1
	}
	results := make(SearchResults, 0, 1)
	for y := h.Min.Y; y <= h.Max.Y-dy; y++ {
	inner:
		for x := h.Min.X; x <= h.Max.X-dx; x++ {
			totalDiff := 0
			for ys := n.Min.Y; ys < n.Max.Y; ys += ivy {
				for xs := n.Min.X; xs < n.Max.X; xs += ivx {
					o := haystack.PixOffset(x+xs-n.Min.X, y+ys-n.Min.Y)
					r, g, b := haystack.Pix[o+0], haystack.Pix[o+1], haystack.Pix[o+2]
					o = needle.PixOffset(xs, ys)
					rs, gs, bs := needle.Pix[o+0], needle.Pix[o+1], needle.Pix[o+2]
					diff := diff(r, rs)/3 + diff(g, gs)/3 + diff(b, bs)/3
					if diff > maxDiff {
						continue inner
					}
					totalDiff += int(diff)
				}
			}
			nr := image.Rect(x, y, x+dx, y+dy)
			ix := len(results) - 1
			if len(results) != 0 && results[ix].Overlaps(nr) {
				results[ix].Rectangle = results[ix].Union(nr)
				continue
			}
			results = append(results, SearchResult{nr, totalDiff})
		}
	}

	return results
}

type ImageSearch struct {
	rw   sync.RWMutex
	c    *image.NRGBA
	g    *image.Gray
	b    image.Rectangle
	pix  map[string]*pixel
	tess *gosseract.Client
}

func NewImageSearch() *ImageSearch {
	return &ImageSearch{
		pix:  make(map[string]*pixel),
		tess: gosseract.NewClient(),
	}
}

func (i *ImageSearch) Close() error {
	return i.tess.Close()
}

func (i *ImageSearch) Set(img *image.NRGBA) {
	i.c = img
	i.b = img.Bounds()
	i.g = nil
}

func (i *ImageSearch) Bounds() image.Rectangle {
	return i.b
}

func (i *ImageSearch) Gray() *image.Gray {
	if i.g != nil {
		return i.g
	}
	i.rw.Lock()
	i.g = ToGray(i.c)
	i.rw.Unlock()
	return i.g
}

type RR struct {
	Min, Max struct {
		X float64
		Y float64
	}
}

func (rr RR) String() string {
	return fmt.Sprintf("(%.2f %.2f)-(%.2f %.2f)", rr.Min.X, rr.Min.Y, rr.Max.X, rr.Max.Y)
}

func RelativeRect(x0, y0, x1, y1 float64) RR {
	var r RR
	r.Min.X, r.Min.Y, r.Max.X, r.Max.Y = x0, y0, x1, y1
	return r
}

func (i *ImageSearch) Relative(r RR) image.Rectangle {
	n := i.b
	w, h := float64(i.b.Dx()), float64(i.b.Dy())
	n.Min.X = int(r.Min.X * w)
	n.Min.Y = int(r.Min.Y * h)
	n.Max.X = int(r.Max.X * w)
	n.Max.Y = int(r.Max.Y * h)
	return n
}

type pixel struct {
	x, y  int
	value bool
	se    *pixel
	e     *pixel
	s     *pixel
}

func (i *ImageSearch) pixels(b image.Rectangle) *pixel {
	id := b.String()
	px, ok := i.pix[id]
	if ok {
		return px
	}

	root := &pixel{x: b.Min.X}

	current := root
	for y := b.Min.Y; y < b.Max.Y; y++ {
		current.y = y
		var s *pixel
		if y != b.Max.Y-1 {
			s = &pixel{x: b.Min.X, y: y + 1}
			current.s = s
		}
		for x := b.Min.X; x < b.Max.X-1; x++ {
			current.e = &pixel{x: x + 1, y: y}
			current = current.e
		}

		current = s
	}

	current = root
	for y := b.Min.Y; y < b.Max.Y-1; y++ {
		s := current.s
		nextrow := s
		for x := b.Min.X; x < b.Max.X-1; x++ {
			current.s = nextrow
			nextrow = nextrow.e
			current = current.e
		}
		current = s
	}

	current = root
	for y := b.Min.Y; y < b.Max.Y-1; y++ {
		for x := b.Min.X; x < b.Max.X-1; x++ {
			current.se = current.e.s
		}
	}

	i.pix[id] = root
	return root
}

func (i *ImageSearch) Color(x, y int) color.NRGBA {
	return i.c.At(x, y).(color.NRGBA)
}

func (i *ImageSearch) SearchCluster(region image.Rectangle, clr color.NRGBA, minSize, maxSize int, maxDiff uint8) SearchResults {
	source := i.c.SubImage(region).(*image.NRGBA)
	b := source.Bounds()

	i.rw.Lock()
	defer i.rw.Unlock()
	root := i.pixels(b)

	for py := root; py != nil; py = py.s {
		for px := py; px != nil; px = px.e {
			o := source.PixOffset(px.x, px.y)
			c := source.Pix[o : o+3]
			diff := diff(c[0], clr.R)/3 + diff(c[1], clr.G)/3 + diff(c[2], clr.B)/3
			px.value = diff <= maxDiff
			if diff <= maxDiff {
				px.value = true
			}
		}
	}

	var a func(px *pixel) *image.Rectangle
	a = func(px *pixel) *image.Rectangle {
		if px == nil || !px.value {
			return nil
		}

		px.value = false
		r := image.Rect(px.x, px.y, px.x+1, px.y+1)
		se := a(px.se)
		if se != nil {
			r = r.Union(*se)
		}
		e := a(px.e)
		if e != nil {
			r = r.Union(*e)
		}
		s := a(px.s)
		if s != nil {
			r = r.Union(*s)
		}
		return &r
	}

	re := make(SearchResults, 0)
	for py := root; py != nil; py = py.s {
		for px := py; px != nil; px = px.e {
			r := a(px)
			if r != nil {
				re = append(re, SearchResult{Rectangle: *r})
			}
		}
	}

	return re
}

func (i *ImageSearch) Search(region image.Rectangle, sub image.Image, maxDiff uint8) SearchResults {
	var source image.Image
	var cb func(haystack, needle image.Image, interval float64, maxDiff uint8) SearchResults
	var subimg func(r image.Rectangle) image.Image

	switch sub.(type) {
	case *image.NRGBA:
		source = i.c.SubImage(region)
		cb = func(haystack, needle image.Image, interval float64, maxDiff uint8) SearchResults {
			return search(haystack.(*image.NRGBA), needle.(*image.NRGBA), interval, maxDiff)
		}
		subimg = func(r image.Rectangle) image.Image { return i.c.SubImage(r) }
	case *image.Gray:
		source = i.Gray().SubImage(region)
		cb = func(haystack, needle image.Image, interval float64, maxDiff uint8) SearchResults {
			return searchGray(haystack.(*image.Gray), needle.(*image.Gray), interval, maxDiff)
		}
		subimg = func(r image.Rectangle) image.Image { return i.g.SubImage(r) }

	default:
		panic("unsupported image type")
	}

	var final = make(SearchResults, 0, 1)
	res := cb(source, sub, 0.5, maxDiff)
	for _, r := range res {
		res = cb(subimg(r.Rectangle), sub, 0.2, maxDiff)
		for _, r := range res {
			res = cb(subimg(r.Rectangle), sub, 0, maxDiff)
			final = append(final, res...)
		}
	}

	sort.Sort(final)
	return final
}
