package check_channel

import (
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/types"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const checkKey = "check_img:%s"

var (
	//go:embed check.png
	checkImage []byte
)

type AccessRecord struct {
	UserAgent string `json:"user_agent"`
	IP        string `json:"ip"`
	Remark    string `json:"remark"`
}

type CheckImgProcess struct {
	ModelName string
	ImageUrl  string
	ID        string
}

func CreateCheckImgProcess(modelName string) *CheckImgProcess {
	c := &CheckImgProcess{
		ModelName: modelName,
	}
	id := utils.GetRandomString(10)
	c.ImageUrl = fmt.Sprintf("%s/api/image/%s", config.ServerAddress, id)
	c.ID = id
	accessRecord := make([]*AccessRecord, 0)
	err := cache.SetCache(fmt.Sprintf(checkKey, id), accessRecord, 10*time.Minute)
	if err != nil {
		return nil
	}
	return c
}

func (c *CheckImgProcess) GetName() string {
	return "Relay detection via image"
}

func (c *CheckImgProcess) GetRequest() *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: c.ModelName,
		Messages: []types.ChatCompletionMessage{
			{
				Role: types.ChatMessageRoleUser,
				Content: []types.ChatMessagePart{
					{
						Type: types.ContentTypeImageURL,
						ImageURL: &types.ChatMessageImageURL{
							URL: c.ImageUrl,
						},
					},
					{
						Type: types.ContentTypeText,
						Text: "Can you see my picture? Please answer 1 or 0. Do not output irrelevant content.",
					},
				},
			},
		},
	}
}

func (c *CheckImgProcess) Check(_ *types.ChatCompletionRequest, resp *types.ChatCompletionResponse, openaiErr *types.OpenAIError) []*CheckResult {
	checkResults := make([]*CheckResult, 0)
	if openaiErr != nil {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: openaiErr.Message,
		})

		return checkResults
	}

	// 响应检测
	if len(resp.Choices) > 0 {
		result := &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: "Failed to get response data",
		}
		if content, ok := resp.Choices[0].Message.Content.(string); ok {
			switch content {
			case "1":
				result.Status = CheckStatusSuccess
				result.Remark = "GPT detected the image"
			case "0":
				result.Status = CheckStatusFailed
				result.Remark = "GPT did not detect the image"
			default:
				result.Status = CheckStatusFailed
				result.Remark = fmt.Sprintf("GPT response does not meet requirements: %s", content)
			}
			checkResults = append(checkResults, result)
		}
	} else {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Response",
			Status: CheckStatusFailed,
			Remark: "No response data returned",
		})
	}

	// 图片请求检测
	accessRecord, err := GetAccessRecord(c.ID)
	if err != nil {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Image request check",
			Status: CheckStatusFailed,
			Remark: fmt.Sprintf("Failed to get request records: %s", err.Error()),
		})
		return checkResults
	}

	if len(accessRecord) == 0 {
		checkResults = append(checkResults, &CheckResult{
			Name:   "Image request check",
			Status: CheckStatusFailed,
			Remark: "No request records",
		})
		return checkResults
	} else {
		result := &CheckResult{
			Name:   "Image request check",
			Status: CheckStatusFailed,
			Remark: "No request records",
		}
		remark := []string{}
		accessRecordLen := len(accessRecord)
		if accessRecordLen > 0 {
			remark = append(remark, fmt.Sprintf("Passed through %d relay(s)", accessRecordLen))

			for index, record := range accessRecord {
				checkChannelImg(record)
				remark = append(remark, fmt.Sprintf("Relay #%d: IP(%s), User-Agent(%s), result(%s)", index+1, record.IP, record.UserAgent, record.Remark))
			}
			result.Remark = strings.Join(remark, "\n")
			result.Status = CheckStatusSuccess
		}
		checkResults = append(checkResults, result)
	}

	return checkResults
}

func AppendAccessRecord(id string, c *gin.Context) error {
	accessRecord, err := GetAccessRecord(id)

	if err != nil {
		return err
	}

	record := &AccessRecord{
		UserAgent: c.Request.UserAgent(),
		IP:        c.ClientIP(),
	}

	checkChannelImg(record)

	accessRecord = append(accessRecord, record)
	return cache.SetCache(fmt.Sprintf(checkKey, id), accessRecord, 10*time.Minute)
}

func GetAccessRecord(id string) ([]*AccessRecord, error) {
	return cache.GetCache[[]*AccessRecord](fmt.Sprintf(checkKey, id))
}

func CheckImageResponse(c *gin.Context) {
	c.Header("Content-Type", "image/png")
	c.Header("Content-Disposition", "inline")

	c.Data(http.StatusOK, "image/png", checkImage)
}

func checkChannelImg(record *AccessRecord) {
	if strings.Contains(record.UserAgent, "OpenAI Image Downloader") {
		record.Remark = "OpenAI"
		return
	}

	if strings.Contains(record.UserAgent, "IPS/1.0") {
		record.Remark = "Azure"
		return
	}

	if strings.Contains(record.UserAgent, "Go-http-client") {
		record.Remark = "Go relay"
		return
	}

	record.Remark = "Unknown"
}
