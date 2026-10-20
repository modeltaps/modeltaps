package azure_v1

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func TestImageEndpointsSupported(t *testing.T) {
	provider := AzureV1ProviderFactory{}.Create(&model.Channel{}).(*AzureV1Provider)
	cases := map[int]string{
		config.RelayModeImagesGenerations: "/v1/images/generations",
		config.RelayModeImagesEdits:       "/v1/images/edits",
		config.RelayModeImagesVariations:  "/v1/images/variations",
	}
	for mode, want := range cases {
		got, err := provider.GetSupportedAPIUri(mode)
		if err != nil {
			t.Errorf("GetSupportedAPIUri(%d) returned error %v", mode, err.Message)
			continue
		}
		if got != want {
			t.Errorf("GetSupportedAPIUri(%d) = %q, want %q", mode, got, want)
		}
	}
}
