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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	postcards "github.com/jphastings/dotpostcard"
	"github.com/jphastings/dotpostcard/formats"
	"github.com/jphastings/dotpostcard/formats/atproto"
	"github.com/jphastings/dotpostcard/formats/atproto/pdsupload"
	"github.com/jphastings/dotpostcard/formats/metadata"
	"github.com/jphastings/dotpostcard/internal/cmdhelp"
	"github.com/jphastings/dotpostcard/internal/images"
	"github.com/jphastings/dotpostcard/pkg/describe"
	"github.com/jphastings/dotpostcard/types"
)

const (
	DefaultCardThreshold = 24
	lowDiskBytes         = 1_000_000_000
	workers              = 2
	maxInFlight          = 6
)

type Options struct {
	OutDir string
	// TopSideFront is whether the front of each card faces outward in the feeder.
	TopSideFront bool
	// Quality sets how much resolution and fidelity converted formats keep.
	Quality       formats.Quality
	CardThreshold float64
	// Open connects to the scanner; the default is the real USB scanner.
	Open func() (Transport, error)
	// Stop is cancelled to finish the current card and then stop feeding.
	Stop context.Context
	Log  io.Writer
	// LLM, when set, orients and describes each card; Describe overrides the backend it names.
	LLM      describe.Config
	Describe describe.Describer
	// Formats are converted for every card built, as `postcards -f` would: any name
	// postcards.CodecsByFormat accepts, or "atproto" to upload. Unknown names fail the run
	// before anything is scanned.
	Formats []string
	// ATUser and ATPassword fall back to the same environment variables as the postcards tool.
	ATUser, ATPassword string
	// OnStart is called with how many formats each card is written as, once the scanner is
	// open and before anything is fed.
	OnStart func(formats int)
	// OnCard is called as each card finishes, in the order they finish and never concurrently.
	OnCard func(CardResult)
	// OnScanned is called as soon as the scanner has delivered both sides of a card, before any
	// processing, and never concurrently with OnCard.
	OnScanned func()
	// OnScannerStopped is called once Stop has halted the scanner and released it, with how
	// many scanned cards are still being processed.
	OnScannerStopped func(unsaved int)
}

// Output is something written, or uploaded, for a card.
type Output struct {
	Format string
	// Path is the file written in OutDir, several joined with commas for components, or the
	// URI of an upload.
	Path  string
	Bytes int64 // zero for uploads
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
	// Rescan is set when a side lost enough of a corner that the card was set aside, unbuilt, as
	// for-rescan-NNNN.jpg to be fed again straighter.
	Rescan bool
	// Notes are the describer's own, and why the LLM step left the card as scanned or its orientation was ignored.
	Notes []string
	// LLMFailed is set when the LLM step errored, so the card was left as scanned.
	LLMFailed bool
	// Flip is how the card's back turns over, as the LLM oriented it (book when it didn't).
	Flip types.Flip
	// Described is set when the LLM step described the card.
	Described bool
	// Outputs are the files and uploads made for the card; for a Rescan card, the preview to feed it again by.
	Outputs []Output
	Elapsed time.Duration
	Err     error
	// ConvertErr is set when converting or uploading the built card failed; the card's
	// components are still complete.
	ConvertErr error
}

type Report struct {
	Cards []CardResult
	// Unpaired is set when the scan ended with one side of a card and not its partner. That
	// side is dropped, and the card needs feeding again.
	Unpaired bool
	// StoppedMidCard is set when the user stopped while a card was part-scanned; it was dropped.
	StoppedMidCard bool
	// Interrupted is set when the run's context was cancelled: cards not yet written were
	// dropped, and Discarded counts the scanned ones among them.
	Interrupted bool
	Discarded   int
}

var (
	usedNumberRE = regexp.MustCompile(`^(?:for-rescan-)?(\d+)(?:-(?:front|back|meta)\.|\.|$)`)
	allDigitsRE  = regexp.MustCompile(`^\d+$`)
)

