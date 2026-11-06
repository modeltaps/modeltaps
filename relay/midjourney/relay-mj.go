// Path: relay/relay-mj.go
package midjourney

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/controller"
	"github.com/modeltaps/modeltaps/model"
	provider "github.com/modeltaps/modeltaps/providers/midjourney"
	"github.com/modeltaps/modeltaps/relay"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// mjImageMaxBytes 限制 /mj/image 回流的图片大小。
const mjImageMaxBytes int64 = 50 << 20

// mjImagePublicClient 抓取不属于渠道上游主机的图片地址（通常是 Discord/CDN）：
// 拨号时拒绝私网/回环/链路本地地址，重定向逐跳复检，防 SSRF。
var mjImagePublicClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           safefetch.GuardedDialer(5 * time.Second).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
	Timeout:       60 * time.Second,
	CheckRedirect: safefetch.CheckRedirect(3),
}

// mjImageUpstreamClient 抓取与任务所属渠道 base URL 同主机的图片：该地址由管理员配置，
// 可能是内网部署的 Midjourney 代理，因此不做私网拦截，但不跟随重定向。
var mjImageUpstreamClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           utils.NewDialer().DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
	Timeout: 60 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// mjImageClient 按图片地址选择 client：与渠道 base URL 同 host:port 才允许访问私网。
func mjImageClient(task *model.Midjourney, imageURL *url.URL) *http.Client {
	if task.ChannelId > 0 {
		if channel, err := model.GetChannelById(task.ChannelId); err == nil && channel.GetBaseURL() != "" {
			if base, err := url.Parse(channel.GetBaseURL()); err == nil && base.Host != "" &&
				strings.EqualFold(base.Host, imageURL.Host) && base.Scheme == imageURL.Scheme {
				return mjImageUpstreamClient
			}
		}
	}
	return mjImagePublicClient
}

func RelayMidjourneyImage(c *gin.Context) {
	taskId := c.Param("id")
	midjourneyTask := model.GetByOnlyMJId(taskId)
	if midjourneyTask == nil {
		c.JSON(400, gin.H{
			"error": "midjourney_task_not_found",
		})
		return
	}
	imageURL, err := safefetch.CheckURL(midjourneyTask.ImageUrl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_image_url",
		})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, imageURL.String(), nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_image_url",
		})
		return
	}
	resp, err := mjImageClient(midjourneyTask, imageURL).Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "http_get_image_failed",
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "http_get_image_failed",
		})
		return
	}
	if resp.ContentLength > mjImageMaxBytes {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "image_too_large",
		})
		return
	}
	// 只回流图片，避免把任意上游内容（HTML/JSON 等）以本站源返回给浏览器
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "unexpected_content_type",
		})
		return
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = io.Copy(c.Writer, io.LimitReader(resp.Body, mjImageMaxBytes))
	if err != nil {
		log.Println("Failed to stream image:", err)
	}
}

// RelayMidjourneyNotify 原本接收上游代理的任务回调，但网关从不下发 notifyHook（任务状态靠轮询同步），
// 所以这个入口只会被客户端调用；它按任务 ID 改写任意任务的状态和图片地址，
// 会造成跨租户篡改和 SSRF，因此一律拒绝。
func RelayMidjourneyNotify(c *gin.Context) *provider.MidjourneyResponse {
	return &provider.MidjourneyResponse{
		Code:        4,
		Description: "notify_not_supported",
		Properties:  nil,
		Result:      "",
	}
}

