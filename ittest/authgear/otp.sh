#!/usr/bin/env bash
# 取某个邮箱 / 手机号最近一次收到的验证码，只打印验证码本身。
#   ./otp.sh agmail1@example.test
#   ./otp.sh +85251000001
# DEV_MODE=true 时 AuthGear 不外发邮件/短信，整条正文写进容器日志，这里从日志取；
# 万一 DEV_MODE 被关掉（邮件真的进了 Mailpit），邮箱一路自动回退到 Mailpit API，
# 调用方无感知。手机号在 DEV_MODE 关闭时无处可取，会直接报错。
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker python3 curl

TARGET="${1:-}"
[[ -n "$TARGET" ]] || die "用法：./otp.sh <email|phone>"
TIMEOUT="${OTP_TIMEOUT:-20}"

# 注意：解析脚本必须用 python3 -c 传进去。写成 `python3 - <<'PY'` 时 heredoc 会占掉
# stdin，管道里的日志就被丢弃了。
PARSE_LOG='
import re, sys
target = sys.argv[1]
code = None
for line in sys.stdin:
    if "suppressed by development mode" not in line and "suppressed in development mode" not in line:
        continue
    m = re.search(r"recipient=(\S+)", line)
    if not m or m.group(1).strip(chr(34)) != target:
        continue
    marker = "body=" + chr(34)
    start = line.find(marker)
    if start < 0:
        continue
    body = line[start + len(marker):]
    end = body.find(chr(34) + " ")
    if end >= 0:
        body = body[:end]
    d = re.search(r"(?<![0-9])([0-9]{4,8})(?![0-9])", body)
    if d:
        code = d.group(1)
if code:
    print(code)
'

PARSE_MAILPIT='
import json, re, sys, urllib.request
target, api = sys.argv[1], sys.argv[2]
data = json.load(sys.stdin)
for msg in data.get("messages") or []:
    if not any(t.get("Address") == target for t in msg.get("To") or []):
        continue
    with urllib.request.urlopen(api + "/api/v1/message/" + msg["ID"]) as r:
        detail = json.load(r)
    text = (detail.get("Text") or "") + (detail.get("HTML") or "")
    m = re.search(r"(?<![0-9])([0-9]{4,8})(?![0-9])", text)
    if m:
        print(m.group(1))
    break
'

from_logs() {
  dc logs --no-log-prefix --tail 2000 authgear 2>/dev/null \
    | python3 -c "$PARSE_LOG" "$TARGET"
}

from_mailpit() {
  [[ "$TARGET" == *@* ]] || return 0
  curl -fsS "${MAILPIT_API}/api/v1/messages?limit=50" 2>/dev/null \
    | python3 -c "$PARSE_MAILPIT" "$TARGET" "$MAILPIT_API" 2>/dev/null
}

for (( i = 0; i < TIMEOUT; i++ )); do
  code="$(from_logs)"
  [[ -n "$code" ]] && { printf '%s\n' "$code"; exit 0; }
  code="$(from_mailpit)"
  [[ -n "$code" ]] && { printf '%s\n' "$code"; exit 0; }
  sleep 1
done

die "没取到 ${TARGET} 的验证码（DEV_MODE=${DEV_MODE}；日志与 Mailpit 均无记录）"
