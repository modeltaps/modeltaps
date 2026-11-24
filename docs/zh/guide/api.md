---
title: "API 接入"
layout: doc
outline: deep
lastUpdated: true
---

# API 接入

Modeltaps 同时提供 OpenAI、Claude、Gemini 三套兼容接口。既有应用无需改代码，只要把接入地址换成 Modeltaps 的站点地址、把 API Key 换成你在 [API Key](/zh/guide/tokens) 页面创建的密钥即可。

下文示例中的 `https://your-modeltaps-domain.com` 请替换为你实际使用的站点地址，`sk-替换为你的key` 替换为你的 API Key。三套协议都使用同一把 API Key。

::: tip 先跑通再接入
在「API Key」页面打开某个 API Key 的「测试命令」，Modeltaps 会把你的密钥、站点地址与可用模型拼成一条可直接运行的 `curl`，复制到终端即可验证。密钥只显示一次，不是刚创建或重置的密钥，需要先在弹窗里粘贴。
:::

## OpenAI 兼容接口

使用方式与 [OpenAI API](https://platform.openai.com/docs/api-reference/introduction) 一致，接入地址为站点根地址。

::: warning API Base 的写法
不同客户端对 API Base 的期望格式不同，若连接失败请依次尝试：

- `https://your-modeltaps-domain.com`
- `https://your-modeltaps-domain.com/v1`
- `https://your-modeltaps-domain.com/v1/chat/completions`
  :::

### 使用示例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/v1/chat/completions \
    --header 'Authorization: Bearer sk-替换为你的key' \
    -H "Content-Type: application/json" \
    --data '{
      "model": "gpt-5-mini",
      "messages": [
          {
              "role": "user",
              "content": "hi~"
          }
      ]
  }'
```

### 可用的接口

除对话之外，`/v1` 下还提供文本补全、Responses、向量化（embeddings）、重排序（rerank）、内容审核（moderations）、图片生成 / 编辑 / 变体、语音合成 / 转写 / 翻译，以及实时（realtime）接口。请求与响应结构与 OpenAI 官方一致。决策模型通过 `/v1/decisions` 调用，见[决策模型](/zh/guide/decisions)。

查询当前 API Key 可用的模型：

```bash
curl https://your-modeltaps-domain.com/v1/models \
  --header 'Authorization: Bearer sk-替换为你的key'
```

## Claude 兼容接口

使用方式与 [Claude API](https://docs.anthropic.com/en/api/messages) 一致，接入地址为 `https://your-modeltaps-domain.com/claude`，鉴权使用 `x-api-key` 头。

### 使用示例

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/claude/v1/messages \
    -H "Content-Type: application/json" \
    -H "x-api-key: sk-替换为你的key" \
    -H "anthropic-version: 2023-06-01" \
    --data '{
  "model": "claude-sonnet-4-5",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "hi~"
    }
  ]
}'
```

查询该协议下可用的模型：

```bash
curl https://your-modeltaps-domain.com/claude/v1/models \
  -H "x-api-key: sk-替换为你的key"
```

## Gemini 兼容接口

接入地址为 `https://your-modeltaps-domain.com/gemini`，鉴权使用 `x-goog-api-key` 头，路径中的版本段（如 `v1beta`）与官方保持一致。

### 使用示例

```bash
curl --request POST \
  --url https://your-modeltaps-domain.com/gemini/v1beta/models/gemini-2.5-pro:generateContent \
  --header 'Content-Type: application/json' \
  --header 'x-goog-api-key: sk-替换为你的key' \
  --data '{
	"contents": [
		{
			"role": "user",
			"parts": [
				{
					"text": "hi~"
				}
			]
		}
	]
}'
```

查询该协议下可用的模型：

```bash
curl https://your-modeltaps-domain.com/gemini/v1beta/models \
  -H "x-goog-api-key: sk-替换为你的key"
```

### Live API（实时语音与视频）

Gemini Live 会话（语音、摄像头画面、屏幕共享走同一条 WebSocket）也用这个基础地址。Google SDK 会自己拼出完整地址，设置好 base URL 照常连接即可：

