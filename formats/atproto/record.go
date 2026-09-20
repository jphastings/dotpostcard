// Package atproto lets postcards be stored as org.dotpostcard.postcard records on an
// atproto PDS, and read back from one. See lexicons/org/dotpostcard/postcard.json for the
// record shape this package mirrors.
package atproto

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/jphastings/dotpostcard/types"
)

// RecordType is the value of a record's "$type" field.
const RecordType = "org.dotpostcard.postcard"

const (
	flipTokenBook      = "org.dotpostcard.postcard#flipBook"
	flipTokenLeftHand  = "org.dotpostcard.postcard#flipLeftHand"
	flipTokenCalendar  = "org.dotpostcard.postcard#flipCalendar"
	flipTokenRightHand = "org.dotpostcard.postcard#flipRightHand"
	flipTokenNone      = "org.dotpostcard.postcard#flipNone"
)

// Record mirrors the org.dotpostcard.postcard lexicon record exactly.
type Record struct {
	Type      string    `json:"$type"`
	Image     Blob      `json:"image"`
	Flip      string    `json:"flip"`
	Sides     []Side    `json:"sides"`
	Physical  Physical  `json:"physical"`
	Locale    string    `json:"locale,omitempty"`
	SentOn    *Date     `json:"sentOn,omitempty"`
	Sender    *Person   `json:"sender,omitempty"`
	Recipient *Person   `json:"recipient,omitempty"`
	Location  *Location `json:"location,omitempty"`
	Context   *Context  `json:"context,omitempty"`
}

// Date's fields have no omitempty: all three are required by the lexicon, and 0 would be an
// invalid (not merely absent) month or day.
type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type Physical struct {
	FrontSize   Size      `json:"frontSize"`
	ThicknessUm int       `json:"thicknessUm,omitempty"`
	CardColor   *RGBColor `json:"cardColor,omitempty"`
}

// RGBColor's fields have no omitempty: 0 is a meaningful channel value, and all three are
// required by the lexicon.
type RGBColor struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// Blob is an atproto blob reference, as returned by com.atproto.repo.uploadBlob.
type Blob struct {
	Type     string  `json:"$type"`
	Ref      BlobRef `json:"ref"`
	MimeType string  `json:"mimeType"`
	Size     int64   `json:"size"`
}

type BlobRef struct {
	Link string `json:"$link"`
}

type Side struct {
	Description   string         `json:"description,omitempty"`
	Transcription *AnnotatedText `json:"transcription,omitempty"`
	Secrets       []Secret       `json:"secrets,omitempty"`
}

type AnnotatedText struct {
	Text        string       `json:"text"`
	Annotations []Annotation `json:"annotations,omitempty"`
}

type Annotation struct {
	Type      string `json:"type"`
	Value     string `json:"value,omitempty"`
	ByteStart int    `json:"byteStart"`
	ByteEnd   int    `json:"byteEnd"`
}

// Secret.Prehidden defaults to true (per the lexicon): most secrets are already obscured
// before upload, so only the false case is worth spending a field on.
type Secret struct {
	Prehidden *bool   `json:"prehidden,omitempty"`
	Points    []Point `json:"points"`
}

// Point holds coordinates in ten-thousandths of the side's width/height, per the lexicon.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Size struct {
	WidthPx  int  `json:"widthPx"`
	HeightPx int  `json:"heightPx"`
	WidthMm  *int `json:"widthMm,omitempty"`
	HeightMm *int `json:"heightMm,omitempty"`
}

type Person struct {
	Name string `json:"name,omitempty"`
	Uri  string `json:"uri,omitempty"`
}

