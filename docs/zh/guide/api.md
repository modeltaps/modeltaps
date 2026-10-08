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
在「API Key」页面打开某个 API Key 的「测试命令」，Modeltaps 会把你的真实密钥、站点地址与可用模型拼成一条可直接运行的 `curl`，复制到终端即可验证。
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

除对话之外，`/v1` 下还提供文本补全、Responses、向量化（embeddings）、重排序（rerank）、内容审核（moderations）、图片生成 / 编辑 / 变体、语音合成 / 转写 / 翻译，以及实时（realtime）接口。请求与响应结构与 OpenAI 官方一致。

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

## 在控制台查看接口目录

侧边栏「API」分组按模态列出各页。`/panel/api` 是总览，每张模态卡片可进入「接口文档」或「控制台」。接口文档页的内容与本页一致，但地址已经换成你实际使用的站点地址：

| 模态 | 接口文档 | 控制台 | 覆盖的接口 |
| --- | --- | --- | --- |
| 对话 | `/panel/api/docs/chat` | `/panel/api/chat` | `/v1/chat/completions`、`/v1/responses`、`/claude/v1/messages`、`/gemini/v1beta/models/{model}:generateContent` |
| 图像 | `/panel/api/docs/image` | `/panel/api/image` | `/v1/images/generations`、`/v1/images/edits`、`/v1/images/variations`，以及 Recraft、Midjourney 扩展接口 |
| 语音 | `/panel/api/docs/speech` | `/panel/api/speech` | `/v1/audio/speech`、`/v1/audio/transcriptions`、`/v1/audio/translations`，以及 `/v1/realtime` |
| 视频 | `/panel/api/docs/video` | —（暂未开放） | Kling 的文生视频 / 图生视频与任务查询，以及 Gemini Veo 长任务接口 |

每个接口文档页都包含「接口端点」「请求示例」「可用模型」与完整文档链接。示例里的密钥是占位符 `sk-YOUR_TOKEN`，替换成你在「API Key」页面创建的 API Key 即可运行。

### 控制台

「对话」「图像」「语音」三页即控制台，直接调用本站 `/v1`：从示例卡片或底部输入栏开始，模型与参数以输入栏上的 chip 选择。对话为流式输出、可随时停止，逐轮显示首 token 延迟、耗时与用量；图像可预览与下载生成结果；语音分「语音合成」「语音识别」两个分段，可试听与下载合成音频，也可上传或录制音频做转写。每页都能查看本次请求的等效代码，照着改就能接入自己的程序。

模型选择器是贴在模型 chip 上的菜单，支持搜索、按能力筛选与按供应商分类，只列出该能力下可用的模型。

- 控制台用的是系统自动为你创建的专用密钥，不会展示也无需复制，**用量与费用计入你的个人账户**，与手动创建的 API Key 分开计费；
- 只在个人上下文可用，切换到组织后页面会提示并给出出口（组织额度的试用另行支持）；
- 某个能力当前没有可用模型时，页面仍可浏览，并说明原因与处理入口，不会发出请求；
- 管理员可在系统设置里把 `builtin_chat_enabled` 关掉，整站停用控制台；
- 旧的 Playground 地址（`/playground`、`/panel/playground`）会自动跳转到 `/panel/api/chat`。

#### 对比

侧边栏「API」分组里的「对比」（`/panel/api/compare`）把 2–4 个对话模型并排成列：右上「添加列」最多加到 4 列，每列在列头选模型，也可在列头覆盖温度、最大 tokens 与思考深度。底部输入框的「同步输入」开着时一条消息同时发给所有列，关着只发给当前选中的列，每列可单独停止。「查看代码」可在列间切换，给出该列合并覆盖参数后的等效请求。

Midjourney 与异步任务的执行记录不在目录页里，请到「日志」页面的「Midjourney」「异步任务」标签查看。

## 关于模型与分组

- 你能调用哪些模型，取决于 API Key 所选的渠道分组，以及 API Key 是否开启了模型限制。可用清单在「仪表盘」的「当前可用模型」中查看，也可以用上面各协议的 models 接口查询。
- Claude / Gemini 协议的请求只能路由到对应类型的渠道。若这两个协议查不到模型，说明当前分组内没有该类型渠道，改用 OpenAI 兼容接口即可。
- 请求失败时可到 [用量与日志](/zh/guide/usage) 页面按 API Key 与时间定位具体那一条记录，明细里会显示结束原因与计费构成。

## 相关

- [推理设置](/zh/guide/reasoning)：控制推理模型的思考行为
- [特殊用法](/zh/guide/special)：非通用场景的调用方式
- [API Key 与额度](/zh/guide/tokens)：额度、有效期、模型限制与 IP 白名单
- [常见问题](/zh/guide/faq)：接入报错排查
