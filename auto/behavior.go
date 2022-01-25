package auto

import (
	"fmt"
	"image"
	"strings"
)

type Result struct {
	Match bool
	image.Rectangle
}

type Behaviors struct {
	list  []Behavior
	state *State
}

func NewBehaviors(behaviors ...Behavior) *Behaviors {
	return &Behaviors{list: behaviors, state: &State{}}
}

func (b *Behaviors) Do(search *ImageSearch) error {
	cache := make(map[string]Result)
	state := b.state
	b.state = &State{}
	if err := state.DoNext(search, cache); err != nil {
		return err
	}

	for _, bh := range b.list {
		if err := bh.Do(b.state, search, cache); err != nil {
			return err
		}
		if b.state.Stopped() {
			break
		}
	}

	return nil
}

type Test interface {
	ID() string
	Test(*ImageSearch) Result
}

type SubImgTest struct {
	Uniq      string
	Img       image.Image
	Tolerance uint8
	Region    image.Rectangle
}

func (s SubImgTest) ID() string {
	return fmt.Sprintf("%s - %d - %s", s.Uniq, s.Tolerance, s.Region.String())
}

func (s SubImgTest) Test(search *ImageSearch) Result {
	r := Result{}
	res := search.Search(s.Region, s.Img, s.Tolerance)
	if len(res) == 0 {
		return r
	}
	r.Match = true
	r.Rectangle = res[0].Rectangle
	return r
}

func NewSubImgTest(id string, img image.Image, tolerance uint8, region image.Rectangle) SubImgTest {
	return SubImgTest{Uniq: id, Img: img, Tolerance: tolerance, Region: region}
}

type TestCallback func(*ImageSearch) Result

type CallbackTest struct {
	Uniq string
	cb   TestCallback
}

func (c CallbackTest) ID() string                      { return c.Uniq }
func (c CallbackTest) Test(search *ImageSearch) Result { return c.cb(search) }

func NewCallbackTest(id string, cb TestCallback) CallbackTest {
	return CallbackTest{Uniq: id, cb: cb}
}

type TestGroup struct {
	Tests []Test
}

func (g TestGroup) ID() string {
	n := make([]string, len(g.Tests))
	for i, t := range g.Tests {
		n[i] = t.ID()
	}
	return strings.Join(n, "\n")
}

type OrTest struct {
	TestGroup
}

func (or OrTest) Test(search *ImageSearch) Result {
	for _, t := range or.Tests {
		res := t.Test(search)
		if res.Match {
			return res
		}
	}
	return Result{}
}

func NewOrTest(tests ...Test) OrTest {
	return OrTest{TestGroup{Tests: tests}}
}

type AndTest struct {
	TestGroup
}

func (and AndTest) Test(search *ImageSearch) Result {
	var res Result
	for _, t := range and.Tests {
		res = t.Test(search)
		if !res.Match {
			return res
		}
	}
	return res
}

func NewAndTest(tests ...Test) AndTest {
	return AndTest{TestGroup{Tests: tests}}
}

type State struct {
	stop      bool
	behaviors []Behavior
}

func (s *State) Stop()         { s.stop = true }
func (s *State) Stopped() bool { return s.stop }
func (s *State) Next(b Behavior) {
	if s.behaviors == nil {
		s.behaviors = make([]Behavior, 0, 1)
	}
	s.behaviors = append(s.behaviors, b)
}

func (s *State) DoNext(search *ImageSearch, cache map[string]Result) error {
	if len(s.behaviors) == 0 {
		return nil
	}

	state := &State{}
	for _, b := range s.behaviors {
		if err := b.Do(state, search, cache); err != nil {
			return err
		}
		if state.Stopped() {
			break
		}
	}

	return state.DoNext(search, cache)
}

type Behavior struct {
	Tests []Test
	Run   func(*State, *ImageSearch, []Result) error
}

func (b *Behavior) Do(state *State, search *ImageSearch, cache map[string]Result) error {
	r, ok := b.Test(search, cache)
	if !ok {
		return nil
	}
	return b.Run(state, search, r)
}

func (b *Behavior) Test(search *ImageSearch, cache map[string]Result) ([]Result, bool) {
	r := make([]Result, len(b.Tests))
	for ix, t := range b.Tests {
		id := t.ID()
		res, ok := cache[id]
		if !ok {
			res = t.Test(search)
			cache[id] = res
		}

		if !res.Match {
			return r, false
		}
		r[ix] = res
	}

	return r, true
}
