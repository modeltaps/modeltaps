package e2e

// fake OpenAI 上游：用 httptest.Server 顶替真实 provider，渠道 BaseURL 指向它。
// 四能力：非流式 JSON（带 usage）、SSE 流式、错误注入（状态码/体 + 挂起 N 秒）、请求捕获。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"
)

// CapturedRequest 是 fake 上游收到的一次请求快照，供断言 header / body。
type CapturedRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// JSONBody 把捕获的请求体解析为 map，解析失败直接 t.Fatalf。
func (r CapturedRequest) JSONBody(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("解析捕获的请求体失败: %v (body=%s)", err, string(r.Body))
	}
	return m
}

// FakeResponse 描述 fake 上游下一次（及后续）应答的形态。
// Stream 非空走 SSE，否则 JSON/Raw 走一次性响应；Delay > 0 时先挂起再应答。
type FakeResponse struct {
	Delay   time.Duration
	Status  int               // 0 视为 200
	Headers map[string]string
	JSON    any               // 非 nil 时序列化为响应体
	Raw     string            // JSON 为 nil 时按原样写出
	Stream  []string          // 每项为一条 SSE data 的 JSON 文本，末尾自动补 [DONE]

	// StreamNoDone 模拟流中断：写完 Stream 分片后直接结束响应体，不补 [DONE]。
	// 上游侧表现为半截流（读到 EOF），用于验证 TextBuilder 兜底计费。
	StreamNoDone bool

	// Hook 在捕获请求后、Delay/应答之前执行，用于在「请求仍在上游挂起」的窗口里观测中间态
	// （典型用途：断言预扣已发生，从而让后续的 Undo 回滚断言可证伪）。
	// 运行在 httptest 的 handler goroutine 上，不得调用 t.Fatalf，只应记录后由测试主协程断言。
	Hook func()
}

// FakeProvider 是可配置的假上游。所有配置项在测试内串行设置，读写均加锁。
type FakeProvider struct {
	Server *httptest.Server

	mu       sync.Mutex
	resp     FakeResponse
	requests []CapturedRequest
}

// NewFakeProvider 启动一个假上游，默认对任意路径返回一条带 usage 的非流式 chat 响应。
// 测试结束自动关闭。
func NewFakeProvider(t *testing.T) *FakeProvider {
	t.Helper()
	f := &FakeProvider{
		resp: FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "hello from fake", 10, 5)},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// SetResponse 覆盖后续所有请求的应答形态。
func (f *FakeProvider) SetResponse(resp FakeResponse) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resp = resp
}

// Requests 返回目前为止捕获的全部请求（副本）。
func (f *FakeProvider) Requests() []CapturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]CapturedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// LastRequest 返回最后一次捕获的请求，无请求时 t.Fatalf。
func (f *FakeProvider) LastRequest(t *testing.T) CapturedRequest {
	t.Helper()
	reqs := f.Requests()
	if len(reqs) == 0 {
		t.Fatalf("fake 上游未收到任何请求")
	}
	return reqs[len(reqs)-1]
}

func (f *FakeProvider) serve(w http.ResponseWriter, r *http.Request) {
	body := readAllBody(r)

	f.mu.Lock()
	f.requests = append(f.requests, CapturedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Header: r.Header.Clone(),
		Body:   body,
	})
	resp := f.resp
	f.mu.Unlock()

	if resp.Hook != nil {
		resp.Hook()
	}

	if resp.Delay > 0 {
		select {
		case <-time.After(resp.Delay):
		case <-r.Context().Done():
			return
		}
	}

	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	if len(resp.Stream) > 0 {
		f.writeStream(w, status, resp.Stream, resp.StreamNoDone)
		return
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if resp.JSON != nil {
		_ = json.NewEncoder(w).Encode(resp.JSON)
		return
	}
	_, _ = w.Write([]byte(resp.Raw))
}

func (f *FakeProvider) writeStream(w http.ResponseWriter, status int, chunks []string, noDone bool) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	flusher, _ := w.(http.Flusher)
	for _, chunk := range chunks {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
		if flusher != nil {
			flusher.Flush()
		}
	}
	// noDone：模拟上游中途断开，直接 return 让 httptest 关闭响应体，客户端读到 EOF。
	if noDone {
		return
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func readAllBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	return body
}

// ChatStreamChunksNoUsage 只构造内容分片，不带 usage、也不带 finish_reason。
// 配合 FakeResponse.StreamNoDone 模拟上游中途断开，触发 TextBuilder 兜底计费。
func ChatStreamChunksNoUsage(modelName string, contents []string) []string {
	id := "chatcmpl-" + utils.GetUUID()
	created := utils.GetTimestamp()
	chunks := make([]string, 0, len(contents))
	for _, content := range contents {
		chunks = append(chunks, contentChunk(id, created, modelName, content))
	}
	return chunks
}

// ChatStreamChunks 构造一组 SSE data 分片：逐字输出 contents，末条携带 usage。
// 形态对齐 providers/openai/chat.go 的解析（data: {chunk}\n\n，[DONE] 由 fake 上游补）。
func ChatStreamChunks(modelName string, contents []string, promptTokens, completionTokens int) []string {
	id := "chatcmpl-" + utils.GetUUID()
	created := utils.GetTimestamp()
	chunks := make([]string, 0, len(contents)+1)

	for _, content := range contents {
		chunks = append(chunks, contentChunk(id, created, modelName, content))
	}

	chunks = append(chunks, mustJSON(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   modelName,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}))

	return chunks
}

// contentChunk 构造一条只带 delta.content 的 SSE 分片。
func contentChunk(id string, created int64, modelName, content string) string {
	return mustJSON(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   modelName,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{"content": content},
				"finish_reason": nil,
			},
		},
	})
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// OpenAIErrorJSON 构造 OpenAI 风格错误体，供错误注入使用。
func OpenAIErrorJSON(message, errType, code string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
			"code":    code,
		},
	}
}

// ChatCompletionJSON 构造一条 OpenAI 非流式 chat 响应（含 usage）。
func ChatCompletionJSON(modelName, content string, promptTokens, completionTokens int) map[string]any {
	return map[string]any{
		"id":      "chatcmpl-" + utils.GetUUID(),
		"object":  "chat.completion",
		"created": utils.GetTimestamp(),
		"model":   modelName,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
}
