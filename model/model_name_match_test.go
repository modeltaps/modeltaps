package model

import (
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestMatchModelName(t *testing.T) {
	cases := []struct {
		upstream, public string
		channelType      int
		tier             string // "" = no match
		def              bool
		extra            []string
	}{
		{"claude-fable-5-2", "claude-fable-5-2", config.ChannelTypeCustom, MatchExact, true, nil},
		{"Claude-Fable-5-2", "claude-fable-5-2", config.ChannelTypeCustom, MatchExact, true, nil},
		// Separators and platform decoration.
		{"anthropic/claude-fable-5.2", "claude-fable-5-2", config.ChannelTypeOpenRouter, MatchPlatform, true, nil},
		{"claude-fable-5-2@default", "claude-fable-5-2", config.ChannelTypeVertexAI, MatchPlatform, true, nil},
		{"us.anthropic.claude-fable-5-2-v1:0", "claude-fable-5-2", config.ChannelTypeBedrock, MatchPlatform, true, nil},
		{"global.anthropic.claude-fable-5-2", "claude-fable-5-2", config.ChannelTypeCustom, MatchPlatform, true, nil},
		{"claude_fable_5_2", "claude-fable-5-2", config.ChannelTypeCustom, MatchPlatform, true, nil},
		// A dated snapshot of an undated public name.
		{"claude-fable-5-2-20261101", "claude-fable-5-2", config.ChannelTypeCustom, MatchPlatform, true, []string{"20261101"}},
		// A different date, or a public name with a date the upstream lacks.
		{"claude-opus-4-5", "claude-opus-4-5-20251101", config.ChannelTypeCustom, "", false, nil},
		// Variants are different models.
		{"claude-fable-5-2-thinking", "claude-fable-5-2", config.ChannelTypeCustom, MatchVariant, false, []string{"thinking"}},
		{"gpt-5-mini", "gpt-5", config.ChannelTypeCustom, MatchVariant, false, []string{"mini"}},
		{"qwen3-235b-a22b", "qwen3", config.ChannelTypeCustom, MatchVariant, false, []string{"235b", "a22b"}},
		{"anthropic/claude-fable-5.2:online", "claude-fable-5-2", config.ChannelTypeOpenRouter, MatchVariant, false, []string{":online"}},
		// Other versions never match.
		{"gpt-5.1", "gpt-5", config.ChannelTypeCustom, "", false, nil},
		{"gpt-4o", "gpt-4", config.ChannelTypeCustom, "", false, nil},
		{"claude-fable-5", "claude-fable-5-2", config.ChannelTypeCustom, "", false, nil},
		{"deepseek-v3", "deepseek-v3-1", config.ChannelTypeCustom, "", false, nil},
		// Same model, other terms: offered, not selected.
		{"anthropic/claude-fable-5.2:free", "claude-fable-5-2", config.ChannelTypeOpenRouter, MatchPlatform, false, []string{":free"}},
		{"anthropic/claude-fable-5.2:nitro", "claude-fable-5-2", config.ChannelTypeOpenRouter, MatchPlatform, false, []string{":nitro"}},
		// ":nitro" is OpenRouter's; elsewhere it is unknown.
		{"claude-fable-5-2:nitro", "claude-fable-5-2", config.ChannelTypeCustom, MatchPending, false, []string{":nitro"}},
		// Unknown additions wait for confirmation.
		{"aws-claude-fable-5-2", "claude-fable-5-2", config.ChannelTypeCustom, MatchPending, false, []string{"aws"}},
		{"cf-claude-fable-5-2-20261101", "claude-fable-5-2", config.ChannelTypeCustom, MatchPending, false, []string{"20261101", "cf"}},
		// Unrelated.
		{"gemini-3-pro", "claude-fable-5-2", config.ChannelTypeCustom, "", false, nil},
	}
	for _, c := range cases {
		got, ok := matchModelName(c.upstream, c.public, c.channelType)
		if c.tier == "" {
			if ok {
				t.Errorf("%s vs %s: got %+v, want no match", c.upstream, c.public, got)
			}
			continue
		}
		if !ok || got.Tier != c.tier || got.Default != c.def || !reflect.DeepEqual(got.Extra, c.extra) {
			t.Errorf("%s vs %s: got %+v (ok=%v), want tier=%s default=%v extra=%v", c.upstream, c.public, got, ok, c.tier, c.def, c.extra)
		}
	}
}

func TestPublicNameIndexCandidates(t *testing.T) {
	idx := newPublicNameIndex([]string{"claude-fable-5-2", "gpt-5", "gpt-5-mini", "gemini-3-pro"})
	got := idx.candidates("anthropic/claude-fable-5.2", config.ChannelTypeOpenRouter)
	if !reflect.DeepEqual(got, []string{"claude-fable-5-2"}) {
		t.Errorf("candidates = %v", got)
	}
	got = idx.candidates("gpt-5-mini", config.ChannelTypeCustom)
	if !reflect.DeepEqual(got, []string{"gpt-5", "gpt-5-mini"}) {
		t.Errorf("candidates = %v", got)
	}
}

func TestLibraryOffersAndPicks(t *testing.T) {
	data := setupModelLibraryTestDB(t)
	if err := DB.AutoMigrate(&ModelSupply{}, &Price{}); err != nil {
		t.Fatal(err)
	}
	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: map[string]*Price{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if _, err := SyncModelLibrary(data); err != nil {
		t.Fatal(err)
	}
	// OpenRouter lists the plain spelling, a free tier and a thinking variant; the relay uses a
	// spelling nobody knows.
	or := &Channel{Name: "or", Type: config.ChannelTypeOpenRouter, Status: config.ChannelStatusEnabled,
		Models: "anthropic/claude-fable-5.2:free,anthropic/claude-fable-5.2,anthropic/claude-fable-5.2-thinking"}
	relay := &Channel{Name: "relay", Type: config.ChannelTypeCustom, Status: config.ChannelStatusEnabled,
		Models: "aws-claude-fable-5-2,gpt-5.6-sol-mini"}
	for _, ch := range []*Channel{or, relay} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}

	offersFor := func(model string) map[int]LibraryChannel {
		items, err := ListLibraryItems()
		if err != nil {
			t.Fatal(err)
		}
		out := map[int]LibraryChannel{}
		for _, it := range items {
			if it.Model == model {
				for _, c := range it.Channels {
					out[c.Id] = c
				}
			}
		}
		return out
	}

	fable := offersFor("claude-fable-5-2")
	if c := fable[or.Id]; c.UpstreamModel != "anthropic/claude-fable-5.2" || c.Tier != MatchPlatform || !c.Default {
		t.Errorf("openrouter offer = %+v, want the plain spelling, selected", c)
	}
	if c := fable[relay.Id]; c.UpstreamModel != "aws-claude-fable-5-2" || c.Tier != MatchPending || c.Default {
		t.Errorf("relay offer = %+v, want pending, not selected", c)
	}
	if c := offersFor("gpt-5.6-sol")[relay.Id]; c.Tier != MatchVariant || c.Default {
		t.Errorf("gpt-5.6-sol offer = %+v, want the mini as a variant", c)
	}

	// The admin keeps OpenRouter out and confirms the relay's spelling.
	res, err := AddLibraryModels(LibraryAddRequest{
		Canonicals: []string{"anthropic/claude-fable-5-2"}, Status: ModelStatusDraft, Supplies: true,
		Channels: map[string][]LibraryAddChannel{
			"anthropic/claude-fable-5-2": {{Id: relay.Id, UpstreamModel: "aws-claude-fable-5-2"}, {Id: 999, UpstreamModel: "made-up"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Supplies != 1 {
		t.Fatalf("result = %+v, want one supply (the unknown channel is ignored)", res)
	}
	var supplies []ModelSupply
	DB.Where("model = ?", "claude-fable-5-2").Find(&supplies)
	if len(supplies) != 1 || supplies[0].ChannelId != relay.Id || supplies[0].UpstreamModel != "aws-claude-fable-5-2" {
		t.Errorf("supplies = %+v", supplies)
	}

	// Both decisions are remembered on their own channel.
	fable = offersFor("claude-fable-5-2")
	if c := fable[relay.Id]; c.Tier != MatchExact || !c.Default {
		t.Errorf("relay after confirm = %+v, want exact", c)
	}
	if c := fable[or.Id]; !c.Excluded || c.selectedByDefault() {
		t.Errorf("openrouter after rejection = %+v, want listed but excluded", c)
	}
	if err := relay.Delete(); err != nil {
		t.Fatal(err)
	}
	var left int64
	DB.Model(&ChannelModelAlias{}).Where("channel_id = ?", relay.Id).Count(&left)
	if left != 0 {
		t.Errorf("aliases of a deleted channel = %d, want 0", left)
	}
}
