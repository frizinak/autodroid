package auto

import (
	"fmt"
	"image"
)

type Result struct {
	Match bool
	image.Rectangle
}

type ID string

type Results map[ID]Result

func (r Results) Set(t TestWithResult, res Result) { r[t.ID()] = res }
func (r Results) Get(id ID) (Result, bool)         { res, ok := r[id]; return res, ok }
func (r Results) Match(id ID) bool                 { return r[id].Match }

func (r Results) Must(id ID) Result {
	res, ok := r[id]
	if !ok || !res.Match {
		panic(fmt.Sprintf("results for %s does not match", id))
	}
	return res
}

func NewResults() Results { return make(Results) }

func GetOrTest(t Test, search *ImageSearch, results Results) bool {
	twr, ok := t.(TestWithResult)
	if !ok {
		return t.Test(search, results)
	}

	id := twr.ID()
	r, ok := results.Get(id)
	if !ok && t.Test(search, results) {
		return true
	}
	if ok && r.Match {
		return true
	}
	return false
}

type Behaviors struct {
	list  []Behavior
	state *State
}

func NewBehaviors(behaviors ...Behavior) *Behaviors {
	return &Behaviors{list: behaviors, state: &State{}}
}

func (b *Behaviors) Do(search *ImageSearch) error {
	results := NewResults()
	state := b.state
	var err error
	b.state, err = state.DoNext(search, results)
	if err != nil {
		return err
	}

	if b.state.Stopped() {
		return nil
	}

	for _, bh := range b.list {
		if err := bh.Do(b.state, search, results); err != nil {
			return err
		}
		if b.state.Stopped() {
			break
		}
	}

	return nil
}

type Test interface {
	Test(*ImageSearch, Results) bool
}

type TestWithResult interface {
	ID() ID
	Test
}

type SubImgTest struct {
	Uniq      ID
	Img       image.Image
	Tolerance uint8
	Region    image.Rectangle
}

func (s SubImgTest) ID() ID { return s.Uniq }

func (s SubImgTest) Test(search *ImageSearch, res Results) bool {
	sr := search.Search(s.Region, s.Img, s.Tolerance)
	var r Result
	if len(sr) != 0 {
		r.Match = true
		r.Rectangle = sr[0].Rectangle
	}
	res.Set(s, r)
	return r.Match
}

func NewSubImgTest(id ID, img image.Image, tolerance uint8, region image.Rectangle) SubImgTest {
	return SubImgTest{Uniq: id, Img: img, Tolerance: tolerance, Region: region}
}

type TestCallback func(CallbackTest, *ImageSearch, Results) bool

type CallbackTest struct {
	Uniq ID
	cb   TestCallback
}

func (c CallbackTest) ID() ID                                   { return c.Uniq }
func (c CallbackTest) Test(search *ImageSearch, r Results) bool { return c.cb(c, search, r) }

func NewCallbackTest(id ID, cb TestCallback) CallbackTest {
	return CallbackTest{Uniq: id, cb: cb}
}

type SimpleTestCallback func(*ImageSearch) Result

type SimpleCallbackTest struct {
	Uniq ID
	cb   SimpleTestCallback
}

func (c SimpleCallbackTest) ID() ID { return c.Uniq }
func (c SimpleCallbackTest) Test(search *ImageSearch, r Results) bool {
	res := c.cb(search)
	r.Set(c, res)
	return res.Match
}

func NewSimpleTest(id ID, cb SimpleTestCallback) SimpleCallbackTest {
	return SimpleCallbackTest{Uniq: id, cb: cb}
}

type TestGroup struct {
	Tests []Test
}

type OrTest struct {
	TestGroup
}

func (or OrTest) Test(search *ImageSearch, results Results) bool {
	for _, t := range or.Tests {
		if GetOrTest(t, search, results) {
			return true
		}

	}
	return false
}

func NewOrTest(tests ...Test) OrTest {
	return OrTest{TestGroup{Tests: tests}}
}

type NotTest struct {
	T Test
}

func (not NotTest) Test(search *ImageSearch, results Results) bool {
	return !not.T.Test(search, results)
}

func NewNotTest(test Test) NotTest {
	return NotTest{T: test}
}

type AndTest struct {
	TestGroup
}

func (and AndTest) Test(search *ImageSearch, results Results) bool {
	for _, t := range and.Tests {
		if !GetOrTest(t, search, results) {
			return false
		}
	}
	return true
}

func NewAndTest(tests ...Test) AndTest {
	return AndTest{TestGroup{Tests: tests}}
}

type AlwaysTest struct {
	T Test
}

func (a AlwaysTest) Test(search *ImageSearch, results Results) bool {
	GetOrTest(a.T, search, results)
	return true
}

func NewAlwaysTest(test Test) AlwaysTest {
	return AlwaysTest{T: test}
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

func (s *State) DoNext(search *ImageSearch, results Results) (*State, error) {
	if len(s.behaviors) == 0 {
		return &State{}, nil
	}

	state := &State{}
	for _, b := range s.behaviors {
		if err := b.Do(state, search, results); err != nil {
			return state, err
		}
		if state.Stopped() {
			break
		}
	}

	return state, nil
}

type Runner func(*State, *ImageSearch, Results) error

type Behavior struct {
	AndTest
	Run Runner
}

func (b Behavior) Do(state *State, search *ImageSearch, results Results) error {
	if !b.Test(search, results) {
		return nil
	}
	return b.Run(state, search, results)
}

func NewBehavior(Tests []Test, Run Runner) Behavior {
	return Behavior{
		AndTest: AndTest{TestGroup{Tests: Tests}},
		Run:     Run,
	}
}
