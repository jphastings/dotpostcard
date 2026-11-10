package fastfoto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Transport is the byte pipe to the scanner: IOKit USB for real, a fake in tests.
type Transport interface {
	Write(p []byte) error
	// Read fills p with at least one byte, like io.Reader.
	Read(p []byte) (int, error)
	// Drain discards anything the scanner has queued for reading.
	Drain()
	Close() error
}

var (
	ErrFeederEmpty = errors.New("the feeder is empty")
	ErrCoverOpen   = errors.New("the scanner's cover is open")
	ErrJam         = errors.New("the feeder jammed")
	ErrDoubleFeed  = errors.New("the scanner pulled in two cards at once")
	ErrReserved    = errors.New("the scanner is reserved by another computer")
	ErrCancelled   = errors.New("the scan was cancelled at the scanner")
	ErrBusyUSB     = errors.New("another app is using the scanner (quit Epson software and Image Capture)")
	ErrNoScanner   = errors.New("no Epson FastFoto FF-680W found; is it plugged in and switched on?")
	ErrWedged      = errors.New("the scanner stopped responding; switch it off and on again")
	// ErrNotResponding wraps USB failures mid-session, so users see advice rather than an IOReturn code.
	ErrNotResponding = errors.New("the scanner isn't responding: check it's switched on (it powers itself off after 10 idle minutes), and if it is, switch it off and on again")
)

// Never add FMTRAW, CRP or a GMT tone curve: the first two have wedged the scanner
// until power-cycled, and the curve clips blank paper to pure white.
// OVSN is kept because it makes each side end at the card's real length.
const scanParameters = "#ADFDPLXOVSN#COLC024#FMTJPG #JPGd100#GMMUG18#RSMd600#RSSd600#BSZi1048576#ACQi0000000i0000000i0005096i0009283"

const (
	writeGap     = 500 * time.Microsecond
	idleLimit    = 60 * time.Second
	warmupLimit  = 90 * time.Second
	headerLength = 64
	// A corrupted reply must not make us allocate gigabytes; real image blocks are 1 MiB.
	maxReplyData = 16 << 20
)

// pollInterval is how long to wait between STAT polls while the scanner warms up.
var pollInterval = time.Second

type session struct {
	t   Transport
	log func(string)
}

func (s *session) write(p []byte) error {
	time.Sleep(writeGap)
	return s.t.Write(p)
}

func (s *session) readFull(p []byte) error {
	for n, empty := 0, 0; n < len(p); {
		got, err := s.t.Read(p[n:])
		n += got
		if err != nil {
			return err
		}
		if got == 0 {
			if empty++; empty > 3 {
				return ErrWedged
			}
		}
	}
	return nil
}

// command sends one ESC/I-2 command and returns the tokens of its 64-byte reply plus
// whatever data the reply announces.
func (s *session) command(name, payload string) (tokens []string, data []byte, err error) {
	if err = s.write([]byte(fmt.Sprintf("%-4sx%07x", name, len(payload)))); err != nil {
		return
	}
	if payload != "" {
		if err = s.write([]byte(payload)); err != nil {
			return
		}
	}
	header := make([]byte, headerLength)
	if err = s.readFull(header); err != nil {
		return
	}
	more, perr := strconv.ParseUint(string(header[5:12]), 16, 32)
	if perr != nil {
		return nil, nil, fmt.Errorf("unreadable reply %q to %s", header[:12], name)
	}
	if more > maxReplyData {
		return nil, nil, fmt.Errorf("corrupted reply to %s announcing %d bytes", name, more)
	}
	tokens = splitTokens(header[12:])
	if more > 0 {
		data = make([]byte, more)
		if err = s.readFull(data); err != nil {
			return
		}
	}
	return
}