func coverMidjourneyTaskDto(originTask *model.Midjourney) (midjourneyTask provider.MidjourneyDto) {
	midjourneyTask.MjId = originTask.MjId
	midjourneyTask.Progress = originTask.Progress
	midjourneyTask.PromptEn = originTask.PromptEn
	midjourneyTask.State = originTask.State
	midjourneyTask.SubmitTime = originTask.SubmitTime
	midjourneyTask.StartTime = originTask.StartTime
	midjourneyTask.FinishTime = originTask.FinishTime
	midjourneyTask.ImageUrl = ""

	midjourneyTask.ImageUrl = originTask.ImageUrl
	midjourneyTask.Status = originTask.Status
	midjourneyTask.FailReason = originTask.FailReason
	midjourneyTask.Action = originTask.Action
	midjourneyTask.Description = originTask.Description
	midjourneyTask.Prompt = originTask.Prompt
	if originTask.Buttons != "" {
		var buttons []provider.ActionButton
		err := json.Unmarshal([]byte(originTask.Buttons), &buttons)
		if err == nil {
			midjourneyTask.Buttons = buttons
		}
	}
	if originTask.Properties != "" {
		var properties provider.Properties
		err := json.Unmarshal([]byte(originTask.Properties), &properties)
		if err == nil {
			midjourneyTask.Properties = &properties
		}
	}
	return
}

func RelaySwapFace(c *gin.Context) *provider.MidjourneyResponse {
	mjProvider, errWithMJ := getMJProviderWithRequest(c, provider.RelayModeMidjourneySwapFace, nil)
	if errWithMJ != nil {
		return errWithMJ
	}

	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	var swapFaceRequest provider.SwapFaceRequest
	err := common.UnmarshalBodyReusable(c, &swapFaceRequest)
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "bind_request_body_failed")
	}
	if swapFaceRequest.SourceBase64 == "" || swapFaceRequest.TargetBase64 == "" {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "sour_base64_and_target_base64_is_required")
	}

	quotaInstance, errWithOA := getQuota(c, provider.MjActionSwapFace)
	if errWithOA != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: errWithOA.Message,
		}
	}
	requestURL := getMjRequestPath(c.Request.URL.String())

	mjResp, _, err := mjProvider.Send(60, requestURL)
	if err != nil {
		quotaInstance.Undo(c)
		return &mjResp.Response
	}

	defer func(ctx context.Context) {
		if mjResp.StatusCode == 200 && mjResp.Response.Code == 1 {
			quotaInstance.Consume(c, &types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1}, false)
		} else {
			quotaInstance.Undo(c)
		}
	}(c.Request.Context())

	quota := quotaInstance.GetTotalQuotaByUsage(&types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1})

	midjResponse := &mjResp.Response

	// 如果帐号负载已满，且是fast模式，则更改为relax模式
	mjModelType := c.GetString("mj_model")
	if midjResponse.Code == 3 && mjModelType == "fast" {
		logger.LogWarn(c, "No MJ account available, trying to switch to relax mode")
		// 退钱
		quotaInstance.Undo(c)
		c.Set("mj_model", "relax")
		mjModelType = "relax"
		quotaInstance, errWithOA = getQuota(c, provider.MjActionSwapFace)
		if errWithOA != nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: errWithOA.Message,
			}
		}
		mjResp, _, err = mjProvider.Send(60, requestURL)
		if err != nil {
			quotaInstance.Undo(c)
			return &mjResp.Response
		}
		quota = quotaInstance.GetTotalQuotaByUsage(&types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1})

		midjResponse = &mjResp.Response
	}

	midjourneyTask := &model.Midjourney{
		UserId:      userId,
		TokenID:     tokenId,
		Code:        midjResponse.Code,
		Action:      provider.MjActionSwapFace,
		MjId:        midjResponse.Result,
		Prompt:      "InsightFace",
		PromptEn:    "",
		Description: midjResponse.Description,
		State:       "",
		SubmitTime:  startTime,
		StartTime:   time.Now().UnixNano() / int64(time.Millisecond),
		FinishTime:  0,
		ImageUrl:    "",
		Status:      "",
		Progress:    "0%",
		FailReason:  "",
		ChannelId:   c.GetInt("channel_id"),
		Quota:       quota,
		Mode:        mjModelType,
	}
	err = midjourneyTask.Insert()
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "insert_midjourney_task_failed")
	}
	// 开始激活任务
	controller.ActivateUpdateMidjourneyTaskBulk()

	c.Writer.WriteHeader(mjResp.StatusCode)
	respBody, err := json.Marshal(midjResponse)
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "unmarshal_response_body_failed")
	}
	_, err = io.Copy(c.Writer, bytes.NewBuffer(respBody))
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "copy_response_body_failed")
	}
	return nil
}

