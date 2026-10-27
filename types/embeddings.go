package types

import (
	"errors"
	"fmt"
)

type EmbeddingRequest struct {
	Model          string `json:"model" binding:"required"`
	Input          any    `json:"input" binding:"required"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
	User           string `json:"user,omitempty"`
}

type Embedding struct {
	Object    string `json:"object"`
	Embedding any    `json:"embedding"`
	Index     int    `json:"index"`
}

type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  *Usage      `json:"usage,omitempty"`
}

// ParseInput parses an embeddings input that is a string or an array of strings. Token arrays
// ([]int) are rejected: text-only upstreams return 400 for them, other channels should forward
// the raw input.
func (r EmbeddingRequest) ParseInput() ([]string, error) {
	if r.Input == nil {
		return nil, errors.New("input is required")
	}

	switch input := r.Input.(type) {
	case string:
		return []string{input}, nil
	case []any:
		if len(input) == 0 {
			return nil, errors.New("invalid input: empty array")
		}
		texts := make([]string, 0, len(input))
		for i, item := range input {
			str, ok := item.(string)
			if !ok {
				if isNumber(item) {
					return nil, fmt.Errorf("invalid input[%d]: expected string, got %T (token arrays are not supported)", i, item)
				}
				return nil, fmt.Errorf("invalid input[%d]: expected string, got %T", i, item)
			}
			texts = append(texts, str)
		}
		return texts, nil
	default:
		return nil, fmt.Errorf("invalid input: expected string or array of strings, got %T", r.Input)
	}
}

// ParseInputString returns the first parsed text, for upstreams that accept a single string.
func (r EmbeddingRequest) ParseInputString() (string, error) {
	texts, err := r.ParseInput()
	if err != nil {
		return "", err
	}
	return texts[0], nil
}

// isNumber reports whether v is a JSON number (decoded as float64).
func isNumber(v any) bool {
	_, ok := v.(float64)
	return ok
}