```text
wss://your-modeltaps-domain.com/gemini/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent
```

```python
from google import genai
from google.genai import types

client = genai.Client(
    api_key="sk-replace-with-your-key",
    http_options=types.HttpOptions(base_url="https://your-modeltaps-domain.com/gemini"),
)

async with client.aio.live.connect(model="gemini-3.8-live", config={"response_modalities": ["AUDIO"]}) as session:
    ...
```

- 密钥放在 `x-goog-api-key` 请求头或 `key` 查询参数里。支持 `v1`、`v1alpha` 和 `v1beta`（SDK 设了自定义 base URL 时用 `v1`，转发上游时按 `v1beta`）；暂不支持临时令牌（`BidiGenerateContentConstrained`）。
- 模型取自第一条消息的 `setup.model`，会话路由到带有该模型的 Gemini 渠道（Google AI Studio 密钥或服务账号），消息原样转发。
- 每一轮按模型返回的 `usageMetadata` 计费：按模态区分的输入 token（文本、音频、图片和视频帧）、输出 token 和思考 token。密钥、余额或预算用完时会话结束。
- 错误和 Google 一样以带原因的 WebSocket 关闭帧返回：1007 表示 setup 无效，1008 表示请求被拒（密钥、额度、模型、内容审核）。

## 编程 Agent

编程 Agent 有各自的专属接入地址。路径表示是哪个 Agent 在调用，协议仍由后面的路径决定；鉴权、路由、计费和错误都与上面的接口相同。不同的只有模型列表：每个地址按对应 Agent 能识别的格式返回，Agent 的模型选择器里就能看到你的 API Key 可调用的模型。

| 接入地址 | 协议 | 适用 |
| --- | --- | --- |
| `https://your-modeltaps-domain.com/claude-code` | Anthropic Messages：`/v1/messages`、`/v1/messages/count_tokens`、`/v1/models` | Claude Code 与 Claude Agent SDK |
| `https://your-modeltaps-domain.com/codex/v1` | OpenAI：`/responses`、`/chat/completions` 以及 `/v1` 下的其他接口 | Codex CLI |
| `https://your-modeltaps-domain.com/coding-agent/v1` | OpenAI，与 `/v1` 相同 | 其他 OpenAI 兼容的编程 Agent |
| `https://your-modeltaps-domain.com/coding-agent` | Anthropic Messages，与 `/claude` 相同 | 其他 Anthropic 兼容的编程 Agent |