type Location struct {
	Name        string `json:"name,omitempty"`
	Latitude    string `json:"latitude,omitempty"`
	Longitude   string `json:"longitude,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
}

type Context struct {
	Description string  `json:"description,omitempty"`
	Author      *Person `json:"author,omitempty"`
}

// FromMetadata builds the record for meta, referencing image as its blob.
func FromMetadata(meta types.Metadata, image Blob) Record {
	rec := Record{
		Type:      RecordType,
		Image:     image,
		Flip:      flipToToken(meta.Flip),
		Locale:    meta.Locale,
		Sender:    personToRecord(meta.Sender),
		Recipient: personToRecord(meta.Recipient),
		Location:  locationToRecord(meta.Location),
		Context:   contextToRecord(meta.Context),
	}

	rec.Sides = append(rec.Sides, sideToRecord(meta.Front))
	if meta.Flip != types.FlipNone {
		rec.Sides = append(rec.Sides, sideToRecord(meta.Back))
	}

	rec.Physical = physicalToRecord(meta.Physical)

	if meta.SentOn != nil && !meta.SentOn.IsZero() {
		y, m, d := meta.SentOn.Date()
		rec.SentOn = &Date{Year: y, Month: int(m), Day: d}
	}

	return rec
}

func physicalToRecord(p types.Physical) Physical {
	phys := Physical{
		FrontSize: Size{
			WidthPx:  p.FrontDimensions.PxWidth,
			HeightPx: p.FrontDimensions.PxHeight,
		},
	}
	if p.FrontDimensions.HasPhysical() {
		w, _ := p.FrontDimensions.CmWidth.Float64()
		h, _ := p.FrontDimensions.CmHeight.Float64()
		wmm := int(math.Round(w * 10))
		hmm := int(math.Round(h * 10))
		phys.FrontSize.WidthMm = &wmm
		phys.FrontSize.HeightMm = &hmm
	}

	if p.ThicknessMM != 0 {
		phys.ThicknessUm = int(math.Round(p.ThicknessMM * 1000))
	}

	if p.CardColor != nil {
		phys.CardColor = &RGBColor{R: int(p.CardColor.R), G: int(p.CardColor.G), B: int(p.CardColor.B)}
	}

	return phys
}

// ToMetadata interprets the record back into postcard metadata.
func (r Record) ToMetadata() (types.Metadata, error) {
	flip, err := tokenToFlip(r.Flip)
	if err != nil {
		return types.Metadata{}, err
	}

	loc, err := locationFromRecord(r.Location)
	if err != nil {
		return types.Metadata{}, err
	}

	meta := types.Metadata{
		Flip:      flip,
		Locale:    r.Locale,
		Sender:    personFromRecord(r.Sender),
		Recipient: personFromRecord(r.Recipient),
		Location:  loc,
		Context:   contextFromRecord(r.Context),
	}

	if len(r.Sides) > 0 {
		meta.Front = sideFromRecord(r.Sides[0])
	}
	if len(r.Sides) > 1 {
		meta.Back = sideFromRecord(r.Sides[1])
	}

	meta.Physical = physicalFromRecord(r.Physical)

	if r.SentOn != nil {
		t := time.Date(r.SentOn.Year, time.Month(r.SentOn.Month), r.SentOn.Day, 0, 0, 0, 0, time.UTC)
		// time.Date silently normalises an out-of-range component (eg. 30 February becomes 2
		// March), so an impossible date must be caught by checking it didn't move.
		if y, m, d := t.Date(); y != r.SentOn.Year || int(m) != r.SentOn.Month || d != r.SentOn.Day {
			return types.Metadata{}, fmt.Errorf("invalid sentOn date %04d-%02d-%02d", r.SentOn.Year, r.SentOn.Month, r.SentOn.Day)
		}
		meta.SentOn = &types.Date{Time: t}
	}

	return meta, nil
}

func physicalFromRecord(p Physical) types.Physical {
	var out types.Physical

	out.FrontDimensions.PxWidth = p.FrontSize.WidthPx
	out.FrontDimensions.PxHeight = p.FrontSize.HeightPx
	if p.FrontSize.WidthMm != nil && p.FrontSize.HeightMm != nil {
		out.FrontDimensions.CmWidth = big.NewRat(int64(*p.FrontSize.WidthMm), 10)
		out.FrontDimensions.CmHeight = big.NewRat(int64(*p.FrontSize.HeightMm), 10)
	}

	if p.ThicknessUm != 0 {
		out.ThicknessMM = float64(p.ThicknessUm) / 1000
	}

	if p.CardColor != nil {
		out.CardColor = &types.Color{R: uint8(p.CardColor.R), G: uint8(p.CardColor.G), B: uint8(p.CardColor.B), A: 0xff}
	}

	return out
}

func flipToToken(flip types.Flip) string {
	switch flip {
	case types.FlipBook:
		return flipTokenBook
	case types.FlipLeftHand:
		return flipTokenLeftHand
	case types.FlipCalendar:
		return flipTokenCalendar
	case types.FlipRightHand:
		return flipTokenRightHand
	default:
		return flipTokenNone
	}
}

func tokenToFlip(token string) (types.Flip, error) {
	switch token {
	case flipTokenBook:
		return types.FlipBook, nil
	case flipTokenLeftHand:
		return types.FlipLeftHand, nil
	case flipTokenCalendar:
		return types.FlipCalendar, nil
	case flipTokenRightHand:
		return types.FlipRightHand, nil
	case flipTokenNone, "":
		return types.FlipNone, nil
	default:
		return "", fmt.Errorf("unknown flip token %q", token)
	}
}

func personToRecord(p types.Person) *Person {
	if p.Name == "" && p.Uri == "" {
		return nil
	}
	return &Person{Name: p.Name, Uri: p.Uri}
}

func personFromRecord(p *Person) types.Person {
	if p == nil {
		return types.Person{}
	}
	return types.Person{Name: p.Name, Uri: p.Uri}
}

func locationToRecord(l types.Location) *Location {
	if l.Name == "" && l.Latitude == nil && l.Longitude == nil && l.CountryCode == "" {
		return nil
	}

	loc := &Location{Name: l.Name, CountryCode: l.CountryCode}
	if l.Latitude != nil {
		loc.Latitude = strconv.FormatFloat(*l.Latitude, 'f', -1, 64)
	}
	if l.Longitude != nil {
		loc.Longitude = strconv.FormatFloat(*l.Longitude, 'f', -1, 64)
	}
	return loc
}

func locationFromRecord(l *Location) (types.Location, error) {
	if l == nil {
		return types.Location{}, nil
	}

	out := types.Location{Name: l.Name, CountryCode: l.CountryCode}
	if l.Latitude != "" {
		v, err := strconv.ParseFloat(l.Latitude, 64)
		if err != nil {
			return types.Location{}, fmt.Errorf("parsing latitude %q: %w", l.Latitude, err)
		}
		out.Latitude = &v
	}
	if l.Longitude != "" {
		v, err := strconv.ParseFloat(l.Longitude, 64)
		if err != nil {
			return types.Location{}, fmt.Errorf("parsing longitude %q: %w", l.Longitude, err)
		}
		out.Longitude = &v
	}
	return out, nil
}

func contextToRecord(c types.Context) *Context {
	if c.Description == "" && c.Author.Name == "" && c.Author.Uri == "" {
		return nil
	}
	return &Context{Description: c.Description, Author: personToRecord(c.Author)}
}

func contextFromRecord(c *Context) types.Context {
	if c == nil {
		return types.Context{}
	}
	return types.Context{Description: c.Description, Author: personFromRecord(c.Author)}
}

func sideToRecord(s types.Side) Side {
	rs := Side{Description: s.Description}

	if s.Transcription.Text != "" || len(s.Transcription.Annotations) > 0 {
		at := &AnnotatedText{Text: s.Transcription.Text}
		for _, a := range s.Transcription.Annotations {
			at.Annotations = append(at.Annotations, Annotation{
				Type:      string(a.Type),
				Value:     a.Value,
				ByteStart: int(a.Start),
				ByteEnd:   int(a.End),
			})
		}
		rs.Transcription = at
	}

	for _, secret := range s.Secrets {
		sec := Secret{Prehidden: prehiddenToRecord(secret.Prehidden)}
		for _, p := range secret.Points {
			sec.Points = append(sec.Points, Point{
				X: int(math.Round(p.X * 10000)),
				Y: int(math.Round(p.Y * 10000)),
			})
		}
		rs.Secrets = append(rs.Secrets, sec)
	}

	return rs
}

// prehiddenToRecord omits the field for the (default, common) true case, and writes an
// explicit false otherwise.
func prehiddenToRecord(prehidden bool) *bool {
	if prehidden {
		return nil
	}
	f := false
	return &f
}

func prehiddenFromRecord(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

func sideFromRecord(s Side) types.Side {
	ts := types.Side{Description: s.Description}

	if s.Transcription != nil {
		ts.Transcription.Text = s.Transcription.Text
		for _, a := range s.Transcription.Annotations {
			ts.Transcription.Annotations = append(ts.Transcription.Annotations, types.Annotation{
				Type:  types.AnnotationType(a.Type),
				Value: a.Value,
				Start: uint(a.ByteStart),
				End:   uint(a.ByteEnd),
			})
		}
	}

	for _, secret := range s.Secrets {
		poly := types.Polygon{Prehidden: prehiddenFromRecord(secret.Prehidden)}
		for _, p := range secret.Points {
			poly.Points = append(poly.Points, types.Point{
				X: float64(p.X) / 10000,
				Y: float64(p.Y) / 10000,
			})
		}
		ts.Secrets = append(ts.Secrets, poly)
	}

	return ts
}

// Diff reports the names of the fields that differ between two records, ignoring the image
// blob. Differences within a side are reported one level deep, eg. "sides[1].transcription".
func Diff(a, b Record) []string {
	var diffs []string

	add := func(name string, equal bool) {
		if !equal {
			diffs = append(diffs, name)
		}
	}

	add("flip", a.Flip == b.Flip)
	add("physical.frontSize", reflect.DeepEqual(a.Physical.FrontSize, b.Physical.FrontSize))
	add("locale", a.Locale == b.Locale)
	add("sentOn", reflect.DeepEqual(a.SentOn, b.SentOn))
	add("sender", reflect.DeepEqual(a.Sender, b.Sender))
	add("recipient", reflect.DeepEqual(a.Recipient, b.Recipient))
	add("location", reflect.DeepEqual(a.Location, b.Location))
	add("context", reflect.DeepEqual(a.Context, b.Context))
	add("physical.thicknessUm", a.Physical.ThicknessUm == b.Physical.ThicknessUm)
	add("physical.cardColor", reflect.DeepEqual(a.Physical.CardColor, b.Physical.CardColor))

	maxSides := len(a.Sides)
	if len(b.Sides) > maxSides {
		maxSides = len(b.Sides)
	}

	for i := 0; i < maxSides; i++ {
		var sa, sb Side
		if i < len(a.Sides) {
			sa = a.Sides[i]
		}
		if i < len(b.Sides) {
			sb = b.Sides[i]
		}

		if sa.Description != sb.Description {
			diffs = append(diffs, fmt.Sprintf("sides[%d].description", i))
		}
		if !transcriptionEqual(sa.Transcription, sb.Transcription) {
			diffs = append(diffs, fmt.Sprintf("sides[%d].transcription", i))
		}
		if !secretsEqual(sa.Secrets, sb.Secrets) {
			diffs = append(diffs, fmt.Sprintf("sides[%d].secrets", i))
		}
	}

	return diffs
}

func transcriptionEqual(a, b *AnnotatedText) bool {
	deref := func(t *AnnotatedText) AnnotatedText {
		if t == nil {
			return AnnotatedText{}
		}
		return *t
	}

	ta, tb := deref(a), deref(b)
	if ta.Text != tb.Text {
		return false
	}
	if len(ta.Annotations) == 0 && len(tb.Annotations) == 0 {
		return true
	}
	return reflect.DeepEqual(ta.Annotations, tb.Annotations)
}

func secretsEqual(a, b []Secret) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		// Prehidden nil and an explicit true both mean the same thing (nil is just the
		// omitted-default spelling of it), so compare the resolved value, not the pointer.
		if prehiddenFromRecord(a[i].Prehidden) != prehiddenFromRecord(b[i].Prehidden) {
			return false
		}
		if !reflect.DeepEqual(a[i].Points, b[i].Points) {
			return false
		}
	}
	return true
}

// ValidRecordKey enforces atproto's record key rules: 1-512 characters from
// [A-Za-z0-9._:~-], and not "." or "..".
var rkeyRE = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,512}$`)

func ValidRecordKey(rkey string) error {
	if rkey == "." || rkey == ".." {
		return fmt.Errorf("record key %q is reserved", rkey)
	}
	if !rkeyRE.MatchString(rkey) {
		return fmt.Errorf("record key %q must be 1-512 characters from [A-Za-z0-9._:~-]", rkey)
	}
	return nil
}
