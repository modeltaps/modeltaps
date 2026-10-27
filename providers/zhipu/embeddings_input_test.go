package zhipu

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestConvertFromEmbeddingOpenaiRejectsTokenArray(t *testing.T) {
	_, errWithCode := convertFromEmbeddingOpenai(&types.EmbeddingRequest{Model: "embedding-2", Input: []any{float64(1), float64(2)}})
	if errWithCode == nil || errWithCode.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %v, want a 400 error", errWithCode)
	}
}