func cardName(card int) string   { return fmt.Sprintf("%04d", card) }
func rescanName(card int) string { return "for-rescan-" + cardName(card) + ".jpg" }

type run struct {
	opts Options
	// ctx is the run's own: Stop only means finish the current card, which includes its LLM call.
	ctx context.Context
	log func(format string, args ...any)

	mu      sync.Mutex
	results []CardResult
	// pending is how many scanned cards haven't yet been written or dropped.
	pending   int
	discarded int
	wg        sync.WaitGroup
	slots     chan struct{}
	// inflight bounds how many decoded cards are held in memory, including those waiting on the LLM.
	inflight chan struct{}
	// llmSlots is separate so slow model calls never hold up image processing.
	llmSlots chan struct{}

	namesMu sync.Mutex
	// taken holds the titled names already given to cards in this run.
	taken map[string]bool

	codecs     []formats.Codec
	encOpts    formats.EncodeOptions
	atClient   *atproto.Client
	wantUpload bool
	// wantComponents is whether component files belong in OutDir; otherwise they are staged in a temporary directory in OutDir.
	wantComponents bool
}

// Run scans a stack and turns each pair of sides into a card, numbered on from the highest
// number already used in OutDir. Cards are processed while the stack is still feeding; the
// scanner never waits on image work.
func Run(ctx context.Context, opts Options) (Report, error) {
	if opts.CardThreshold == 0 {
		opts.CardThreshold = DefaultCardThreshold
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
	if opts.OnStart == nil {
		opts.OnStart = func(int) {}
	}
	if opts.OnCard == nil {
		opts.OnCard = func(CardResult) {}
	}
	if opts.OnScanned == nil {
		opts.OnScanned = func() {}
	}
	if opts.OnScannerStopped == nil {
		opts.OnScannerStopped = func(int) {}
	}
	if opts.Describe == nil && opts.LLM.Backend != "" {
		d, err := describe.New(ctx, opts.LLM)
		if err != nil {
			return Report{}, err
		}
		opts.Describe = d
	}
	r := &run{
		opts:     opts,
		ctx:      ctx,
		log:      func(format string, args ...any) { fmt.Fprintf(opts.Log, format+"\n", args...) },
		slots:    make(chan struct{}, workers),
		inflight: make(chan struct{}, maxInFlight),
		llmSlots: make(chan struct{}, llmConcurrency),
		taken:    map[string]bool{},
	}
	if err := r.prepareFormats(); err != nil {
		return Report{}, err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return Report{}, err
	}
	highest, err := r.highestUsed()
	if err != nil {
		return Report{}, err
	}
	if free, ok := freeBytes(opts.OutDir); ok && free < lowDiskBytes {
		r.log("⚠︎ Only %.1f GB free on this disk; each postcard needs about 40 MB while it's being processed", float64(free)/1e9)
	}

	unpaired, cutShort, scanErr := r.scan(ctx, highest)
	r.wg.Wait()

	report := r.report()
	report.Unpaired = unpaired
	report.StoppedMidCard = cutShort
	report.Interrupted = ctx.Err() != nil
	report.Discarded = r.discarded
	if report.Interrupted && errors.Is(scanErr, ctx.Err()) {
		scanErr = nil
	}
	problems := 0
	for _, c := range report.Cards {
		if c.Err != nil || c.ConvertErr != nil {
			problems++
		}
	}
	errs := []error{scanErr}
	switch {
	case problems == 1:
		errs = append(errs, errors.New("1 postcard had a problem; see above"))
	case problems > 1:
		errs = append(errs, fmt.Errorf("%d postcards had problems; see above", problems))
	}
	return report, errors.Join(errs...)
}

// highestUsed is the highest card number that any file in OutDir starts with.
func (r *run) highestUsed() (highest int, err error) {
	entries, err := os.ReadDir(r.opts.OutDir)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if m := usedNumberRE.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			highest = max(highest, n)
		}
	}
	return highest, nil
}

