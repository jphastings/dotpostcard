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

	// frontData, err := io.ReadAll(b.frontFile)
	// if err != nil {
	// 	return pc, fmt.Errorf("unable to read front image content for dimension analysis: %w", err)
	// }

	// w, h, err := resolution.Decode(frontData)
	// if err != nil {
	// 	return pc, fmt.Errorf("unable to extract physical dimensions from front image: %w", err)
	// }

	// pc.Meta.FrontDimensions = types.Size{
	// 	CmWidth:  w,
	// 	CmHeight: h,
	// }

	// TODO: Compare to back dimensions and assert similarity

	if b.backFile != nil {
		back, _, err := image.Decode(b.backFile)
		if err != nil {
			return types.Postcard{}, fmt.Errorf("couldn't decode postcard's back image: %w", err)
		}
		pc.Back = back
	}

	return pc, nil
}
