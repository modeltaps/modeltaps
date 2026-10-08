#!/usr/bin/env bash
# 停掉整套环境。默认保留数据卷与项目配置；带 -v 时连数据卷、var/ 与 out/ 一起清掉。
# 幂等：什么都没起过时也正常返回 0。
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

WIPE=0
for arg in "$@"; do
  case "$arg" in
    -v|--volumes) WIPE=1 ;;
    *) die "未知参数：$arg（只支持 -v/--volumes）" ;;
  esac
done

# modeltaps-up.sh 起的是宿主机进程，不在 compose 里，先收掉。
stop_modeltaps

if (( WIPE )); then
  log "停止容器并删除数据卷"
  dc down -v --remove-orphans
  log "删除 var/ 与 out/"
  rm -rf "$VAR_DIR" "$OUT_DIR"
else
  log "停止容器（保留数据卷与 var/）"
  dc down --remove-orphans
fi

log "完成"
