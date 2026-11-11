// Package fastfoto turns a stack of postcards fed through an Epson FastFoto FF-680W
// into front, back and metadata files ready for the postcards tool.
package fastfoto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/types"
)

const (
	DefaultCardThreshold = 24
	workers              = 2
	maxInFlight          = 6
)

type Options struct {
	OutDir         string
	FirstSideFront bool
	CardThreshold  float64
	// FeedScale stretches the feed-direction measurement to correct the scanner's feed error.
	FeedScale float64
	NoScan    bool
	// Open connects to the scanner; the default is the real USB scanner.
	Open func() (Transport, error)
	// Stop is cancelled to finish the current card and then stop feeding.
	Stop context.Context
	Log  io.Writer
	// LLM, when set, orients and describes each card; Describe overrides the backend it names.
	LLM      LLMConfig
	Describe Describer
}

type CardResult struct {
	Number    int
	Name      string
	WidthPx   int
	HeightPx  int
	WidthCm   float64
	HeightCm  float64
	TiltFront float64 // degrees
	TiltBack  float64
	Flags     []string
	// Notes say what the LLM step did to the card, or why it left the card as scanned.
	Notes []string
	// LLMFailed is set when the LLM step errored, so the card was left as scanned.
	LLMFailed bool
	Elapsed   time.Duration
	Err       error
}

type Report struct {
	Cards []CardResult
	// Orphans are raw pages with no partner, kept in RawDir.
	Orphans  []int
	RawDir   string
	RawBytes int64
}

var (
	rawPageRE = regexp.MustCompile(`^page-(\d+)\.jpg$`)
	outputRE  = regexp.MustCompile(`^(\d+)-(?:front\.png|back\.png|meta\.yaml)$`)
)

func rawName(page int) string  { return fmt.Sprintf("page-%05d.jpg", page) }
func cardName(card int) string { return fmt.Sprintf("%04d", card) }

type run struct {
	opts Options
	// ctx is the run's own: Stop only means finish the current card, which includes its LLM call.
	ctx    context.Context
	rawDir string
	log    func(format string, args ...any)

	mu      sync.Mutex
	results []CardResult
	wg      sync.WaitGroup
	slots   chan struct{}
	// inflight bounds how many decoded cards are held in memory, including those waiting on the LLM.
	inflight chan struct{}
	// llmSlots is separate so slow model calls never hold up image processing.
	llmSlots chan struct{}
}

// Run scans a stack (unless NoScan) and processes every complete pair of pages that
// hasn't been turned into a card yet. Cards are processed while the stack is still
// feeding; the scanner never waits on image work.
func Run(ctx context.Context, opts Options) (Report, error) {
	if opts.CardThreshold == 0 {
		opts.CardThreshold = DefaultCardThreshold
	}
	if opts.FeedScale == 0 {
		opts.FeedScale = 1
	}
	if !(opts.FeedScale > 0) || math.IsInf(opts.FeedScale, 0) {
		return Report{}, fmt.Errorf("the feed scale must be a positive number, not %v", opts.FeedScale)
	}
	if opts.Open == nil {
		opts.Open = OpenUSB
	}
	if opts.Stop == nil {
		opts.Stop = context.Background()
	}
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	if opts.Describe == nil && opts.LLM.Backend != "" {
		d, err := NewDescriber(ctx, opts.LLM)
		if err != nil {
			return Report{}, err
		}
		opts.Describe = d
	}
	r := &run{
		opts:     opts,
		ctx:      ctx,
		rawDir:   filepath.Join(opts.OutDir, "raw"),
		log:      func(format string, args ...any) { fmt.Fprintf(opts.Log, format+"\n", args...) },
		slots:    make(chan struct{}, workers),
		inflight: make(chan struct{}, maxInFlight),
		llmSlots: make(chan struct{}, llmConcurrency),
	}
	if err := os.MkdirAll(r.rawDir, 0o755); err != nil {
		return Report{}, err
	}

	pages, finished, highest, err := r.inventory()
	if err != nil {
		return Report{}, err
	}
	for p := range pages {
		highest = max(highest, (p+1)/2)
	}
	for p := range pages {
		if p%2 == 0 && pages[p-1] && !finished[p/2] {
			r.process(p / 2)
		}
	}

	var scanErr error
	if !opts.NoScan {
		scanErr = r.scan(ctx, 2*highest, pages)
	}
	r.wg.Wait()

	report := r.report()
	errs := []error{scanErr}
	for _, c := range report.Cards {
		if c.Err != nil {
			errs = append(errs, fmt.Errorf("card %s: %w", c.Name, c.Err))
		}
	}
	return report, errors.Join(errs...)
}

