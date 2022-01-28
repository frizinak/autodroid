package auto

import (
	"fmt"
	"image"
	"time"
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

func GetOrTest(t Test, search *ImageSearch, results Results, s *Stats) bool {
	twr, ok := t.(TestWithResult)
	if !ok {
		return t.Test(s, search, results)
	}

	id := twr.ID()
	r, ok := results.Get(id)
	if ok {
		s.Add(id, true, 0)
		return r.Match
	}
	start := time.Now()
	res := t.Test(s, search, results)
	s.Add(id, false, time.Since(start))
	return res
}

type Behaviors struct {
	list  []Behavior
	state *State
	stats *Stats
}

type Stats struct {
	list     []ID
	tests    map[ID]int
	cached   map[ID]int
	duration map[ID]time.Duration
}

func (s *Stats) Add(id ID, cached bool, dur time.Duration) {
	if s == nil {
		return
	}
	if _, ok := s.tests[id]; !ok {
		s.list = append(s.list, id)
		s.tests[id] = 0
	}
	m := s.cached
	if !cached {
		m = s.tests
		s.duration[id] += dur
	}
	m[id]++
}

func (s *Stats) Info(id ID) (tests, cached int, duration time.Duration) {
	if s == nil {
		return
	}
	tests, cached, duration = s.tests[id], s.cached[id], s.duration[id]
	return
}

func (s *Stats) List() []ID {
	return s.list
}

func NewBehaviors(stats bool, behaviors ...Behavior) *Behaviors {
	var s *Stats
	if stats {
		s = &Stats{
			make([]ID, 0),
			make(map[ID]int),
			make(map[ID]int),
			make(map[ID]time.Duration),
		}
	}
	return &Behaviors{list: behaviors, state: &State{}, stats: s}
}

func (b *Behaviors) Stats() *Stats { return b.stats }

func (b *Behaviors) Do(search *ImageSearch) error {
	results := NewResults()
	state := b.state
	var err error
	b.state, err = state.DoNext(b.stats, search, results)
	if err != nil {
		return err
	}

	if b.state.Stopped() {
		return nil
	}

	for _, bh := range b.list {
		if err := bh.Do(b.stats, b.state, search, results); err != nil {
			return err
		}
		if err := b.state.DoImmediate(b.stats, search, results); err != nil {
			return err
		}
		if b.state.Stopped() {
			break
		}
	}

	return nil
}

type Test interface {
	Test(*Stats, *ImageSearch, Results) bool
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

func (s SubImgTest) Test(stats *Stats, search *ImageSearch, res Results) bool {
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

func (c CallbackTest) ID() ID { return c.Uniq }
func (c CallbackTest) Test(stats *Stats, search *ImageSearch, r Results) bool {
	return c.cb(c, search, r)
}

func NewCallbackTest(id ID, cb TestCallback) CallbackTest {
	return CallbackTest{Uniq: id, cb: cb}
}

type SimpleTestCallback func(*ImageSearch) Result

type SimpleCallbackTest struct {
	Uniq ID
	cb   SimpleTestCallback
}

func (c SimpleCallbackTest) ID() ID { return c.Uniq }
func (c SimpleCallbackTest) Test(stat *Stats, search *ImageSearch, r Results) bool {
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

func (or OrTest) Test(stats *Stats, search *ImageSearch, results Results) bool {
	for _, t := range or.Tests {
		if GetOrTest(t, search, results, stats) {
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

func (not NotTest) Test(stats *Stats, search *ImageSearch, results Results) bool {
	return !GetOrTest(not.T, search, results, stats)
}

func NewNotTest(test Test) NotTest {
	return NotTest{T: test}
}

type AndTest struct {
	TestGroup
}

func (and AndTest) Test(stats *Stats, search *ImageSearch, results Results) bool {
	for _, t := range and.Tests {
		if !GetOrTest(t, search, results, stats) {
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

func (a AlwaysTest) Test(stats *Stats, search *ImageSearch, results Results) bool {
	GetOrTest(a.T, search, results, stats)
	return true
}

func NewAlwaysTest(test Test) AlwaysTest {
	return AlwaysTest{T: test}
}

type State struct {
	stop      bool
	next      []Behavior
	immediate []Behavior
}

func (s *State) Stop()         { s.stop = true }
func (s *State) Stopped() bool { return s.stop }

func (s *State) Immediate(b Behavior) {
	if s.immediate == nil {
		s.immediate = make([]Behavior, 0, 1)
	}
	s.immediate = append(s.immediate, b)
}

func (s *State) Next(b Behavior) {
	if s.next == nil {
		s.next = make([]Behavior, 0, 1)
	}
	s.next = append(s.next, b)
}

func (s *State) DoImmediate(stats *Stats, search *ImageSearch, results Results) error {
	ns := &State{}
	immediate := s.immediate
	s.immediate = s.immediate[:0]
	for _, b := range immediate {
		if err := b.Do(stats, ns, search, results); err != nil {
			return err
		}

		if err := ns.DoImmediate(stats, search, results); err != nil {
			return err
		}

		for _, b := range ns.next {
			s.Next(b)
		}

		if ns.Stopped() {
			break
		}
	}

	return nil
}

func (s *State) DoNext(stats *Stats, search *ImageSearch, results Results) (*State, error) {
	ns := &State{}
	if len(s.next) == 0 {
		return ns, nil
	}

	for _, b := range s.next {
		if err := b.Do(stats, ns, search, results); err != nil {
			return ns, err
		}

		if err := ns.DoImmediate(stats, search, results); err != nil {
			return ns, err
		}

		if ns.Stopped() {
			break
		}
	}

	return ns, nil
}

type Runner func(*State, *ImageSearch, Results) error

type Behavior struct {
	AndTest
	Run      Runner
	Fallback Runner
}

func (b Behavior) Do(stats *Stats, state *State, search *ImageSearch, results Results) error {
	if !b.Test(stats, search, results) {
		if b.Fallback != nil {
			return b.Fallback(state, search, results)
		}
		return nil
	}
	return b.Run(state, search, results)
}

func NewBehavior(tests []Test, run Runner) Behavior {
	return NewBehaviorWithFallback(tests, run, nil)
}

func NewBehaviorWithFallback(tests []Test, run Runner, fallback Runner) Behavior {
	return Behavior{
		AndTest:  AndTest{TestGroup{Tests: tests}},
		Run:      run,
		Fallback: fallback,
	}
}
