package types

import (
	"encoding/json"
	"fmt"
	"image"

	"gopkg.in/yaml.v3"
)

type Postcard struct {
	Name  string
	Meta  Metadata
	Front image.Image
	Back  image.Image
}

type Location struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

func (l Location) String() string {
	if l.Latitude == nil || l.Longitude == nil {
		return l.Name
	}

	return fmt.Sprintf("%s (%.5f,%.5f)", l.Name, *l.Latitude, *l.Longitude)
}

type Polygon []Point

type Side struct {
	Description   string    `json:"description,omitempty"`
	Transcription string    `json:"transcription,omitempty"`
	Secrets       []Polygon `json:"secrets,omitempty"`
}

type Context struct {
	Author      Person `json:"author"`
	Description string `json:"description"`
}

type Metadata struct {
	Locale          string   `json:"locale"`
	Location        Location `json:"location,omitempty"`
	Flip            Flip     `json:"flip" yaml:"flip"`
	SentOn          Date     `json:"sentOn,omitempty" yaml:"sent_on"`
	Sender          Person   `json:"sender,omitempty"`
	Recipient       Person   `json:"recipient,omitempty"`
	Front           Side     `json:"front,omitempty"`
	Back            Side     `json:"back,omitempty"`
	FrontDimensions Size     `json:"frontSize" yaml:"front_size,omitempty"`
	Context         Context  `json:"context,omitempty"`
}

var _ json.Marshaler = (*Polygon)(nil)
var _ yaml.Marshaler = (*Polygon)(nil)
var _ json.Unmarshaler = (*Polygon)(nil)
var _ yaml.Unmarshaler = (*Polygon)(nil)
