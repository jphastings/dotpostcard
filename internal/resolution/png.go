package resolution

import (
	"encoding/binary"
	"fmt"
	"math/big"

	pngstructure "github.com/dsoprea/go-png-image-structure"
)

const (
	pngHeader = "\x89\x50\x4E\x47\x0D\x0A\x1A\x0A"
)

func decodePNG(data []byte) (*big.Rat, *big.Rat, error) {
	pmp := pngstructure.NewPngMediaParser()

	intfc, err := pmp.ParseBytes(data)
	if err != nil {
		return nil, nil, err
	}

	cs := intfc.(*pngstructure.ChunkSlice)
	index := cs.Index()
	phys, ok := index["pHYs"]
	if !ok {
		// No physical dimension information
		return nil, nil, nil
	}
	b := phys[0].Data
	if len(b) < 9 {
		return nil, nil, fmt.Errorf("incomplete PNG pHYs header")
	}

	unit := b[8]
	if unit != 1 {
		return nil, nil, fmt.Errorf("invalid PNG resolution unit (%d)", unit)
	}

	pdX := binary.BigEndian.Uint32(b[0:4])
	pdY := binary.BigEndian.Uint32(b[4:8])

	// Scale down by 100, because PNG gives units in meters
	return big.NewRat(int64(pdX), 100), big.NewRat(int64(pdY), 100), nil
}