func splitTokens(b []byte) []string {
	var tokens []string
	for _, t := range strings.Split(string(b), "#") {
		t = strings.Trim(t, " \x00")
		if len(t) >= 3 && !strings.HasPrefix(t, "---") {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// tokenError turns a scanner error or attention token into an error, or nil.
func tokenError(tok string) error {
	lower := strings.ToLower(tok)
	switch {
	case lower == "atncan":
		return ErrCancelled
	case !strings.HasPrefix(lower, "err"):
		return nil
	case strings.Contains(lower, "adf pe"):
		return ErrFeederEmpty
	case strings.Contains(lower, "adf opn"):
		return ErrCoverOpen
	case strings.Contains(lower, "adf pj"):
		return ErrJam
	case strings.Contains(lower, "adf df"):
		return ErrDoubleFeed
	}
	return fmt.Errorf("scanner error %q", tok)
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// resync clears up after a session that ended badly. The scanner refuses commands while
// replies from the last session are still queued or its session is still open.
func (s *session) resync() error {
	s.t.Drain()
	if err := s.write([]byte("FIN x0000000")); err != nil {
		return err
	}
	s.t.Drain()
	return nil
}

func (s *session) lock() error {
	if err := s.write([]byte("\x1CX")); err != nil {
		return err
	}
	ack := make([]byte, 1)
	if err := s.readFull(ack); err != nil {
		return fmt.Errorf("the scanner didn't answer; it may be reserved by another computer: %w", err)
	}
	if ack[0] != 0x06 {
		return fmt.Errorf("the scanner refused the session (replied %#x)", ack[0])
	}
	return nil
}

func (s *session) awaitReady(ctx context.Context) error {
	deadline := time.Now().Add(warmupLimit)
	warned := false
	for {
		tokens, data, err := s.command("STAT", "")
		if err != nil {
			return err
		}
		// The scanner reports its state (ERRADF PE, ERRADF OPN) in the data block.
		tokens = append(tokens, splitTokens(data)...)
		busy := false
		for _, tok := range tokens {
			if err := tokenError(tok); err != nil {
				return err
			}
			switch tok {
			case "nrdRSVD":
				return ErrReserved
			case "nrdBUSY":
				busy = true
			}
		}
		if !busy {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrWedged
		}
		if !warned {
			s.log("scanner warming up")
			warned = true
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// setup runs STAT, PARA and TRDT; afterwards IMG polls return the stack.
func (s *session) setup(ctx context.Context) error {
	if err := s.awaitReady(ctx); err != nil {
		return err
	}
	tokens, _, err := s.command("PARA", scanParameters)
	if err != nil {
		return err
	}
	if !containsToken(tokens, "parOK") {
		return fmt.Errorf("the scanner rejected the scan settings: %v", tokens)
	}
	tokens, _, err = s.command("TRDT", "")
	if err != nil {
		return err
	}
	for _, tok := range tokens {
		if err := tokenError(tok); err != nil {
			return err
		}
	}
	return nil
}

func containsToken(tokens []string, want string) bool {
	for _, t := range tokens {
		if t == want {
			return true
		}
	}
	return false
}

type side struct {
	data   []byte
	height int
}

// values parses a token's fixed-width numbers: d+3 digits, i+7 digits.
func values(s string) []int {
	var out []int
	for len(s) > 0 {
		var width int
		switch s[0] {
		case 'd':
			width = 4
		case 'i':
			width = 8
		default:
			return out
		}
		if len(s) < width {
			return out
		}
		n, err := strconv.Atoi(s[1:width])
		if err != nil {
			return out
		}
		out = append(out, n)
		s = s[width:]
	}
	return out
}

// feed polls IMG until the scanner reports the batch finished, calling save with each
// completed side's JPEG. Once stop is done, it stops after the current card (every
// second side). FIN is always sent once the scanner is locked.
func (s *session) feed(ctx, stop context.Context, save func(jpeg []byte) error) (err error) {
	if err = s.resync(); err != nil {
		return err
	}
	if err = s.lock(); err != nil {
		return err
	}
	defer func() {
		if _, _, finErr := s.command("FIN", ""); finErr != nil && err == nil {
			err = finErr
		}
	}()
	if err = s.setup(ctx); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			s.command("CAN", "")
		}
	}()

	open := map[byte]*side{}
	var cur byte
	sides := 0
	lastProgress := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if stop.Err() != nil && len(open) == 0 && sides%2 == 0 {
			return nil
		}

		tokens, data, err := s.command("IMG", "")
		if err != nil {
			return err
		}

		var target *side
		targetSet := false
		finished, batchDone := []byte(nil), false
		var tokErr error
		// The side letter can follow the token that starts the side, so read it first.
		for _, tok := range tokens {
			if strings.HasPrefix(tok, "typIMG") && len(tok) == 7 {
				cur = tok[6]
			}
		}
		for _, tok := range tokens {
			if e := tokenError(tok); e != nil {
				if tokErr == nil {
					tokErr = e
				}
				continue
			}
			switch {
			case strings.HasPrefix(tok, "pst"):
				if open[cur] != nil {
					return fmt.Errorf("the scanner started side %c twice; a side was lost", cur)
				}
				open[cur] = &side{}
			case strings.HasPrefix(tok, "pen"):
				sd := open[cur]
				if sd == nil {
					return fmt.Errorf("the scanner ended side %c that never started; a side was lost", cur)
				}
				if !targetSet {
					target, targetSet = sd, true
				}
				if v := values(tok[3:]); len(v) == 2 {
					sd.height = v[1]
				}
				finished = append(finished, cur)
			case tok == "lftd000":
				batchDone = true
			}
		}
		if !targetSet {
			target = open[cur]
		}
		if len(data) > 0 && target != nil {
			target.data = append(target.data, data...)
		}
		// Sides finished in a reply that also carries an error are still saved.
		for _, letter := range finished {
			if want := "AB"[sides%2]; letter != want {
				return fmt.Errorf("side %c finished where %c was expected; a side was lost", letter, want)
			}
			sd := open[letter]
			delete(open, letter)
			jpeg, perr := patchHeight(sd.data, sd.height)
			if perr != nil {
				return perr
			}
			if err := save(jpeg); err != nil {
				return err
			}
			sides++
		}
		if tokErr != nil {
			return tokErr
		}
		if batchDone && (len(open) > 0 || sides%2 != 0) {
			return errors.New("the scanner finished its batch with a side missing")
		}

		switch {
		case len(tokens) > 0 || len(data) > 0:
			lastProgress = time.Now()
		case time.Since(lastProgress) > idleLimit:
			return ErrWedged
		default:
			if err := sleep(ctx, time.Millisecond); err != nil {
				return err
			}
		}
		if batchDone {
			return nil
		}
	}
}

// patchHeight rewrites the JPEG's SOF height, which the scanner sets to the maximum
// frame (9283) rather than the length actually scanned.
func patchHeight(jpeg []byte, height int) ([]byte, error) {
	if !bytes.HasPrefix(jpeg, []byte{0xFF, 0xD8}) {
		return nil, errors.New("scanner sent data that isn't a JPEG")
	}
	for i := 2; i+9 < len(jpeg); {
		if jpeg[i] != 0xFF {
			return nil, errors.New("malformed JPEG from scanner")
		}
		marker := jpeg[i+1]
		if marker >= 0xC0 && marker <= 0xC2 {
			if height > 0 {
				jpeg[i+5], jpeg[i+6] = byte(height>>8), byte(height)
			}
			return jpeg, nil
		}
		i += 2 + int(jpeg[i+2])<<8 + int(jpeg[i+3])
	}
	return nil, errors.New("JPEG from scanner has no frame header")
}
