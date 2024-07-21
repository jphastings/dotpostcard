package sides

import (
	"fmt"
	"image"

	"github.com/jphastings/postcards/types"
)

func (b bundle) Decode() (types.Postcard, error) {
	pc, err := b.metaBundle.Decode()
	if err != nil {
		return types.Postcard{}, err
	}

	pc.Name = b.name

	front, _, err := image.Decode(b.frontFile)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't decode postcard's front image: %w", err)
	}
	pc.Front = front

	back, _, err := image.Decode(b.backFile)
	if err != nil {
		return types.Postcard{}, fmt.Errorf("couldn't decode postcard's back image: %w", err)
	}
	pc.Back = back

	// TODO: Check physical sizes & set FrontSize of metadata

	return pc, nil
}
