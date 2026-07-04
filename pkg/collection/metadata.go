package collection

import "github.com/jphastings/dotpostcard/types"

// storedMetadata mirrors types.Metadata for JSON storage. types.Polygon only
// implements json.Unmarshaler (not json.Marshaler), and its UnmarshalJSON
// requires a "type": "polygon"/"box" discriminator that the default,
// reflection-based marshal of a bare types.Polygon never writes — so
// marshalling a types.Metadata and then unmarshalling it back, as
// AddWebPostcard and Metadata do, fails on any secret region. Swapping in
// types.SecretPolygon (which already round-trips through encoding/json
// symmetrically) for storage avoids that without editing the types package.
type storedMetadata struct {
	Locale    string         `json:"locale,omitempty"`
	Location  types.Location `json:"location,omitempty"`
	Flip      types.Flip     `json:"flip,omitempty"`
	SentOn    *types.Date    `json:"sentOn,omitempty"`
	Sender    types.Person   `json:"sender,omitempty"`
	Recipient types.Person   `json:"recipient,omitempty"`
	Front     storedSide     `json:"front,omitempty"`
	Back      storedSide     `json:"back,omitempty"`
	Context   types.Context  `json:"context,omitempty"`
	Physical  types.Physical `json:"physical,omitempty"`
}

type storedSide struct {
	Description   string                `json:"description,omitempty"`
	Transcription types.AnnotatedText   `json:"transcription,omitempty"`
	Secrets       []types.SecretPolygon `json:"secrets,omitempty"`
}

func toStoredMetadata(meta types.Metadata) storedMetadata {
	return storedMetadata{
		Locale:    meta.Locale,
		Location:  meta.Location,
		Flip:      meta.Flip,
		SentOn:    meta.SentOn,
		Sender:    meta.Sender,
		Recipient: meta.Recipient,
		Front:     toStoredSide(meta.Front),
		Back:      toStoredSide(meta.Back),
		Context:   meta.Context,
		Physical:  meta.Physical,
	}
}

func toStoredSide(side types.Side) storedSide {
	var secrets []types.SecretPolygon
	if len(side.Secrets) > 0 {
		secrets = make([]types.SecretPolygon, len(side.Secrets))
		for i, p := range side.Secrets {
			secrets[i] = types.SecretPolygon{Type: "polygon", Prehidden: p.Prehidden, Points: p.Points}
		}
	}

	return storedSide{
		Description:   side.Description,
		Transcription: side.Transcription,
		Secrets:       secrets,
	}
}

func (sm storedMetadata) toMetadata() types.Metadata {
	// types.Color's hex-string JSON representation has no room for alpha, and
	// its UnmarshalJSON leaves it at the zero value rather than the opaque
	// default that every other decode path (e.g. types.ColorFromString) uses;
	// restore it here so a stored, fully-opaque CardColor doesn't come back
	// as fully transparent.
	physical := sm.Physical
	if physical.CardColor != nil {
		opaque := *physical.CardColor
		opaque.A = 0xff
		physical.CardColor = &opaque
	}

	return types.Metadata{
		Locale:    sm.Locale,
		Location:  sm.Location,
		Flip:      sm.Flip,
		SentOn:    sm.SentOn,
		Sender:    sm.Sender,
		Recipient: sm.Recipient,
		Front:     sm.Front.toSide(),
		Back:      sm.Back.toSide(),
		Context:   sm.Context,
		Physical:  physical,
	}
}

func (ss storedSide) toSide() types.Side {
	var secrets []types.Polygon
	if len(ss.Secrets) > 0 {
		secrets = make([]types.Polygon, len(ss.Secrets))
		for i, p := range ss.Secrets {
			secrets[i] = types.Polygon{Prehidden: p.Prehidden, Points: p.Points}
		}
	}

	return types.Side{
		Description:   ss.Description,
		Transcription: ss.Transcription,
		Secrets:       secrets,
	}
}