// scan feeds the stack, queueing a card whenever its second side lands. New cards number on
// from highest. unpaired is whether a side was left without its partner.
func (r *run) scan(ctx context.Context, highest int) (unpaired, cutShort bool, err error) {
	t, err := r.opts.Open()
	if err != nil {
		return false, false, err
	}
	defer t.Close()
	r.opts.OnStart(r.formatCount())

	var first []byte
	s := &session{t: t, log: func(msg string) { r.log("%s", msg) }}
	cutShort, err = s.feed(ctx, r.opts.Stop, func(jpeg []byte) error {
		if first == nil {
			first = jpeg
			return nil
		}
		highest++
		r.process(highest, [2][]byte{first, jpeg})
		first = nil
		return nil
	})
	if first != nil && !cutShort {
		r.log("a side has no partner and is dropped: feed its card again")
	}
	if r.opts.Stop.Err() != nil && ctx.Err() == nil {
		r.mu.Lock()
		unsaved := r.pending
		r.mu.Unlock()
		r.opts.OnScannerStopped(unsaved)
	}
	return first != nil && !cutShort, cutShort, err
}

// process queues a card on the worker pool, which never blocks the caller.
// ponytail: cards waiting for a worker hold their JPEG bytes (about 24 MB per card), with no
// hard bound on the queue. Bound it (block feed's callback) if scans outrun processing for long.
func (r *run) process(card int, sides [2][]byte) {
	r.wg.Add(1)
	r.mu.Lock()
	r.pending++
	r.opts.OnScanned()
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		r.inflight <- struct{}{}
		defer func() { <-r.inflight }()
		start := time.Now()
		res, dropped := r.buildOne(card, sides)
		res.Elapsed = time.Since(start)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.pending--
		if dropped {
			r.discarded++
			return
		}
		r.results = append(r.results, res)
		r.opts.OnCard(res)
	}()
}

// formatCount is how many formats each card is written as, counted as the postcards tool does.
func (r *run) formatCount() int {
	n := len(r.codecs)
	if r.wantUpload {
		n++
	}
	if r.wantComponents {
		n++
	}
	return n
}

// buildOne makes one card. dropped is set when the run was aborted before the card was written.
func (r *run) buildOne(number int, sides [2][]byte) (res CardResult, dropped bool) {
	res = CardResult{Number: number, Name: cardName(number)}
	defer func() {
		if p := recover(); p != nil {
			res.Err = fmt.Errorf("%v", p)
		}
	}()

	if r.ctx.Err() != nil {
		return res, true
	}
	var c *card
	r.withSlot(r.slots, func() { c, res.Err = r.loadCard(sides) })
	if res.Err != nil {
		return
	}
	res.TiltFront, res.TiltBack = c.tilt[0], c.tilt[1]
	res.Flags = c.flags
	if c.rescan {
		res.Rescan = true
		r.withSlot(r.slots, func() { res.Err = r.writeRescanPreview(number, c.rawFront) })
		res.Outputs = []Output{{Format: "Rescan preview", Path: rescanName(number)}}
		return
	}
	res.WidthPx, res.HeightPx = c.size.PxWidth, c.size.PxHeight
	res.WidthCm, _ = c.size.CmWidth.Float64()
	res.HeightCm, _ = c.size.CmHeight.Float64()

	flip, sideMeta := types.FlipBook, [2]types.Side{}
	front, back := image.Image(c.front), image.Image(c.back)
	if r.opts.Describe != nil {
		var d describe.Description
		var err error
		r.withSlot(r.llmSlots, func() { d, err = r.describe(front, back) })
		if r.ctx.Err() != nil {
			return res, true
		}
		res.Notes = append(res.Notes, d.Notes...)
		if err != nil {
			res.LLMFailed = true
			res.Notes = append(res.Notes, fmt.Sprintf("LLM step failed, left as scanned: %v", err))
		} else {
			res.Described = true
			if slug := describe.Slug(d.Title); slug != "" && !allDigitsRE.MatchString(slug) {
				res.Name = r.claimName(slug)
			}
			var sides types.Metadata
			d.Fill(&sides)
			sideMeta = [2]types.Side{sides.Front, sides.Back}
			f, b, wantFlip := d.Turns()
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
			}
		}
	}

	dir := r.opts.OutDir
	if !r.wantComponents {
		dir, res.Err = os.MkdirTemp(r.opts.OutDir, ".fastfoto-*")
		if res.Err != nil {
			return
		}
		defer os.RemoveAll(dir)
	}
	if r.ctx.Err() != nil {
		return res, true
	}
	res.Flip = flip
	r.withSlot(r.slots, func() { res.Err = r.write(dir, res.Name, c.size, flip, sideMeta, front, back) })
	if res.Err == nil && r.wantComponents {
		res.Outputs = append(res.Outputs, componentOutput(dir, res.Name))
	}
	if res.Err == nil && (len(r.codecs) > 0 || r.wantUpload) {
		r.withSlot(r.slots, func() { res.ConvertErr = r.convert(&res, dir) })
		if !r.wantComponents && res.ConvertErr != nil {
			if err := r.keepStaged(dir); err != nil {
				res.ConvertErr = fmt.Errorf("%w (and its components couldn't be kept: %v)", res.ConvertErr, err)
			} else {
				res.ConvertErr = fmt.Errorf("%w (components kept in %s)", res.ConvertErr, r.opts.OutDir)
			}
		}
	}
	return
}

