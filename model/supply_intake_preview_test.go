package model

import "testing"

// The preview counts what each rule decides over the catalog, first match winning, and judges a
// typed name, in the catalog or not.
func TestPreviewIntakeRules(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "gpt-x", Status: ModelStatusPublished, Endpoints: `["chat"]`},
		{Model: "gpt-x-preview", Status: ModelStatusPublished, Endpoints: `["chat"]`},
		{Model: "embed-1", Status: ModelStatusPublished, Endpoints: `["embeddings"]`},
		{Model: "gpt-x-alias", AliasOf: "gpt-x"},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&ModelSupply{Model: "gpt-x", ChannelId: 1, UpstreamModel: "gpt-x", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewIntakeRules(DefaultIntakeRules(), "gpt-x", true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Total != 3 {
		t.Fatalf("total = %d, want the three public models", preview.Total)
	}
	counts := map[int]int{}
	for _, r := range preview.Rules {
		counts[r.Index] = r.Count
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 1 || counts[-1] != 0 || len(preview.Rules) != 4 {
		t.Fatalf("counts = %v, rules %+v", counts, preview.Rules)
	}
	if len(preview.Models) != 3 || preview.Models[0].Model != "embed-1" || preview.Models[0].Rule != 2 {
		t.Fatalf("listed models = %+v", preview.Models)
	}
	if m := preview.Model; m == nil || !m.Known || !m.Supplied || m.Rule != 1 || m.Action != IntakeActionEnable || !m.JoinLast {
		t.Fatalf("gpt-x = %+v, want the chat rule, joining last since it is supplied", preview.Model)
	}
	preview, _ = PreviewIntakeRules(DefaultIntakeRules(), "brand-new-exp", false)
	if m := preview.Model; m == nil || m.Known || m.Rule != 0 || m.Action != IntakeActionPending {
		t.Fatalf("a new name = %+v, want the preview rule", preview.Model)
	}
	preview, _ = PreviewIntakeRules(nil, "", false)
	if len(preview.Rules) != 1 || preview.Rules[0].Index != -1 || preview.Rules[0].Count != 3 {
		t.Fatalf("no rules: %+v, want everything on the fallback", preview.Rules)
	}
}
