---
title: "打开按线路路由"
layout: doc
outline: deep
lastUpdated: true
---

# 打开按线路路由

线路把每个公开模型按档位分给承接它的渠道，带暂停规则和故障切换。线路功能发布时是关着的：`ModelSupplyEnabled` 打开之前，请求仍按渠道的模型列表和模型映射路由，和以前完全一样，线路相关页面只展示「打开后会怎样」。

先用自己的数据核对迁移会改什么，再打开。下面每一步都用 root 账号操作：登录站点后，在浏览器开发者工具的控制台里发请求，用的就是当前登录会话。

```js
// 后面几步共用的小函数。
const api = (method, path, body) =>
  fetch(path, { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) }).then((r) => r.json());
```

## 1. 备份数据库

先备一份能恢复的数据（SQLite：停掉网关后复制数据库文件；MySQL 或 PostgreSQL：导出一份 dump）。迁移会写入线路、目录模型和价格。

## 2. 试运行

```js
const report = (await api('POST', '/api/model_supply/migrate', { dry_run: true })).data;
```

试运行不写任何数据。继续之前先看报告：

| 字段 | 要核对什么 |
| --- | --- |
| `supplies_planned` | 会建多少条线路，每个渠道的每个模型一条。 |
| `new_models`、`new_aliases`、`status_changes` | 会新增或发布的目录模型。渠道列出的名字一旦发布，客户端就能看到。 |
| `billing_changes`、`price_copies`、`price_conflicts`、`plus_mappings` | 以后按公开模型而不是上游名计价的请求。价格有冲突的先改好。 |
| `unresolved_wildcards`、`skipped_entries`、`duplicate_entries` | 不会变成线路的渠道条目。到渠道上改，或者确认可以舍弃。 |
| `policy_gaps` | 目标模型将没有线路的路由策略。 |
| `routing_differences` | 按用户分组列出访问方式会变的模型，每一条都应该在预期之内。 |

每改一处就再试运行一次，直到报告里只剩你想要的改动。

## 3. 迁移并打开

```js
await api('POST', '/api/model_supply/migrate', { dry_run: false });
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'true' });
```

这次的报告应当和最后一次试运行一致。

## 4. 核对

- 用几个真实的 API key 调 `GET /v1/models`，除了报告里列出的差异，清单应和打开前一样。
- 在模型的「线路」页签里，请求预览会显示下一个请求发往哪个渠道。优先用预览，少发测试请求：用无效密钥发出的请求会触发默认的凭据规则，把渠道暂停。
- 打开后的第一个小时留意供应商页和日志。

## 关回去

```js
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'false' });
```

路由立刻回到渠道的模型列表。线路会保留，再次打开不用重新迁移；这期间渠道有改动的话，还是先试运行一次。