func RelayMidjourneyTaskImageSeed(c *gin.Context) *provider.MidjourneyResponse {
	taskId := c.Param("id")
	userId := c.GetInt("id")
	originTask := model.GetByMJId(userId, taskId)
	if originTask == nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "task_no_found")
	}

	mjProvider, errWithMJ := getMJProviderWithChannelId(c, originTask.ChannelId)
	if errWithMJ != nil {
		return errWithMJ
	}

	requestURL := getMjRequestPath(c.Request.URL.String())
	midjResponseWithStatus, _, err := mjProvider.Send(30, requestURL)
	if err != nil {
		return &midjResponseWithStatus.Response
	}
	midjResponse := &midjResponseWithStatus.Response
	c.Writer.WriteHeader(midjResponseWithStatus.StatusCode)
	respBody, err := json.Marshal(midjResponse)
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "unmarshal_response_body_failed")
	}
	_, err = io.Copy(c.Writer, bytes.NewBuffer(respBody))
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "copy_response_body_failed")
	}
	return nil
}

func RelayMidjourneyTask(c *gin.Context, relayMode int) *provider.MidjourneyResponse {
	userId := c.GetInt("id")
	var err error
	var respBody []byte
	switch relayMode {
	case provider.RelayModeMidjourneyTaskFetch:
		taskId := c.Param("id")
		originTask := model.GetByMJId(userId, taskId)
		if originTask == nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: "task_no_found",
			}
		}
		midjourneyTask := coverMidjourneyTaskDto(originTask)
		respBody, err = json.Marshal(midjourneyTask)
		if err != nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: "unmarshal_response_body_failed",
			}
		}
	case provider.RelayModeMidjourneyTaskFetchByCondition:
		var condition = struct {
			IDs []string `json:"ids"`
		}{}
		err = c.BindJSON(&condition)
		if err != nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: "do_request_failed",
			}
		}
		var tasks []provider.MidjourneyDto
		if len(condition.IDs) != 0 {
			originTasks := model.GetByMJIds(userId, condition.IDs)
			for _, originTask := range originTasks {
				midjourneyTask := coverMidjourneyTaskDto(originTask)
				tasks = append(tasks, midjourneyTask)
			}
		}
		if tasks == nil {
			tasks = make([]provider.MidjourneyDto, 0)
		}
		respBody, err = json.Marshal(tasks)
		if err != nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: "unmarshal_response_body_failed",
			}
		}
	}

	c.Writer.Header().Set("Content-Type", "application/json")

	_, err = io.Copy(c.Writer, bytes.NewBuffer(respBody))
	if err != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: "copy_response_body_failed",
		}
	}
	return nil
}

