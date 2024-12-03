package types

import (
	"fmt"
	"image"
)

type Postcard struct {
	Name  string
	Meta  Metadata
	Front image.Image
	Back  image.Image
}

func (pc Postcard) Sides() int {
	if pc.Back == nil {
		return 1
	}
	return 2
}

type Location struct {
	Name      string   `json:"name,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

func (l Location) String() string {
	if l.Latitude == nil || l.Longitude == nil {
		return l.Name
	}

	return fmt.Sprintf("%s (%.5f,%.5f)", l.Name, *l.Latitude, *l.Longitude)
}

type Polygon struct {
	Prehidden bool    `json:"prehidden"`
	Points    []Point `json:"points"`
}

type Side struct {
	Description   string        `json:"description,omitempty" yaml:"description,omitempty"`
	Transcription AnnotatedText `json:"transcription,omitempty" yaml:"transcription,omitempty"`
	Secrets       []Polygon     `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

type Context struct {
	Author      Person `json:"author"`
	Description string `json:"description"`
}

type Metadata struct {
	Name      string   `json:"-" yaml:"-"`
	Locale    string   `json:"locale,omitempty"`
	Location  Location `json:"location,omitempty" yaml:"location,omitempty"`
	Flip      Flip     `json:"flip" yaml:"flip"`
	SentOn    Date     `json:"sentOn,omitempty" yaml:"sent_on,omitempty"`
	Sender    Person   `json:"sender,omitempty" yaml:"sender,omitempty"`
	Recipient Person   `json:"recipient,omitempty" yaml:"recipient,omitempty"`
	Front     Side     `json:"front,omitempty" yaml:"front,omitempty"`
	Back      Side     `json:"back,omitempty" yaml:"back,omitempty"`
	Context   Context  `json:"context,omitempty" yaml:"context,omitempty"`
	Physical  Physical `json:"physical,omitempty" yaml:"physical,omitempty"`
}

type Physical struct {
	FrontDimensions Size    `json:"frontSize" yaml:"front_size,omitempty"`
	ThicknessMM     float64 `json:"thicknessMM,omitempty" yaml:"thickness_mm,omitempty"`
	CardColour      string  `json:"cardColour,omitempty" yaml:"card_colour,omitempty"`
}

func (pc Postcard) String() string {
	return fmt.Sprintf("<Postcard: %s>", pc.Name)
}