// inventory lists the raw pages, the cards whose metadata exists (so are complete), and
// the highest card number that any output file already uses.
func (r *run) inventory() (pages, finished map[int]bool, highest int, err error) {
	pages, finished = map[int]bool{}, map[int]bool{}
	raws, err := os.ReadDir(r.rawDir)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, e := range raws {
		if m := rawPageRE.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			pages[n] = true
		}
	}
	outs, err := os.ReadDir(r.opts.OutDir)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, e := range outs {
		if m := outputRE.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			highest = max(highest, n)
			finished[n] = finished[n] || strings.HasSuffix(e.Name(), "-meta.yaml")
		}
	}
	return pages, finished, highest, nil
}

// scan feeds the stack, saving each side as it arrives and queueing a card whenever
// its second side lands. Pages continue numbering after basePage.
func (r *run) scan(ctx context.Context, basePage int, pages map[int]bool) error {
	t, err := r.opts.Open()
	if err != nil {
		return err
	}
	defer t.Close()

	next := basePage
	s := &session{t: t, log: func(msg string) { r.log("%s", msg) }}
	err = s.feed(ctx, r.opts.Stop, func(jpeg []byte) error {
		next++
		if err := writeAtomic(filepath.Join(r.rawDir, rawName(next)), func(w io.Writer) error {
			_, err := w.Write(jpeg)
			return err
		}); err != nil {
			return err
		}
		pages[next] = true
		if next%2 == 0 {
			r.process(next / 2)
		}
		return nil
	})
	if err != nil && next%2 == 1 {
		r.log("page %d has no partner and stays in %s", next, r.rawDir)
	}
	return err
}

// process queues a card on the worker pool, which never blocks the caller.
func (r *run) process(card int) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.inflight <- struct{}{}
		defer func() { <-r.inflight }()
		start := time.Now()
		res := r.buildOne(card)
		res.Elapsed = time.Since(start)
		r.mu.Lock()
		r.results = append(r.results, res)
		r.mu.Unlock()
		if res.Err == nil {
			r.log("card %s: %.1f x %.1f cm%s, %.1fs", res.Name, res.WidthCm, res.HeightCm, flagSummary(res.Flags), res.Elapsed.Seconds())
		}
	}()
}

func flagSummary(flags []string) string {
	if len(flags) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d flags)", len(flags))
}

func (r *run) buildOne(number int) (res CardResult) {
	res = CardResult{Number: number, Name: cardName(number)}
	defer func() {
		if p := recover(); p != nil {
			res.Err = fmt.Errorf("%v", p)
		}
	}()

	var c *card
	r.withSlot(r.slots, func() { c, res.Err = r.loadCard(number) })
	if res.Err != nil {
		return
	}
	res.WidthPx, res.HeightPx = c.size.PxWidth, c.size.PxHeight
	res.WidthCm, _ = c.size.CmWidth.Float64()
	res.HeightCm, _ = c.size.CmHeight.Float64()
	res.TiltFront, res.TiltBack = c.tilt[0], c.tilt[1]
	res.Flags = c.flags

	flip, sideMeta := types.FlipBook, [2]types.Side{}
	front, back := image.Image(c.front), image.Image(c.back)
	if r.opts.Describe != nil {
		var d Description
		var err error
		r.withSlot(r.llmSlots, func() { d, err = r.describe(front, back) })
		res.Notes = append(res.Notes, d.Notes...)
		if err != nil {
			res.LLMFailed = true
			res.Notes = append(res.Notes, fmt.Sprintf("LLM step failed, card left as scanned: %v", err))
		} else {
			sideMeta[0] = types.Side{Description: d.Front.Description, Transcription: types.AnnotatedText{Text: d.Front.Transcription}}
			sideMeta[1] = types.Side{Description: d.Back.Description, Transcription: types.AnnotatedText{Text: d.Back.Transcription}}
			f, b, wantFlip := resolveTurns(d.Front.Rotation, d.Back.Rotation)
			rf, rb := turn(front, f), turn(back, b)
			size := c.size
			if f%2 == 1 {
				size.PxWidth, size.PxHeight = size.PxHeight, size.PxWidth
				size.CmWidth, size.CmHeight = size.CmHeight, size.CmWidth
			}
			backSize := types.Size{PxWidth: rb.Bounds().Dx(), PxHeight: rb.Bounds().Dy()}
			if err := types.CheckFlip(types.Size{PxWidth: rf.Bounds().Dx(), PxHeight: rf.Bounds().Dy()}, backSize, wantFlip); err != nil {
				res.Notes = append(res.Notes, fmt.Sprintf("LLM orientation ignored: %v", err))
			} else if f != 0 || b != 0 {
				front, back, c.size, flip = rf, rb, size, wantFlip
				res.WidthPx, res.HeightPx = size.PxWidth, size.PxHeight
				res.WidthCm, _ = size.CmWidth.Float64()
				res.HeightCm, _ = size.CmHeight.Float64()
				res.Notes = append(res.Notes, fmt.Sprintf("front turned %d°, back turned %d°, %s", f*90, b*90, flip))
			}
		}
	}

	r.withSlot(r.slots, func() { res.Err = r.write(res.Name, c.size, flip, sideMeta, front, back) })
	return
}

