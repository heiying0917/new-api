#!/bin/bash
# PG 写路径冒烟（发版前必做）— 2026-09-17 事故后新增。
# 起一次性 postgres:15 容器 → 用待发版二进制跑起来 → setup/login → 建渠道(INSERT json 列) →
# 改渠道(UPDATE json 列) → 读回(Scan) → 断言，任何一步失败 exit 1。
#
# 用法: bash .agents/skills/tke-release/scripts/pg-write-smoke.sh <tokenki 二进制> [端口]
#   二进制需为本机可执行(darwin)：  CGO_ENABLED=0 go build -o /tmp/tokenki-smoke .
# 背景: 关闭 PG PrepareStmt 后 pgx 走 simple protocol，[]byte 参数按 bytea 编码，写 json 列报
#       SQLSTATE 22P02(见 docs/report/2026-09-17-pg-json-column-22p02.md)。单测跑 SQLite 抓不到，
#       只有真 PG + 真写入能暴露此类问题。
set -u
BIN="${1:?usage: pg-write-smoke.sh <tokenki-binary> [port]}"; PORT="${2:-5099}"
CTR=tokenki-pg-smoke; PGPORT=55441; DSN="postgres://tk:tk@127.0.0.1:$PGPORT/tk"; B="http://127.0.0.1:$PORT"
J=/tmp/pg-smoke-cookies.txt; LOG=/tmp/pg-smoke-app.log; PW='E2eTest#2026pw'; FAIL=0
say() { printf '  %s\n' "$*"; }
cleanup() { [ -n "${PID:-}" ] && kill "$PID" 2>/dev/null && wait "$PID" 2>/dev/null; docker rm -f $CTR >/dev/null 2>&1; rm -f "$J"; }
trap cleanup EXIT

echo "== PG write smoke: $(basename "$BIN") on :$PORT =="
docker rm -f $CTR >/dev/null 2>&1
docker run -d --name $CTR -e POSTGRES_USER=tk -e POSTGRES_PASSWORD=tk -e POSTGRES_DB=tk -p $PGPORT:5432 postgres:15 >/dev/null || { say "❌ 无法启动 postgres 容器"; exit 1; }
for i in $(seq 1 30); do docker exec $CTR pg_isready -U tk -q 2>/dev/null && break; sleep 1; done

SQL_DSN="$DSN" PORT="$PORT" SESSION_SECRET=pg-smoke-secret "$BIN" > "$LOG" 2>&1 &
PID=$!
for i in $(seq 1 40); do curl -sf -m 2 "$B/api/status" >/dev/null 2>&1 && break; sleep 1; done
curl -sf -m 5 "$B/api/status" >/dev/null || { say "❌ 服务未起来，日志尾部："; tail -5 "$LOG"; exit 1; }
say "✅ 启动 + 迁移完成（真 PG）"

curl -s -m 10 -X POST "$B/api/setup" -H 'Content-Type: application/json' \
  -d "{\"username\":\"root\",\"password\":\"$PW\",\"confirmPassword\":\"$PW\",\"SelfUseModeEnabled\":true}" >/dev/null
LOGIN=$(curl -s -m 10 -c "$J" -X POST "$B/api/user/login" -H 'Content-Type: application/json' -d "{\"username\":\"root\",\"password\":\"$PW\"}")
UID_=$(echo "$LOGIN" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("data",{}).get("id",""))' 2>/dev/null)
[ -n "$UID_" ] || { say "❌ 登录失败: $(echo "$LOGIN" | head -c 160)"; exit 1; }
H=(-b "$J" -H "New-Api-User: $UID_" -H 'Content-Type: application/json')

# INSERT: channels.channel_info 是 json 列
CREATE=$(curl -s -m 15 "${H[@]}" -X POST "$B/api/channel/" -d '{"mode":"single","channel":{"type":14,"name":"pg-smoke","key":"sk-smoke-dummy","base_url":"https://example.services.ai.azure.com/anthropic","models":"claude-opus-5","group":"default","priority":0,"weight":0,"cost_price":2.2}}')
if echo "$CREATE" | grep -q '"success":true'; then say "✅ 建渠道 INSERT(json 列) 成功"; else say "❌ 建渠道失败: $(echo "$CREATE" | head -c 200)"; FAIL=1; fi

CID=$(curl -s -m 10 "${H[@]}" "$B/api/channel/?p=1&page_size=5" | python3 -c 'import sys,json
d=json.load(sys.stdin); items=d.get("data",{}).get("items") if isinstance(d.get("data"),dict) else d.get("data")
items=items or []; print(items[0]["id"] if items else "")' 2>/dev/null)
if [ -n "$CID" ]; then
  # UPDATE: 同一 json 列的更新路径
  printf '{"id":%s,"name":"pg-smoke-renamed","type":14,"models":"claude-opus-5,claude-sonnet-5","group":"default","base_url":"https://example.services.ai.azure.com/anthropic","cost_price":2.5}' "$CID" > /tmp/pg-smoke-update.json
  UPDATE=$(curl -s -m 15 "${H[@]}" -X PUT "$B/api/channel/" -d @/tmp/pg-smoke-update.json)
  if echo "$UPDATE" | grep -q '"success":true'; then say "✅ 改渠道 UPDATE(json 列) 成功"; else say "❌ 改渠道失败: $(echo "$UPDATE" | head -c 200)"; FAIL=1; fi
  # SCAN: 读回并校验
  NAME=$(curl -s -m 10 "${H[@]}" "$B/api/channel/$CID" | python3 -c 'import sys,json; d=json.load(sys.stdin); c=d.get("data",{}); print(c.get("name",""), "|", "channel_info_ok" if isinstance(c.get("channel_info"),dict) else "channel_info_BAD")' 2>/dev/null)
  case "$NAME" in pg-smoke-renamed*channel_info_ok) say "✅ 读回 Scan 正确: $NAME";; *) say "❌ 读回异常: $NAME"; FAIL=1;; esac
else
  say "❌ 渠道列表为空，INSERT 未落库"; FAIL=1
fi

if grep -q '22P02' "$LOG"; then say "❌ 应用日志出现 SQLSTATE 22P02"; FAIL=1; fi
if grep -qi 'panic' "$LOG"; then say "❌ 应用日志出现 panic"; FAIL=1; fi
if [ $FAIL -eq 0 ]; then echo "== ✅ PG write smoke PASS =="; else echo "== ❌ PG write smoke FAIL（日志: $LOG）=="; exit 1; fi
