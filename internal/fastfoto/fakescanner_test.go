package fastfoto

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type fakeReply struct {
	tokens []string
	data   []byte
}

// fakeScanner speaks just enough ESC/I-2 to feed a scripted stack of JPEG sides.
type fakeScanner struct {
	replies []fakeReply
	next    int
	out     bytes.Buffer
	pending string
	Sent    []string
	closed  bool
	// statData is the data block of STAT replies, where the scanner reports its state.
	statData string
}

// newFakeScanner feeds the sides in order, A then B, optionally failing with a jam
// after the given number of sides (jamAfter < 0 for never).
func newFakeScanner(sides [][]byte, jamAfter int, typAfterPst bool) *fakeScanner {
	f := &fakeScanner{}
	for i, jpeg := range sides {
		if jamAfter >= 0 && i == jamAfter {
			break
		}
		letter := "typIMG" + string("AB"[i%2])
		start := "psti0001700d000i0009283"
		tokens := []string{letter, start}
		if typAfterPst {
			tokens = []string{start, letter}
		}
		half := len(jpeg) / 2
		end := []string{fmt.Sprintf("peni0001700i%07d", 920)}
		if i == len(sides)-1 {
			end = append(end, "lftd000")
		}
		f.replies = append(f.replies,
			fakeReply{}, // the scanner is sometimes still thinking
			fakeReply{tokens: tokens, data: jpeg[:half]},
			fakeReply{data: jpeg[half:]},
			fakeReply{tokens: end},
		)
	}
	if jamAfter >= 0 && jamAfter <= len(sides) {
		f.replies = append(f.replies, fakeReply{tokens: []string{"errADF PJ"}})
	}
	return f
}

func (f *fakeScanner) Write(p []byte) error {
	s := string(p)
	if s == "\x1CX" {
		f.Sent = append(f.Sent, "LOCK")
		f.out.WriteByte(0x06)
		return nil
	}
	if f.pending != "" {
		f.handle(f.pending)
		f.pending = ""
		return nil
	}
	name := strings.TrimSpace(s[:4])
	if n, _ := strconv.ParseUint(s[5:12], 16, 32); n > 0 {
		f.pending = name
		return nil
	}
	f.handle(name)
	return nil
}

func (f *fakeScanner) handle(name string) {
	f.Sent = append(f.Sent, name)
	switch name {
	case "PARA":
		f.reply(name, []string{"parOK"}, nil)
	case "STAT":
		f.reply(name, nil, []byte(f.statData))
	case "IMG":
		var r fakeReply
		if f.next < len(f.replies) {
			r = f.replies[f.next]
			f.next++
		}
		f.reply(name, r.tokens, r.data)
	default:
		f.reply(name, nil, nil)
	}
}

func (f *fakeScanner) reply(name string, tokens []string, data []byte) {
	header := fmt.Sprintf("%-4sx%07x", name, len(data))
	for _, t := range tokens {
		header += "#" + t
	}
	f.out.WriteString(header)
	f.out.Write(make([]byte, headerLength-len(header)))
	f.out.Write(data)
}

func (f *fakeScanner) Read(p []byte) (int, error) {
	if f.out.Len() == 0 {
		return 0, io.EOF
	}
	return f.out.Read(p)
}

func (f *fakeScanner) Drain() { f.out.Reset() }

func (f *fakeScanner) Close() error { f.closed = true; return nil }

func (f *fakeScanner) sentFIN() bool {
	return len(f.Sent) > 0 && f.Sent[len(f.Sent)-1] == "FIN"
}
