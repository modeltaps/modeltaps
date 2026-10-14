package check_channel

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand/v2"
	"strconv"
	"sync"

	"github.com/modeltaps/modeltaps/types"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The font is parsed once; a Face is not safe for concurrent use, so each render creates its own (cheap).
var loadDigitFont = sync.OnceValues(func() (*opentype.Font, error) {
	return opentype.Parse(gobold.TTF)
})

// renderDigitsPNG draws the digits black on white and returns the PNG as a data URL.
// The image travels inside the request, so the check does not need a publicly reachable server
// address; reading the digits back proves the image content really reached the model.
func renderDigitsPNG(digits string) (string, error) {
	f, err := loadDigitFont()
	if err != nil {
		return "", err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 96, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return "", err
	}
	defer face.Close()

	const width, height = 480, 200
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face}
	textWidth := d.MeasureString(digits).Ceil()
	ascent := face.Metrics().Ascent.Ceil()
	d.Dot = fixed.P((width-textWidth)/2, (height+ascent)/2-8)
	d.DrawString(digits)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

type CheckImgProcess struct {
	ModelName string
	expected  string
	imageURL  string
	renderErr error
}

func CreateCheckImgProcess(modelName string) *CheckImgProcess {
	expected := strconv.Itoa(100000 + rand.IntN(900000))
	imageURL, err := renderDigitsPNG(expected)
	return &CheckImgProcess{ModelName: modelName, expected: expected, imageURL: imageURL, renderErr: err}
}

const imageProbeKey = "image"

func (c *CheckImgProcess) GetKey() string {
	return imageProbeKey
}

func (c *CheckImgProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role: types.ChatMessageRoleUser,
				Content: []types.ChatMessagePart{
					{Type: types.ContentTypeText, Text: "Read the number in the image. Respond with ONLY the digits, nothing else."},
					{Type: types.ContentTypeImageURL, ImageURL: &types.ChatMessageImageURL{URL: c.imageURL}},
				},
			},
		},
	}
}

func (c *CheckImgProcess) Check(_ CheckContext, _ *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	if c.renderErr != nil {
		return []*CheckResult{{Name: resultImage, Status: CheckStatusFailed, Code: "image_render_failed", Params: map[string]any{"message": c.renderErr.Error()}}}
	}
	if openaiErr != nil {
		return failedResponse(openaiErr)
	}

	answer, ok := firstChoiceContent(resp)
	if !ok {
		return []*CheckResult{noContent(resultImage)}
	}
	params := map[string]any{"expected": c.expected}
	if validateChallenge(answer, c.expected) {
		return []*CheckResult{{Name: resultImage, Status: CheckStatusSuccess, Code: "image_passed", Params: params}}
	}
	params["answer"] = truncate(answer, 200)
	return []*CheckResult{{Name: resultImage, Status: CheckStatusFailed, Code: "image_mismatch", Params: params}}
}