用量日志会记下每次请求来自哪个 Agent：以接入地址为准，走其他地址时按 User-Agent 识别（Claude Code、Codex、OpenCode、Gemini CLI）。组织可以按工具查看用量，见 [组织](/zh/guide/organization#用量)。

控制台可以直接生成这些配置：在 **API Key** 页打开某把 Key 的 ⋯ 菜单（或页头的 **接入** 按钮），选 **接入工具**，会填好站点地址、Key 和默认模型。

### Claude Code {#claude-code}

把网关写进 `~/.claude/settings.json`（也可以在 shell 里 export 同样的变量）：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-modeltaps-domain.com/claude-code",
    "ANTHROPIC_AUTH_TOKEN": "sk-替换为你的key",
    "ANTHROPIC_API_KEY": "",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

- 接入地址不带 `/v1`：Claude Code 会自己补上 `/v1/messages`。
- `ANTHROPIC_API_KEY` 留空。它有值时 Claude Code 会优先用它，而不是 `ANTHROPIC_AUTH_TOKEN`。
- 设置 `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` 后，`/model` 会以 `claude-code/<模型>` 的形式列出你的 Key 可调用的全部对话模型。可接受一百万 tokens 及以上输入的模型以 `[1m]` 结尾，Claude Code 会据此使用该上下文长度。网关会去掉前缀和后缀，用量与日志里显示的是真实模型名。生图、音频和向量模型不会列出。
- `cache_control` 缓存断点原样送到上游，同一个 Claude Code 会话的请求固定在同一个渠道上，上游提示缓存可以持续命中。`/context` 的 token 数来自 `/v1/messages/count_tokens`，这是估算值，不计费。

### Codex {#codex}

在 `~/.codex/config.toml` 里加一个 provider，并把 API Key 放进它指定的环境变量：

```toml
model_provider = "modeltaps"
model = "gpt-5.5"

[model_providers.modeltaps]
name = "Modeltaps"
base_url = "https://your-modeltaps-domain.com/codex/v1"
env_key = "MODELTAPS_API_KEY"
wire_api = "responses"
model_catalog_url = "https://your-modeltaps-domain.com/codex/v1/models"
```

```bash
export MODELTAPS_API_KEY=sk-替换为你的key
```

- `wire_api` 必须是 `responses`：Codex 只支持 Responses API。
- `model_catalog_url` 让 Codex 从 Modeltaps 读取模型目录：`/model` 里能看到你的 Key 可调用的对话模型，Codex 也不再提示找不到模型元数据。每个条目带的是 Codex 自己的系统提示，行为和用它内置模型时一样。
- Codex 发送的 `prompt_cache_key` 原样送到上游，同一个 Codex 会话的请求固定在同一个渠道上。

### OpenCode {#opencode}

在 `~/.config/opencode/opencode.json` 里加一个 provider，并把 API Key 放进 `MODELTAPS_API_KEY`：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "modeltaps": {
      "npm": "@ai-sdk/anthropic",
      "name": "Modeltaps",
      "options": { "baseURL": "https://your-modeltaps-domain.com/coding-agent/v1", "apiKey": "{env:MODELTAPS_API_KEY}" },
      "models": { "claude-sonnet-5-5": {} }
    }
  },
  "model": "modeltaps/claude-sonnet-5-5"
}
```

- Claude 模型用 `@ai-sdk/anthropic` 包，OpenCode 会带上 `cache_control`，提示缓存才能生效；其他模型用 `@ai-sdk/openai-compatible`，`baseURL` 相同。
- 在 `models` 里列出要用的模型；在 OpenCode 里运行 `/models` 检查。

### Goose {#goose}

Claude 模型用 Goose 内置的 Anthropic provider，host 指向 `/coding-agent`（不带 `/v1`：Goose 会自己补 `/v1/messages`）。这个 provider 会自己加提示缓存标记。

```bash
export GOOSE_PROVIDER=anthropic
export ANTHROPIC_HOST=https://your-modeltaps-domain.com/coding-agent
export GOOSE_MODEL=claude-sonnet-5-5
goose configure   # 选 Anthropic，输入 API Key
```

其他模型在 `~/.config/goose/custom_providers/modeltaps.json` 加一个自定义的 OpenAI 兼容 provider，再在 `~/.config/goose/config.yaml` 设 `GOOSE_PROVIDER: modeltaps` 和 `GOOSE_MODEL`：

```json
{
  "name": "modeltaps",
  "engine": "openai",
  "display_name": "Modeltaps",
  "api_key_env": "MODELTAPS_API_KEY",
  "base_url": "https://your-modeltaps-domain.com/coding-agent/v1/chat/completions",
  "dynamic_models": true,
  "models": [{ "name": "gpt-5.5", "context_limit": 128000 }]
}
```

- `dynamic_models` 让 Goose 的模型列表取自你的 Key 可调用的模型。
- `goose configure` 会把 Key 存进系统钥匙串。桌面版里同样的设置在「设置 → 模型」。

### 其他编程 Agent {#other-coding-agents}

OpenAI 兼容的 Agent 指向 `https://your-modeltaps-domain.com/coding-agent/v1`，Anthropic 兼容的指向 `https://your-modeltaps-domain.com/coding-agent`。行为与 `/v1`、`/claude` 完全一致，请求会计入编程 Agent 流量。

## 在控制台查看接口目录

侧边栏「API」分组按模态列出各页。`/panel/docs` 是总览，每张模态卡片可进入「接口文档」或「控制台」。接口文档页的内容与本页一致，但地址已经换成你实际使用的站点地址：

