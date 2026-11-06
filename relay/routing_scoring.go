package relay

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"

	"github.com/tidwall/gjson"
)

// logprobs 打分：把路由标签编成选项字母 A…P，让决策模型只回一个字母，再从第一个输出 token 的
// top_logprobs 里读出每个字母的概率。这样阈值比较的是模型真实的输出概率，而不是它自报的数字。
// 决策模型或渠道不返回 logprobs 时（Claude、推理模型、多数聚合网关），auto 模式改用自报置信度。

const routingLetterSystemPrompt = `You are a request router. You will be given a question, a list of lettered options, and the first message of a user's request. Pick the single option that best answers the question for that request. Reply with the option's letter only, no other text.`

// routingLetterMaxTokens 只需要第一个 token；有的上游（经聚合网关的部分 OpenAI 模型）要求不少于 16。
const routingLetterMaxTokens = 16

// routingTopLogprobs OpenAI 兼容接口允许的上限。
const routingTopLogprobs = 20

// routingMinCoverage 选项字母的概率之和低于它，说明模型没在回答选项，本次打分不可信。
const routingMinCoverage = 0.5

// routingLogprobsCacheTTL 记住某渠道上的决策模型不返回 logprobs 的时长。
const routingLogprobsCacheTTL = 24 * time.Hour

const routingOptionLetters = "ABCDEFGHIJKLMNOP"

// errNoLogprobs 上游成功应答但没带 logprobs：这个渠道上的决策模型不支持概率打分。
var errNoLogprobs = errors.New("decision model did not return logprobs")

func buildLetterMessages(policy *model.RoutingPolicy, text string) []types.ChatCompletionMessage {
	var sb strings.Builder
	if policy.Question != "" {
		sb.WriteString(policy.Question)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Options:\n")
	for i, route := range policy.Routes {
		sb.WriteByte(routingOptionLetters[i])
		sb.WriteString(". ")
		sb.WriteString(route.Label)
		if route.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(route.Description)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nUser request:\n<<<\n")
	sb.WriteString(text)
	sb.WriteString("\n>>>\n\nAnswer with one letter.")
	return []types.ChatCompletionMessage{
		{Role: types.ChatMessageRoleSystem, Content: routingLetterSystemPrompt},
		{Role: types.ChatMessageRoleUser, Content: sb.String()},
	}
}

func buildLetterRequest(policy *model.RoutingPolicy, upstreamModel, text string) *types.ChatCompletionRequest {
	temperature := 0.0
	logprobs := true
	return &types.ChatCompletionRequest{
		Model:       upstreamModel,
		Messages:    buildLetterMessages(policy, text),
		MaxTokens:   routingLetterMaxTokens,
		Temperature: &temperature,
		LogProbs:    &logprobs,
		TopLogProbs: routingTopLogprobs,
	}
}

// optionLetterIndex 把一个 token 认成选项字母：去掉空白和常见的跟随标点，大小写不敏感。
func optionLetterIndex(token string, routes int) int {
	token = strings.TrimSpace(token)
	token = strings.TrimRight(token, ".):：）")
	if len(token) != 1 {
		return -1
	}
	index := strings.IndexByte(routingOptionLetters, strings.ToUpper(token)[0])
	if index < 0 || index >= routes {
		return -1
	}
	return index
}

// parseLetterLogprobs 从 choices[0].logprobs 里算出每个标签的概率。logprobs 缺失返回 errNoLogprobs；
// 选项字母的概率之和低于 routingMinCoverage 返回普通错误（走兜底）。
func parseLetterLogprobs(policy *model.RoutingPolicy, logprobs any) (*ClassifyResult, error) {
	if logprobs == nil {
		return nil, errNoLogprobs
	}
	raw, err := json.Marshal(logprobs)
	if err != nil {
		return nil, errNoLogprobs
	}
	tokens := gjson.GetBytes(raw, "content")
	if !tokens.IsArray() || len(tokens.Array()) == 0 {
		return nil, errNoLogprobs
	}
	// 第一个非空白 token 才是答案：有的模型先吐一个换行或空格。
	var answer gjson.Result
	for _, token := range tokens.Array() {
		if strings.TrimSpace(token.Get("token").String()) != "" {
			answer = token
			break
		}
	}
	if !answer.Exists() {
		return nil, errors.New("decision model returned an empty answer")
	}
	candidates := answer.Get("top_logprobs").Array()
	if len(candidates) == 0 {
		// 只给了被采样 token 的 logprob：只能知道这一个选项的概率。
		candidates = []gjson.Result{answer}
	}

	routes := len(policy.Routes)
	mass := make([]float64, routes)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		token := candidate.Get("token").String()
		if seen[token] {
			continue
		}
		seen[token] = true
		index := optionLetterIndex(token, routes)
		if index < 0 {
			continue
		}
		mass[index] += math.Exp(candidate.Get("logprob").Float())
	}
	coverage := 0.0
	for _, m := range mass {
		coverage += m
	}
	if coverage < routingMinCoverage {
		return nil, errors.New("decision model did not answer with an option letter (coverage " + strconv.FormatFloat(coverage, 'f', 2, 64) + ")")
	}

	probabilities := make([]relay_util.RoutingLabelProbability, 0, routes)
	for i, route := range policy.Routes {
		probabilities = append(probabilities, relay_util.RoutingLabelProbability{Label: route.Label, Probability: mass[i] / coverage})
	}
	sort.SliceStable(probabilities, func(i, j int) bool { return probabilities[i].Probability > probabilities[j].Probability })
	return &ClassifyResult{
		Label:         probabilities[0].Label,
		Confidence:    probabilities[0].Probability,
		Scoring:       model.RoutingScoringLogprobs,
		Probabilities: probabilities,
		Coverage:      math.Min(coverage, 1),
	}, nil
}

// isLogprobsRejection 上游因为 logprobs 参数本身拒绝了请求（而不是鉴权、额度等别的原因）。
func isLogprobsRejection(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "logprob")
}

// ---- 能力缓存：按「渠道 + 上游模型名」记住不返回 logprobs 的组合，只在本实例内存里 ----

type logprobsSupportEntry struct {
	supported bool
	expires   time.Time
}

var logprobsSupport sync.Map // key string -> logprobsSupportEntry

func logprobsSupportKey(channelID int, upstreamModel string) string {
	return strconv.Itoa(channelID) + "\x00" + upstreamModel
}

// lookupLogprobsSupport 返回已知结论；没有或已过期时 known 为 false。
func lookupLogprobsSupport(channelID int, upstreamModel string) (supported, known bool) {
	value, ok := logprobsSupport.Load(logprobsSupportKey(channelID, upstreamModel))
	if !ok {
		return false, false
	}
	entry := value.(logprobsSupportEntry)
	if routingPolicyNow().After(entry.expires) {
		logprobsSupport.Delete(logprobsSupportKey(channelID, upstreamModel))
		return false, false
	}
	return entry.supported, true
}

func rememberLogprobsSupport(channelID int, upstreamModel string, supported bool) {
	logprobsSupport.Store(logprobsSupportKey(channelID, upstreamModel), logprobsSupportEntry{
		supported: supported,
		expires:   routingPolicyNow().Add(routingLogprobsCacheTTL),
	})
}

// ResetRoutingScoringCache 清空 logprobs 能力缓存（测试用）。
func ResetRoutingScoringCache() {
	logprobsSupport.Range(func(key, _ any) bool {
		logprobsSupport.Delete(key)
		return true
	})
}
