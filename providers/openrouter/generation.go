package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/gorm"
)

// generation 统计拉取/回填参数(W9-M6)。整条链路异步、失败静默降级,绝不影响主链路/计费/日志写入。
const (
	// 单次 generation 请求的上限,避免侧链路挂死占用连接。
	genFetchTimeout = 8 * time.Second
	// 侧链路 body 读取上限,防御异常大响应。
	genMaxBodyBytes = 1 << 20 // 1MB
)

// genFetchDelays: generation 统计在上游可能滞后于响应结束,按退避重试等待其就绪。
var genFetchDelays = []time.Duration{1500 * time.Millisecond, 3 * time.Second, 5 * time.Second}

// genUpdateDelays: 消费日志在 BatchUpdateEnabled 下按间隔(默认 5s)批量落库,
// 回填需容忍这段延迟,按退避重试直到定位到该 request_id 的日志行。
var genUpdateDelays = []time.Duration{0, 2 * time.Second, 4 * time.Second, 6 * time.Second}

// enrichLogWithGeneration 在独立 TrackedGoroutine 中拉取 OpenRouter generation 统计并回填日志 metadata。
// 任一环节缺参/失败/超时都静默返回,不产生任何主链路副作用。
func enrichLogWithGeneration(baseURL, apiKey, proxyAddr, genID, requestID string) {
	if genID == "" || requestID == "" || apiKey == "" {
		return
	}

	common.TrackedGoroutine(func() {
		data := fetchGenerationWithRetry(baseURL, apiKey, proxyAddr, genID)
		if data == nil {
			return
		}

		extra := map[string]any{}
		if data.ProviderName != "" {
			extra["or_provider"] = data.ProviderName
		}
		if data.Latency > 0 {
			extra["or_latency"] = int(data.Latency)
		}
		if data.GenerationTime > 0 {
			extra["or_generation_time"] = int(data.GenerationTime)
		}
		if len(extra) == 0 {
			return
		}

		for _, d := range genUpdateDelays {
			if d > 0 {
				time.Sleep(d)
			}
			err := model.UpdateLogMetadataByRequestId(requestID, extra)
			if err == nil {
				return
			}
			// 日志尚未落库(批量 flush 未到)时重试;其它错误无重试价值,记一条系统日志后放弃。
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.SysError("openrouter generation log update failed: " + err.Error())
				return
			}
		}
	})
}

// fetchGenerationWithRetry 按退避重试拉取 generation 统计,拿到含 provider 侧字段的数据即返回;
// 全部失败返回 nil(调用方静默降级)。
func fetchGenerationWithRetry(baseURL, apiKey, proxyAddr, genID string) *GenerationData {
	reqURL := strings.TrimSuffix(baseURL, "/") + "/v1/generation?id=" + url.QueryEscape(genID)

	for _, d := range genFetchDelays {
		time.Sleep(d)
		data, err := fetchGeneration(reqURL, apiKey, proxyAddr)
		if err != nil {
			continue
		}
		if data != nil && (data.ProviderName != "" || data.Latency > 0 || data.GenerationTime > 0) {
			return data
		}
	}
	return nil
}

// fetchGeneration 发一次带渠道代理与鉴权的 GET,解析 generation 数据。
// 使用 context 超时兜底(全局 HTTPClient.Timeout 面向长连接过长),只读 Authorization 用渠道 key。
func fetchGeneration(reqURL, apiKey, proxyAddr string) (*GenerationData, error) {
	ctx := utils.SetProxy(proxyAddr, context.Background())
	ctx, cancel := context.WithTimeout(ctx, genFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := requester.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("openrouter generation status " + resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, genMaxBodyBytes))
	if err != nil {
		return nil, err
	}

	var genResp GenerationResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, err
	}
	return &genResp.Data, nil
}

// firstChannelKey 取渠道多 key 的首个非空 key(与 openai 系一致按换行分隔)。
func firstChannelKey(raw string) string {
	for _, k := range strings.Split(raw, "\n") {
		if k = strings.TrimSpace(k); k != "" {
			return k
		}
	}
	return ""
}
