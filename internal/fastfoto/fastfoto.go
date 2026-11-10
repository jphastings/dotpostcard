// Package fastfoto turns a stack of postcards fed through an Epson FastFoto FF-680W
// into front, back and metadata files ready for the postcards tool.
package fastfoto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	"github.com/jphastings/dotpostcard/types"
)

const (
	DefaultCardThreshold = 24
	workers              = 2
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
	opts   Options
	rawDir string
	log    func(format string, args ...any)

	mu      sync.Mutex
	results []CardResult
	wg      sync.WaitGroup
	slots   chan struct{}
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
	r := &run{
		opts:   opts,
		rawDir: filepath.Join(opts.OutDir, "raw"),
		log:    func(format string, args ...any) { fmt.Fprintf(opts.Log, format+"\n", args...) },
		slots:  make(chan struct{}, workers),
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
		r.slots <- struct{}{}
		defer func() { <-r.slots }()

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

	firstPage, secondPage := 2*number-1, 2*number
	var frames [2]*frame
	for i, page := range [2]int{firstPage, secondPage} {
		f, err := os.Open(filepath.Join(r.rawDir, rawName(page)))
		if err != nil {
			res.Err = err
			return
		}
		frames[i], err = decodeFrame(f)
		f.Close()
		if err != nil {
			res.Err = fmt.Errorf("%s: %w", rawName(page), err)
			return
		}
	}
	front, back := frames[1], frames[0]
	if r.opts.FirstSideFront {
		front, back = back, front
	}

	c, err := buildCard(front, back, r.opts.CardThreshold, r.opts.FeedScale)
	if err != nil {
		res.Err = err
		return
	}
	res.WidthPx, res.HeightPx = c.size.PxWidth, c.size.PxHeight
	res.WidthCm, _ = c.size.CmWidth.Float64()
	res.HeightCm, _ = c.size.CmHeight.Float64()
	res.TiltFront, res.TiltBack = c.tilt[0], c.tilt[1]
	res.Flags = c.flags

	meta, err := metadata.Codec(metadata.AsYAML).Encode(types.Postcard{
		Name: res.Name,
		Meta: types.Metadata{
			Flip:     types.FlipBook,
			Physical: types.Physical{FrontDimensions: c.size},
		},
	}, nil)
	if err != nil {
		res.Err = err
		return
	}

	// The metadata file goes last: its presence means the card is complete.
	dir := r.opts.OutDir
	for _, w := range []struct {
		suffix string
		write  func(io.Writer) error
	}{
		{"front.png", func(w io.Writer) error { return png.Encode(w, c.front) }},
		{"back.png", func(w io.Writer) error { return png.Encode(w, c.back) }},
		{"meta.yaml", func(w io.Writer) error {
			b, err := meta[0].Bytes()
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}},
	} {
		if err = writeAtomic(filepath.Join(dir, res.Name+"-"+w.suffix), w.write); err != nil {
			res.Err = err
			return
		}
	}
	return
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
