package controller

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/storage"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/gin-gonic/gin"
)

// imageStorageTestSettings fills the secrets the form left empty. As when saving the form, an
// empty secret means "keep the saved one", and the saved one is only reused for the saved
// endpoint, bucket and key id (or, for SM.MS and Imgur, the saved provider), so a test cannot
// hand a stored secret to another host or account.
func imageStorageTestSettings(form, saved storage.Settings) storage.Settings {
	same := func(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }
	if form.S3.AccessKeySecret == "" && same(form.S3.Endpoint, saved.S3.Endpoint) &&
		same(form.S3.BucketName, saved.S3.BucketName) && strings.TrimSpace(form.S3.AccessKeyId) == strings.TrimSpace(saved.S3.AccessKeyId) {
		form.S3.AccessKeySecret = saved.S3.AccessKeySecret
	}
	if form.OSS.AccessKeySecret == "" && same(form.OSS.Endpoint, saved.OSS.Endpoint) &&
		same(form.OSS.BucketName, saved.OSS.BucketName) && strings.TrimSpace(form.OSS.AccessKeyId) == strings.TrimSpace(saved.OSS.AccessKeyId) {
		form.OSS.AccessKeySecret = saved.OSS.AccessKeySecret
	}
	if form.SMMSSecret == "" {
		form.SMMSSecret = saved.SMMSSecret
	}
	if form.ImgurClientId == "" {
		form.ImgurClientId = saved.ImgurClientId
	}
	return form
}

// imageStorageTestPNG is a small image to upload: a 64×64 square in two tones.
func imageStorageTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			c := color.RGBA{R: 0x2f, G: 0x6f, B: 0xeb, A: 0xff}
			if (x/16+y/16)%2 == 1 {
				c = color.RGBA{R: 0xdb, G: 0xe7, B: 0xfd, A: 0xff}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// TestImageStorage uploads a small image with the image storage as it stands in the settings
// form, saved or not, and returns its URL.
func TestImageStorage(c *gin.Context) {
	var form storage.Settings
	if err := c.ShouldBindJSON(&form); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid parameters", nil)
		return
	}
	form.Provider = strings.TrimSpace(form.Provider)
	if !storage.ValidProvider(form.Provider) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeImageStorageProviderInvalid, "Unknown image storage provider", nil)
		return
	}
	settings := imageStorageTestSettings(form, storage.CurrentSettings())
	url, err := storage.UploadWith(settings, imageStorageTestPNG(), "modeltaps-test-"+utils.GetUUID()+".png")
	switch {
	case errors.Is(err, storage.ErrStorageOff):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeImageStorageOff, "Choose an image storage first", nil)
	case errors.Is(err, storage.ErrStorageIncomplete):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeImageStorageIncomplete, "Fill in every required field of the image storage", nil)
	case errors.Is(err, storage.ErrStorageOSSRegion):
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeImageStorageOSSRegion, err.Error(), nil)
	case err != nil:
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeImageStorageTestFailed, "Test upload failed: "+err.Error(), map[string]any{"reason": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"url": url}})
	}
}
