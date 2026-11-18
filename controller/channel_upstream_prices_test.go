package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func TestChannelKeepsPriceList(t *testing.T) {
	cases := map[int]bool{
		config.ChannelTypeOpenRouter: true,
		config.ChannelTypeDeepInfra:  true, // a preset platform, priced from its own list
		config.ChannelTypeTogether:   true, // a preset platform, priced from models.dev
		config.ChannelTypeOpenAI:     false,
	}
	for typ, want := range cases {
		if got := channelKeepsPriceList(&model.Channel{Type: typ}); got != want {
			t.Errorf("type %d keeps a price list = %v, want %v", typ, got, want)
		}
	}
}