// describeTimeout stops a hung model call holding an LLM slot for ever.
var describeTimeout = 5 * time.Minute

// describe keeps a panicking describer or model adapter from failing the card.
func (r *run) describe(front, back image.Image) (d Description, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	ctx, cancel := context.WithTimeout(r.ctx, describeTimeout)
	defer cancel()
	return r.opts.Describe(ctx, front, back)
}

// withSlot runs fn while holding one of the limited slots.
func (r *run) withSlot(slots chan struct{}, fn func()) {
	slots <- struct{}{}
	defer func() { <-slots }()
	fn()
}

// turn rotates img clockwise by the given number of quarter turns.
func turn(img image.Image, quarters int) image.Image {
	return images.Orient(img, [4]int{1, 6, 3, 8}[quarters])
}

func (r *run) loadCard(number int) (*card, error) {
	var frames [2]*frame
	for i, page := range [2]int{2*number - 1, 2 * number} {
		f, err := os.Open(filepath.Join(r.rawDir, rawName(page)))
		if err != nil {
			return nil, err
		}
		frames[i], err = decodeFrame(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rawName(page), err)
		}
	}
	front, back := frames[1], frames[0]
	if r.opts.FirstSideFront {
		front, back = back, front
	}
	return buildCard(front, back, r.opts.CardThreshold, r.opts.FeedScale)
}

func (r *run) write(name string, size types.Size, flip types.Flip, sides [2]types.Side, front, back image.Image) error {
	meta, err := metadata.Codec(metadata.AsYAML).Encode(types.Postcard{
		Name: name,
		Meta: types.Metadata{
			Flip:     flip,
			Front:    sides[0],
			Back:     sides[1],
			Physical: types.Physical{FrontDimensions: size},
		},
	}, nil)
	if err != nil {
		return err
	}

	// The metadata file goes last: its presence means the card is complete.
	for _, w := range []struct {
		suffix string
		write  func(io.Writer) error
	}{
		{"front.png", func(w io.Writer) error { return png.Encode(w, front) }},
		{"back.png", func(w io.Writer) error { return png.Encode(w, back) }},
		{"meta.yaml", func(w io.Writer) error {
			b, err := meta[0].Bytes()
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}},
	} {
		if err := writeAtomic(filepath.Join(r.opts.OutDir, name+"-"+w.suffix), w.write); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(path string, write func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	// CreateTemp makes 0600 files. The sync means a power loss can't leave a zero-length
	// file that looks complete.
	if err := errors.Join(tmp.Chmod(0o644), tmp.Sync()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (r *run) report() Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	rep := Report{Cards: append([]CardResult(nil), r.results...), RawDir: r.rawDir}
	sort.Slice(rep.Cards, func(i, j int) bool { return rep.Cards[i].Number < rep.Cards[j].Number })

	entries, _ := os.ReadDir(r.rawDir)
	pages := map[int]bool{}
	for _, e := range entries {
		if m := rawPageRE.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			pages[n] = true
			if info, err := e.Info(); err == nil {
				rep.RawBytes += info.Size()
			}
		}
	}
	for p := range pages {
		partner := p + 1
		if p%2 == 0 {
			partner = p - 1
		}
		if !pages[partner] {
			rep.Orphans = append(rep.Orphans, p)
		}
	}
	sort.Ints(rep.Orphans)
	return rep
}
