package xmp

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIptcCoordMarshalXML(t *testing.T) {
	tests := []struct {
		name string
		x    iptcCoord
		want string
	}{
		{"rounds long float-tail precision to 6 decimal places", 0.45348837210000004, "<Iptc4xmpExt:rbX>0.453488</Iptc4xmpExt:rbX>"},
		{"tiny values never render in exponent notation", 0.000001, "<Iptc4xmpExt:rbX>0.000001</Iptc4xmpExt:rbX>"},
		{"values below the rounding floor collapse to zero", 0.0000004, "<Iptc4xmpExt:rbX>0</Iptc4xmpExt:rbX>"},
		{"zero round-trips as a plain integer", 0, "<Iptc4xmpExt:rbX>0</Iptc4xmpExt:rbX>"},
		{"one round-trips as a plain integer", 1, "<Iptc4xmpExt:rbX>1</Iptc4xmpExt:rbX>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(iptcRegionVertex{ParseType: "Resource", X: tt.x})
			assert.NoError(t, err)
			assert.Contains(t, string(out), tt.want)
			assert.NotContains(t, string(out), "e-")
			assert.NotContains(t, string(out), "e+")
		})
	}
}
