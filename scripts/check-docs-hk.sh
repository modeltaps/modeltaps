#!/usr/bin/env bash
# Guard the Hong Kong (zh-HK) surfaces against simplified-Chinese characters and
# non-Hong-Kong terminology creeping back in. Covered surfaces:
#   - markdown under docs/hk and docs/ops/hk (fenced code blocks are skipped so
#     commands and payload samples do not trigger false positives)
#   - the value side of web/src/i18n/locales/zh_HK.json and of the repo's own
#     web/src/i18n/locales/modeltaps/zh_HK.json (JSON keys are ignored, they carry
#     upstream naming and never reach the UI)
#
# Usage: scripts/check-docs-hk.sh [target ...]
#   target may be a directory (all *.md below it are scanned) or a *.json file
#   (value side only). Default: docs/hk docs/ops/hk web/src/i18n/locales/zh_HK.json
#
# Allowlist: add "path-substring|term" entries to ALLOWLIST below to let through
# a term that is intentionally kept for semantic reasons (e.g. 資料 meaning
# "material" rather than "data"). Separate multiple entries with ";".
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -gt 0 ]]; then
  TARGETS=("$@")
else
  TARGETS=(
    "$REPO_ROOT/docs/hk"
    "$REPO_ROOT/docs/ops/hk"
    "$REPO_ROOT/web/src/i18n/locales/zh_HK.json"
    "$REPO_ROOT/web/src/i18n/locales/modeltaps/zh_HK.json"
  )
fi

TARGET_DIRS=()
TARGET_FILES=()
for target in "${TARGETS[@]}"; do
  if [[ -d "$target" ]]; then
    TARGET_DIRS+=("$target")
  elif [[ -f "$target" ]]; then
    TARGET_FILES+=("$target")
  else
    echo "check-docs-hk: target not found: $target" >&2
    exit 2
  fi
done

# Simplified-only characters (each differs from its traditional counterpart).
SIMPLIFIED_CHARS="\
们 个 为 这 会 说 讲 让 认 识 语 请 谢 试 设 备 务 员 单 双 图 网 络 线 级 组 结 给 经 纪 约 计 记 讨 论 证 评 软 \
数 据 库 户 类 参 关 联 开 闭 门 问 间 闻 阅 队 际 陈 险 随 隐 现 见 观 视 规 览 觉 时 应 该 详 业 东 车 达 边 \
过 还 进 远 迟 适 选 递 运 连 逻 处 复 团 圆 国 学 举 兴 发 变 欢 权 极 构 标 树 检 验 样 机 积 称 稳 种 断 继 \
维 缓 编 缩 绑 绝 统 纳 输 转 载 较 辑 错 钟 键 镜 银 铁 钱 锁 针 钉 顶 项 顺 须 预 领 频 题 颜 风 飞 马 骤 驱 \
体 众 优 侧 亿 仅 从 仓 价 传 伪 写 决 净 减 汉 济 测 湾 满 滤 汇 灯 烦 热 灭 爱 献 状 独 环 盘 监 确 础 礼 简 \
签 笔 节 范 终 绍 织 综 纯 纸 练 职 脑 获 荐 药 华 万 补 装 讯 许 译 词 话 询 读 课 调 谈 财 账 责 败 货 质 贴 \
费 贸 资 赖 赞 辅 阵 阶 阳 阴 顾 显 严 丽 义 乐 习 乡 书 买 乱 争 亚 产 亲 仪 尽 层 属 岁 岛 币 帮 归 当 张 弹 \
录 彻 总 恶 惊 忆 战 扫 执 扩 扬 担 拟 择 换 损 摄 旧 杂 来 档 桥 楼 欧 沟 洁 浏 润 渐 滚 烧 营 盖 码 竞 绘 绩 \
绿 缆 罗 苏 荣 蓝 虑 虽 谱 贡 轮 轻 违 逊 遗 邮 邻 郑 韵 颗 顿 飘 饭 饰 馆 驳 驻 验 鸟 鸡 鲜 麦 齿 龄 龙"

# Non-Hong-Kong terminology (mainland-traditional or Taiwan wording). Mirrors the
# left column of the zh-HK glossary: 登錄→登入、默認→預設、服務器→伺服器、
# 賬戶/帳號→帳戶、使用者→用戶、網址列→網址欄、緩存→快取、資料庫→數據庫、
# 網路/軟體/硬體/數位→網絡/軟件/硬件/數碼、日志→日誌。
# 設置 / 資料 / 文件 are deliberately absent: they need per-sentence judgement.
NON_HK_TERMS="登錄 默認 服務器 賬戶 帳號 使用者 網址列 緩存 資料庫 網路 軟體 硬體 數位 日志"

# Intentional exceptions: semicolon-separated "path-substring|term" entries,
# e.g. "docs/hk/faq.md|資料;zh_HK.json|文件". Empty today — the term list above
# carries no semantically ambiguous word, so H1/H2 kept nothing that needs one.
ALLOWLIST=""

# while-read instead of mapfile: keeps the script usable on bash 3.2 (macOS).
files=()
if [[ ${#TARGET_DIRS[@]} -gt 0 ]]; then
  while IFS= read -r file; do
    files+=("$file")
  done < <(find "${TARGET_DIRS[@]}" -type f -name '*.md' | sort)
fi
if [[ ${#TARGET_FILES[@]} -gt 0 ]]; then
  files+=("${TARGET_FILES[@]}")
fi

if [[ ${#files[@]} -eq 0 ]]; then
  echo "check-docs-hk: no files found under ${TARGETS[*]}" >&2
  exit 2
fi

hits="$(awk -v simplified="$SIMPLIFIED_CHARS" -v nonhk="$NON_HK_TERMS" -v allow="$ALLOWLIST" '
  BEGIN {
    ns = split(simplified, s, " ")
    nn = split(nonhk, n, " ")
    na = split(allow, a, ";")
  }
  function allowed(file, term,   i, cut, path) {
    for (i = 1; i <= na; i++) {
      if (a[i] == "" || substr(a[i], 1, 1) == "#") continue
      cut = index(a[i], "|")
      if (cut == 0) continue
      path = substr(a[i], 1, cut - 1)
      if (substr(a[i], cut + 1) == term && index(file, path)) return 1
    }
    return 0
  }
  FNR == 1 { fenced = 0; isjson = (FILENAME ~ /\.json$/) }
  !isjson && /^[ \t]*(```|~~~)/ { fenced = !fenced; next }
  !isjson && fenced { next }
  {
    line = $0
    # JSON: drop the key so upstream key naming never counts as a hit.
    if (isjson) sub(/^[ \t]*"([^"\\]|\\.)*"[ \t]*:[ \t]*/, "", line)
    for (i = 1; i <= ns; i++)
      if (index(line, s[i]) && !allowed(FILENAME, s[i]))
        printf "%s:%d: 简体字 %s\n", FILENAME, FNR, s[i]
    for (i = 1; i <= nn; i++)
      if (index(line, n[i]) && !allowed(FILENAME, n[i]))
        printf "%s:%d: 非港式用語 %s\n", FILENAME, FNR, n[i]
  }
' "${files[@]}")"

if [[ -n "$hits" ]]; then
  echo "$hits"
  echo
  echo "check-docs-hk: 发现 $(printf '%s\n' "$hits" | wc -l | tr -d ' ') 处繁中规范问题（简体字或非港式用语）" >&2
  exit 1
fi

echo "check-docs-hk: ${#files[@]} 个文件通过检查（${TARGETS[*]}）"