// claimName reserves slug, or slug-2, slug-3, ... for the first one that no other card in this
// run has and none of whose files already exist in OutDir.
func (r *run) claimName(slug string) string {
	r.namesMu.Lock()
	defer r.namesMu.Unlock()
	for n := 1; ; n++ {
		name := slug
		if n > 1 {
			name = fmt.Sprintf("%s-%d", slug, n)
		}
		if !r.taken[name] && !r.filesExist(name) {
			r.taken[name] = true
			return name
		}
	}
}

func (r *run) filesExist(name string) bool {
	var files []string
	if r.wantComponents {
		files = []string{name + "-front.png", name + "-back.png", name + "-meta.yaml"}
	}
	for _, codec := range r.codecs {
		files = append(files, codec.OutputNames(name, &r.encOpts)...)
	}
	for _, f := range files {
		if _, err := os.Lstat(filepath.Join(r.opts.OutDir, f)); err == nil {
			return true
		}
	}
	return false
}

// keepStaged moves a card's staged components into OutDir so a failed conversion doesn't lose it.
func (r *run) keepStaged(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(dir, e.Name()), filepath.Join(r.opts.OutDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// prepareFormats validates the requested formats and logs in to atproto if asked, so that
// mistakes surface before a stack is fed.
func (r *run) prepareFormats() error {
	names := make([]string, 0, len(r.opts.Formats))
	r.wantComponents = len(r.opts.Formats) == 0
	for _, name := range r.opts.Formats {
		switch name {
		case "atproto":
			r.wantUpload = true
		case "component":
			// Written natively; converting would rewrite the files being decoded.
			r.wantComponents = true
		default:
			names = append(names, name)
		}
	}
	codecs, supportFiles, err := postcards.CodecsByFormat(names)
	if err != nil {
		return err
	}
	r.codecs = codecs
	r.encOpts = formats.EncodeOptions{IncludeSupportFiles: supportFiles, Quality: r.opts.Quality}
	if !r.wantUpload {
		return nil
	}
	user, password, err := cmdhelp.ATCredentials(r.opts.ATUser, r.opts.ATPassword)
	if err != nil {
		return err
	}
	r.atClient, err = atproto.Login(user, password, cmdhelp.ATPDSHostOverride(), cmdhelp.ATPLCHostOverride())
	if err != nil {
		return fmt.Errorf("logging in to atproto: %w", err)
	}
	return nil
}

// convert does what `postcards -f ...` would do to the card just written, decoding from its
// component files so the output is identical to a later run of that tool.
func (r *run) convert(res *CardResult, dir string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("converting: %v", p)
		}
	}()
	bundles, err := postcards.MakeBundles([]string{filepath.Join(dir, res.Name+"-front.png")})
	if err != nil {
		return err
	}
	if len(bundles) != 1 {
		return fmt.Errorf("expected one postcard in %s's components, found %d", res.Name, len(bundles))
	}
	pc, err := bundles[0].Decode(formats.DecodeOptions{})
	if err != nil {
		return fmt.Errorf("decoding: %w", err)
	}
	for _, codec := range r.codecs {
		fws, err := codec.Encode(pc, &r.encOpts)
		if err != nil {
			return fmt.Errorf("%s: %w", codec.Name(), err)
		}
		for _, fw := range fws {
			// O_EXCL on shared files lets concurrent cards race safely, and leaves a hand-edited one alone.
			_, err := fw.WriteFile(r.opts.OutDir, !fw.Shared)
			if fw.Shared && errors.Is(err, fs.ErrExist) {
				continue
			}
			if err != nil {
				return fmt.Errorf("%s: %w", fw.Filename, err)
			}
			out := Output{Format: codec.Name(), Path: fw.Filename}
			if info, err := os.Stat(filepath.Join(r.opts.OutDir, fw.Filename)); err == nil {
				out.Bytes = info.Size()
			}
			res.Outputs = append(res.Outputs, out)
		}
	}
	if r.wantUpload {
		uri, _, err := pdsupload.UploadPostcard(r.atClient, pc, &r.encOpts, false, true)
		if err != nil {
			return fmt.Errorf("atproto: %w", err)
		}
		res.Outputs = append(res.Outputs, Output{Format: "ATProto", Path: uri})
	}
	return nil
}