func RelayMidjourneySubmit(c *gin.Context, relayMode int) *provider.MidjourneyResponse {
	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	consumeQuota := true
	mjModelType := c.GetString("mj_model")
	var midjRequest provider.MidjourneyRequest
	err := common.UnmarshalBodyReusable(c, &midjRequest)
	if err != nil {
		return provider.MidjourneyErrorWrapper(provider.MjRequestError, "bind_request_body_failed")
	}

	mjProvider, errWithMJ := getMJProviderWithRequest(c, relayMode, &midjRequest)
	if errWithMJ != nil {
		return errWithMJ
	}

	if relayMode == provider.RelayModeMidjourneyAction { // midjourney plus，需要从customId中获取任务信息
		mjErr := CoverPlusActionToNormalAction(&midjRequest)
		if mjErr != nil {
			return mjErr
		}
		relayMode = provider.RelayModeMidjourneyChange
	}

	if relayMode == provider.RelayModeMidjourneyImagine { //绘画任务，此类任务可重复
		if midjRequest.Prompt == "" {
			return provider.MidjourneyErrorWrapper(provider.MjRequestError, "prompt_is_required")
		}
		midjRequest.Action = provider.MjActionImagine
	} else if relayMode == provider.RelayModeMidjourneyDescribe { //按图生文任务，此类任务可重复
		midjRequest.Action = provider.MjActionDescribe
	} else if relayMode == provider.RelayModeMidjourneyShorten { //缩短任务，此类任务可重复，plus only
		midjRequest.Action = provider.MjActionShorten
	} else if relayMode == provider.RelayModeMidjourneyBlend { //绘画任务，此类任务可重复
		midjRequest.Action = provider.MjActionBlend
	} else if relayMode == provider.RelayModeMidjourneyUpload { //绘画任务，此类任务可重复
		midjRequest.Action = provider.MjActionUpload
	} else if midjRequest.TaskId != "" { //放大、变换任务，此类任务，如果重复且已有结果，远端api会直接返回最终结果
		mjId := ""
		if relayMode == provider.RelayModeMidjourneyChange {
			if midjRequest.TaskId == "" {
				return provider.MidjourneyErrorWrapper(provider.MjRequestError, "task_id_is_required")
			} else if midjRequest.Action == "" {
				return provider.MidjourneyErrorWrapper(provider.MjRequestError, "action_is_required")
			} else if midjRequest.Index == 0 {
				return provider.MidjourneyErrorWrapper(provider.MjRequestError, "index_is_required")
			}
			//action = midjRequest.Action
			mjId = midjRequest.TaskId
		} else if relayMode == provider.RelayModeMidjourneySimpleChange {
			if midjRequest.Content == "" {
				return provider.MidjourneyErrorWrapper(provider.MjRequestError, "content_is_required")
			}
			params := ConvertSimpleChangeParams(midjRequest.Content)
			if params == nil {
				return provider.MidjourneyErrorWrapper(provider.MjRequestError, "content_parse_failed")
			}
			mjId = params.TaskId
			midjRequest.Action = params.Action
		} else if relayMode == provider.RelayModeMidjourneyModal {
			//if midjRequest.MaskBase64 == "" {
			//	return provider.MidjourneyErrorWrapper(provider.MjRequestError, "mask_base64_is_required")
			//}
			mjId = midjRequest.TaskId
			midjRequest.Action = provider.MjActionModal
		}

		originTask := model.GetByMJId(userId, mjId)
		if originTask == nil {
			return provider.MidjourneyErrorWrapper(provider.MjRequestError, "task_not_found")
		} else if originTask.Status != "SUCCESS" && relayMode != provider.RelayModeMidjourneyModal {
			return provider.MidjourneyErrorWrapper(provider.MjRequestError, "task_status_not_success")
		} else { //原任务的Status=SUCCESS，则可以做放大UPSCALE、变换VARIATION等动作，此时必须使用原来的请求地址才能正确处理
			mjProvider, errWithMJ = getMJProviderWithChannelId(c, originTask.ChannelId)
			if errWithMJ != nil {
				return errWithMJ
			}

			if originTask.Mode != "" {
				mjModelType = originTask.Mode
				c.Set("mj_model", mjModelType)
			}

			log.Printf("Upscale/variation/reroll detected, fetching original channel info: %d", originTask.ChannelId)
		}
		midjRequest.Prompt = originTask.Prompt

		//if channelType == common.ChannelTypeMidjourneyPlus {
		//	// plus
		//} else {
		//	// 普通版渠道
		//
		//}
	}

	if midjRequest.Action == provider.MjActionInPaint || midjRequest.Action == provider.MjActionCustomZoom {
		consumeQuota = false
	}

	//baseURL := common.ChannelBaseURLs[channelType]
	requestURL := getMjRequestPath(c.Request.URL.String())

	//midjRequest.NotifyHook = "http://127.0.0.1:3000/mj/notify"

	quotaInstance, errWithOA := getQuota(c, midjRequest.Action)
	if errWithOA != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: errWithOA.Message,
		}
	}

	midjResponseWithStatus, responseBody, err := mjProvider.Send(60, requestURL)
	if err != nil {
		quotaInstance.Undo(c)
		return &midjResponseWithStatus.Response
	}

	defer func(ctx context.Context) {
		if consumeQuota && midjResponseWithStatus.StatusCode == 200 {
			quotaInstance.Consume(c, &types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1}, false)
		} else {
			quotaInstance.Undo(c)
		}
	}(c.Request.Context())

	quota := quotaInstance.GetTotalQuotaByUsage(&types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1})

	midjResponse := &midjResponseWithStatus.Response

	// 如果帐号负载已满，且是fast模式，则更改为relax模式
	if midjResponse.Code == 3 && mjModelType == "fast" && midjRequest.TaskId == "" {
		logger.LogWarn(c, "No MJ account available, trying to switch to relax mode")
		// 退钱
		quotaInstance.Undo(c)
		mjModelType = "relax"
		c.Set("mj_model", mjModelType)
		quotaInstance, errWithOA = getQuota(c, midjRequest.Action)
		if errWithOA != nil {
			return &provider.MidjourneyResponse{
				Code:        4,
				Description: errWithOA.Message,
			}
		}

		midjResponseWithStatus, responseBody, err = mjProvider.Send(60, requestURL)
		if err != nil {
			quotaInstance.Undo(c)
			return &midjResponseWithStatus.Response
		}
		quota = quotaInstance.GetTotalQuotaByUsage(&types.Usage{CompletionTokens: 0, PromptTokens: 1, TotalTokens: 1})
		midjResponse = &midjResponseWithStatus.Response
	}

	// 文档：https://github.com/novicezk/midjourney-proxy/blob/main/docs/api.md
	//1-提交成功
	// 21-任务已存在（处理中或者有结果了） {"code":21,"description":"任务已存在","result":"0741798445574458","properties":{"status":"SUCCESS","imageUrl":"https://xxxx"}}
	// 22-排队中 {"code":22,"description":"排队中，前面还有1个任务","result":"0741798445574458","properties":{"numberOfQueues":1,"discordInstanceId":"1118138338562560102"}}
	// 23-队列已满，请稍后再试 {"code":23,"description":"队列已满，请稍后尝试","result":"14001929738841620","properties":{"discordInstanceId":"1118138338562560102"}}
	// 24-prompt包含敏感词 {"code":24,"description":"可能包含敏感词","properties":{"promptEn":"nude body","bannedWord":"nude"}}
	// other: 提交错误，description为错误描述
	midjourneyTask := &model.Midjourney{
		UserId:      userId,
		TokenID:     tokenId,
		Code:        midjResponse.Code,
		Action:      midjRequest.Action,
		MjId:        midjResponse.Result,
		Prompt:      midjRequest.Prompt,
		PromptEn:    "",
		Description: midjResponse.Description,
		State:       "",
		SubmitTime:  time.Now().UnixNano() / int64(time.Millisecond),
		StartTime:   0,
		FinishTime:  0,
		ImageUrl:    "",
		Status:      "",
		Progress:    "0%",
		FailReason:  "",
		ChannelId:   c.GetInt("channel_id"),
		Quota:       quota,
		Mode:        mjModelType,
	}

	if midjResponse.Code != 1 && midjResponse.Code != 21 && midjResponse.Code != 22 {
		//非1-提交成功,21-任务已存在和22-排队中，则记录错误原因
		midjourneyTask.FailReason = midjResponse.Description
		consumeQuota = false
	}

	if midjResponse.Code == 21 { //21-任务已存在（处理中或者有结果了）
		// 将 properties 转换为一个 map
		properties, ok := midjResponse.Properties.(map[string]interface{})
		if ok {
			imageUrl, ok1 := properties["imageUrl"].(string)
			status, ok2 := properties["status"].(string)
			if ok1 && ok2 {
				midjourneyTask.ImageUrl = imageUrl
				midjourneyTask.Status = status
				if status == "SUCCESS" {
					midjourneyTask.Progress = "100%"
					midjourneyTask.StartTime = time.Now().UnixNano() / int64(time.Millisecond)
					midjourneyTask.FinishTime = time.Now().UnixNano() / int64(time.Millisecond)
					midjResponse.Code = 1
				}
			}
		}
		//修改返回值
		if midjRequest.Action != provider.MjActionInPaint && midjRequest.Action != provider.MjActionCustomZoom {
			newBody := strings.Replace(string(responseBody), `"code":21`, `"code":1`, -1)
			responseBody = []byte(newBody)
		}
	}

	if midjResponse.Code == 1 && midjRequest.Action == "UPLOAD" {
		midjourneyTask.Progress = "100%"
		midjourneyTask.Status = "SUCCESS"
	}

	err = midjourneyTask.Insert()
	if err != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: "insert_midjourney_task_failed",
		}
	}
	// 开始激活任务
	controller.ActivateUpdateMidjourneyTaskBulk()

	if midjResponse.Code == 22 { //22-排队中，说明任务已存在
		//修改返回值
		newBody := strings.Replace(string(responseBody), `"code":22`, `"code":1`, -1)
		responseBody = []byte(newBody)
	}

	//resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	bodyReader := io.NopCloser(bytes.NewBuffer(responseBody))

	//for k, v := range resp.Header {
	//	c.Writer.Header().Set(k, v[0])
	//}
	c.Writer.WriteHeader(midjResponseWithStatus.StatusCode)

	_, err = io.Copy(c.Writer, bodyReader)
	if err != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: "copy_response_body_failed",
		}
	}
	err = bodyReader.Close()
	if err != nil {
		return &provider.MidjourneyResponse{
			Code:        4,
			Description: "close_response_body_failed",
		}
	}
	return nil
}