| 模态 | 接口文档 | 控制台 | 覆盖的接口 |
| --- | --- | --- | --- |
| 对话 | `/panel/docs/chat` | `/panel/playground/chat` | `/v1/chat/completions`、`/v1/responses`、`/claude/v1/messages`、`/gemini/v1beta/models/{model}:generateContent` |
| 图像 | `/panel/docs/image` | `/panel/playground/image` | `/v1/images/generations`、`/v1/images/edits`、`/v1/images/variations`，以及 Recraft、Midjourney 扩展接口 |
| 语音 | `/panel/docs/speech` | `/panel/playground/speech` | `/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`，以及 `/v1/realtime` |
| 视频 | `/panel/docs/video` | —（暂未开放） | Kling 的文生视频 / 图生视频与任务查询，以及 Gemini Veo 长任务接口 |

每个接口文档页都包含「接口端点」「请求示例」「可用模型」与完整文档链接。示例里的密钥是占位符 `sk-YOUR_TOKEN`，替换成你在「API Key」页面创建的 API Key 即可运行。

### 控制台

「对话」「图像」「语音」三页即控制台，直接调用本站 `/v1`：从示例卡片或底部输入栏开始，模型与参数以输入栏上的 chip 选择。对话为流式输出、可随时停止，逐轮显示首 token 延迟、耗时与用量；图像可预览与下载生成结果；语音分「语音合成」「语音识别」两个分段，可试听与下载合成音频，也可上传或录制音频做转写。每页都能查看本次请求的等效代码，照着改就能接入自己的程序。

模型选择器是贴在模型 chip 上的菜单，支持搜索、按能力筛选与按供应商分类，只列出该能力下可用的模型。

- 控制台用的是系统自动为你创建的专用密钥，不会展示也无需复制，页面用一个短期凭证发起请求（有效一小时，自动续期），**用量与费用计入你的个人账户**，与手动创建的 API Key 分开计费；
- 只在个人上下文可用，切换到组织后页面会提示并给出出口（组织额度的试用另行支持）；
- 某个能力当前没有可用模型时，页面仍可浏览，并说明原因与处理入口，不会发出请求；
- 管理员可在系统设置里把 `builtin_chat_enabled` 关掉，整站停用控制台；
- 旧的 Playground 地址（`/playground`、`/panel/playground`）会自动跳转到 `/panel/playground/chat`。

#### 对比

侧边栏「API」分组里的「对比」（`/panel/playground/compare`）把 2–4 个对话模型并排成列：右上「添加列」最多加到 4 列，每列在列头选模型，也可在列头覆盖温度、最大 tokens 与思考深度。底部输入框的「同步输入」开着时一条消息同时发给所有列，关着只发给当前选中的列，每列可单独停止。「查看代码」可在列间切换，给出该列合并覆盖参数后的等效请求。

Midjourney 与异步任务的执行记录不在目录页里，请到「日志」页面的「Midjourney」「异步任务」标签查看。

## 关于模型与分组

- 你能调用哪些模型，取决于 API Key 所选的渠道分组，以及 API Key 是否开启了模型限制。可用清单在「仪表盘」的「当前可用模型」中查看，也可以用上面各协议的 models 接口查询。
- `/claude/v1/messages` 的请求可以调用任意对话模型：渠道不支持 Anthropic 协议时，网关会转换请求。无论调用哪个接口，只要渠道支持，Claude 模型都会用 Anthropic 协议发往上游，提示缓存因此保持有效。
- Gemini 协议的请求只能路由到 Gemini 类型的渠道。若 Gemini 协议查不到模型，说明当前分组内没有这类渠道，改用 OpenAI 兼容接口即可。
- 请求失败时可到 [用量与日志](/zh/guide/usage) 页面按 API Key 与时间定位具体那一条记录，明细里会显示结束原因与计费构成。

## 相关

- [推理设置](/zh/guide/reasoning)：控制推理模型的思考行为
- [特殊用法](/zh/guide/special)：非通用场景的调用方式
- [API Key 与额度](/zh/guide/tokens)：额度、有效期、模型限制与 IP 白名单
- [常见问题](/zh/guide/faq)：接入报错排查