const llmConcurrency = 3

// describeTimeout stops a hung model call holding an LLM slot for ever.
var describeTimeout = describe.Timeout

// describe keeps a panicking describer or model adapter from failing the card.
func (r *run) describe(front, back image.Image) (d describe.Description, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	ctx, cancel := context.WithTimeout(r.ctx, describeTimeout)
	defer cancel()
	d, err = r.opts.Describe(ctx, front, back)
	if errors.Is(err, context.DeadlineExceeded) && r.ctx.Err() == nil {
		err = fmt.Errorf("the model didn't answer within %d minutes", int(describeTimeout.Minutes()))
	}
	return d, err
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

func (r *run) loadCard(sides [2][]byte) (*card, error) {
	var frames [2]*frame
	for i, jpeg := range sides {
		var err error
		if frames[i], err = decodeFrame(bytes.NewReader(jpeg)); err != nil {
			return nil, fmt.Errorf("side %d: %w", i+1, err)
		}
	}
	front, back := frames[1], frames[0]
	if r.opts.TopSideFront {
		front, back = back, front
	}
	return buildCard(front, back, r.opts.CardThreshold)
}

func (r *run) writeRescanPreview(number int, front image.Image) error {
	b, err := describe.ShrunkJPEG(front)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(r.opts.OutDir, rescanName(number)), func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}

func (r *run) write(dir, name string, size types.Size, flip types.Flip, sides [2]types.Side, front, back image.Image) error {
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
		if err := writeAtomic(filepath.Join(dir, name+"-"+w.suffix), w.write); err != nil {
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
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// A full disk has left 0-byte files under their final names with no error reported.
	if info, err := os.Stat(path); err != nil || info.Size() != int64(buf.Len()) {
		return fmt.Errorf("the disk didn't keep %s (is it full?)", filepath.Base(path))
	}
	return nil
}

func (r *run) report() Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	rep := Report{Cards: append([]CardResult(nil), r.results...)}
	sort.Slice(rep.Cards, func(i, j int) bool { return rep.Cards[i].Number < rep.Cards[j].Number })
	return rep
}

func componentOutput(dir, name string) Output {
	out := Output{Format: "Component files"}
	var paths []string
	for _, suffix := range []string{"front.png", "back.png", "meta.yaml"} {
		file := name + "-" + suffix
		paths = append(paths, file)
		if info, err := os.Stat(filepath.Join(dir, file)); err == nil {
			out.Bytes += info.Size()
		}
	}
	out.Path = strings.Join(paths, ", ")
	return out
}
