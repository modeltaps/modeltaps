package relay

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestCheckImageEditN(t *testing.T) {
	cases := []struct {
		in      int
		want    int
		wantErr bool
	}{
		{0, 1, false},
		{1, 1, false},
		{MaxImageN, MaxImageN, false},
		{MaxImageN + 1, 0, true},
		{-1, 0, true},
	}
	for _, c := range cases {
		req := types.ImageEditRequest{N: c.in}
		err := checkImageEditN(&req)
		if c.wantErr {
			if err == nil {
				t.Errorf("n=%d: want error, got nil", c.in)
			}
			continue
		}
		if err != nil || req.N != c.want {
			t.Errorf("n=%d: got n=%d err=%v, want n=%d", c.in, req.N, err, c.want)
		}
	}
}
