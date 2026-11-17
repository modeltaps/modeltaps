package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func floatPtr(v float64) *float64 { return &v }

func TestChannelFeeDefault(t *testing.T) {
	ch := &Channel{Type: config.ChannelTypeRequesty}
	ch.applyFeeDefault()
	if ch.GetFeeRate() != 0.05 || ch.FeeNote == "" {
		t.Errorf("requesty default = %v %q", ch.GetFeeRate(), ch.FeeNote)
	}
	ch = &Channel{Type: config.ChannelTypeOpenRouter, FeeRate: floatPtr(0)}
	ch.applyFeeDefault()
	if ch.GetFeeRate() != 0 {
		t.Error("an explicit fee rate was replaced by the default")
	}
	ch = &Channel{Type: config.ChannelTypeOpenAI}
	ch.applyFeeDefault()
	if ch.GetFeeRate() != 0 || ch.FeeNote != "" {
		t.Error("a type without a fee got one")
	}
}
