package types

import "testing"

// A tool's type decides its price, so it must be kept for every tool of a response, not only
// for the one that was counted first.
func TestIncExtraBillingKeepsTheTypeOfEveryTool(t *testing.T) {
	usage := &Usage{}
	usage.IncExtraBilling(APITollTypeWebSearchPreview, "medium")
	usage.IncExtraBilling(APITollTypeImageGeneration, "high-1024x1024")
	usage.IncExtraBilling(APITollTypeImageGeneration, "high-1024x1024")

	if got := usage.ExtraBilling[APITollTypeWebSearchPreview]; got.Type != "medium" || got.CallCount != 1 {
		t.Errorf("web search = %+v, want type medium and 1 call", got)
	}
	if got := usage.ExtraBilling[APITollTypeImageGeneration]; got.Type != "high-1024x1024" || got.CallCount != 2 {
		t.Errorf("image generation = %+v, want type high-1024x1024 and 2 calls", got)
	}
}