func getMjRequestPath(path string) string {
	requestURL := path
	if strings.Contains(requestURL, "/mj-") {
		urls := strings.Split(requestURL, "/mj/")
		if len(urls) < 2 {
			return requestURL
		}
		requestURL = "/mj/" + urls[1]
	}
	return requestURL
}

func getQuota(c *gin.Context, action string) (*relay_util.Quota, *types.OpenAIErrorWithStatusCode) {
	model := c.GetString("mj_model")
	modelName := CoverActionToModelName(action, model)
	quota := relay_util.NewQuota(c, modelName, 1)
	if err := quota.PreQuotaConsumption(); err != nil {
		return nil, err
	}

	return quota, nil
}

func getMJProviderWithRequest(c *gin.Context, relayMode int, request *provider.MidjourneyRequest) (*provider.MidjourneyProvider, *provider.MidjourneyResponse) {
	mjModel := c.GetString("mj_model")
	midjourneyModel, mjErr, _ := GetMjRequestModel(relayMode, request, mjModel)
	if mjErr != nil {
		return nil, MidjourneyErrorFromInternal(mjErr.Code, mjErr.Description)
	}
	if midjourneyModel == "" {
		return nil, MidjourneyErrorFromInternal(provider.MjErrorUnknown, "Invalid request, unable to parse model")
	}

	return getMJProvider(c, midjourneyModel)
}

func getMJProviderWithChannelId(c *gin.Context, channelId int) (*provider.MidjourneyProvider, *provider.MidjourneyResponse) {
	c.Set("specific_channel_id", channelId)

	return getMJProvider(c, "")
}

func getMJProvider(c *gin.Context, modelName string) (*provider.MidjourneyProvider, *provider.MidjourneyResponse) {
	// MJ 伪模型名（mj_imagine 等）无目录行，走豁免入口跳过发布校验
	baseProvider, _, err := relay.GetTaskProvider(c, modelName)
	if err != nil {
		return nil, MidjourneyErrorFromInternal(provider.MjErrorUnknown, "Unable to get provider:"+err.Error())
	}

	mjProvider, ok := baseProvider.(*provider.MidjourneyProvider)
	if !ok {
		return nil, MidjourneyErrorFromInternal(provider.MjErrorUnknown, "Invalid request, unable to get midjourney provider")
	}

	return mjProvider, nil
}
