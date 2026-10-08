package image_test

import (
	"testing"

	img "github.com/modeltaps/modeltaps/common/image"

	"github.com/stretchr/testify/assert"
)

func TestParseBase64File(t *testing.T) {
	mimeType, data, err := img.ParseBase64File("data:audio/mpeg;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=")
	assert.NoError(t, err)
	assert.Equal(t, "audio/mpeg", mimeType)
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=", data)
}
