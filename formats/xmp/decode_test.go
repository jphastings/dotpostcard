package xmp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_scanDegrees(t *testing.T) {
	cases := []struct {
		name      string
		str       string
		wantNil   bool
		wantFloat float64
	}{
		{"North", "45,16.83364010N", false, 45.28056066829611},
		{"East", "7,39.81691860E", false, 7.663615309995059},
		{"South", "2,3.45S", false, -2.0575},
		{"West", "6,7.89W", false, -6.1315},

		{"Invalid North over", "91,0.0N", true, 0},
		{"Invalid North over", "90,0.001N", true, 0},
		{"Invalid North under", "-1,0.0N", true, 0},
		{"Invalid North under", "0,-0.01N", true, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fl := scanDegrees(c.str)
			if c.wantNil {
				assert.Nil(t, fl)
				return
			}

			assert.NotNil(t, fl)
			assert.InDelta(t, c.wantFloat, *fl, 0.0000000001)
		})
	}
}
