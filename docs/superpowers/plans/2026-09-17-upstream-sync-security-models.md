# 上游必要修复同步 + 最新模型列表 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把上游 QuantumNous/new-api（rc.11 → rc.37）中本地仍未修的安全漏洞与生产稳定性修复 cherry-pick 进 Tokenki，并把内置模型列表/默认价格更新到最新一代（Claude 5 系、GPT-5.5/5.6/6-astra、Gemini 3.x GA、OpenAI Realtime GA）。

**Architecture:** 不做整体 merge（上游已删 web/classic 并重构鉴权，Tokenki 生产跑 classic）。在新分支 `chore/upstream-sync-2026-09` 上，按"安全 → 稳定性 → 模型"三层，用 `git cherry-pick -x` 逐个落地上游提交；上游文件结构与本地不同的（relaykit、billing_usage.go 等）改为手工移植；模型列表与定价按官方文档手工补齐。每层结束跑 `go build ./... && go test`，与基线对比"零新增失败"。

**Tech Stack:** Go 1.25、GORM、git cherry-pick / merge-tree、`go test`。不涉及前端。

**Out of scope:** Azure Claude 专用渠道类型（已确认 Anthropic Claude 渠道填 `https://<resource>.services.ai.azure.com/anthropic` 即可跑）；上游 057f71c23（日志元数据隔离，25 文件重构，本地 `formatUserLogs` 已剥离 `admin_info`，延后）；web/default 与 web/classic 不改；不合入 main、不 push。

**基线（2026-09-17）:** 分叉点 `6f415428`（2026-06-11），上游领先 500 提交；本地 `go build ./...` 通过；已知陈旧测试见 `docs/known-stale-tests.txt`。

---

## 冲突预演（`git merge-tree --write-tree --merge-base=<c>^ main <c>`，只读）

| 提交 | 内容 | 预演冲突文件 |
|---|---|---|
| d0bd8aac7 | GHSA-8r8v 数量参数校验+饱和转换（新增 common/quota_math.go） | AGENTS.md, relay/common/relay_utils_test.go |
| c9943d37a | GHSA-8r8v 余下路径 | AGENTS.md |
| 621927f71 | 预扣费饱和拒绝 | relay/helper/price.go, price_test.go, service/quota_saturation_test.go |
| dfc0d6324 | GHSA-j6gc 用户设置更新不回写全量快照 | model/user.go |
| 0cd9dc85e | GHSA-j6gc 续：access_token/aff 原子更新 | model/user.go, model/user_update_test.go, router/api-router.go |
| bfddc5fea | 用户查询 Omit access_token | 无 |
| 986d90ae0 | 优雅关闭 | 无 |
| d3874db61 | 内存限流器优化 | 无 |
| b518d0033 | ResponseHeaderTimeout 防 OOM | service/http_client.go |
| 1751f43ee | SQLite WAL/busy timeout | common/database.go |
| 66031a09d | PG 关闭 prepared statements（gorm 1.25.2→1.25.12） | model/main.go, model/gorm_logger*.go（本地无此文件） |
| 9a8674425 | 重启不重复 DDL（sqlite 驱动 1.9.0→1.11.0） | AGENTS.md, model/main.go, model/user_session_migration_test.go（本地无） |
| d6b5ce99d | Request.GetBody 支持 HTTP/2 重试 | relay/alpha_search_handler.go（本地无）, relay/channel/api_request.go, relay/common/relay_info.go(+_test) |
| bd585d78e | Bedrock 客户端断开即取消 | service/billing_usage.go（本地无）, service/text_quota_test.go |
| 4442bb302 | Claude 请求不注入空 tools | 上游在 relaykit，本地手工移植到 relay/channel/claude/relay-claude.go |
| 2f5f6ba84 / 16bfae175 / 8b41defbe | GPT-5.5+ 完成倍率规则 / Realtime GA / gemini-3 image GA | 无 |
| 6ce7305cd | GPT-5.5/5.6 倍率 | setting/ratio_setting/model_ratio.go（上游整文件重排，改为手工加 4 行） |

冲突处理总原则：**AGENTS.md 一律保留本地版本**（Tokenki 规则文件，上游新增的规则段若与本地无关则丢弃）；本地不存在的文件直接 `git rm`/跳过；测试文件优先取上游版本再按本地符号修正。

---

### Task 0: 建分支、记录基线

**Files:** 无代码改动

- [ ] **Step 1: 确认工作区干净并建分支**

```bash
cd /Users/xuyang/workspace/newapi-juhe
git status --short   # 预期仅 docs/superpowers/WORKLOG.md(M) 与 docs/deploy/report/2026-06-24-*.md(??)
git checkout -b chore/upstream-sync-2026-09
```

- [ ] **Step 2: 跑基线测试并保存结果**

```bash
go build ./... && go vet ./... 2>&1 | tail -5
go test ./... 2>&1 | grep -E '^(FAIL|ok|---)' | grep -vE '^ok' > /tmp/baseline-test.txt; cat /tmp/baseline-test.txt
```
Expected: 仅 `docs/known-stale-tests.txt` 中列出的失败（如 `TestListModelsTokenLimitIncludesTieredBillingModel`、claude `TestRequestOpenAI2ClaudeMessage_*`）。

---

### Task 1: 安全 — GHSA-8r8v 计费整数溢出（3 个 cherry-pick）

**Files:**
- 新增（来自上游）: `common/quota_math.go`, `common/quota_math_test.go`
- 修改: `relay/helper/price.go`, `service/billing.go`, `pkg/billingexpr/round.go`, `relay/common/relay_info.go`, 多个 relay handler
- 测试: `relay/helper/price_test.go`, `service/quota_saturation_test.go`, `relay/common/relay_utils_test.go`

- [ ] **Step 1: cherry-pick d0bd8aac7 并解决冲突**

```bash
git cherry-pick -x d0bd8aac7
git status --short | grep '^UU\|^AA\|^DU\|^UD'
```
冲突处理：
- `AGENTS.md`：`git checkout --ours AGENTS.md`（保留本地）。
- `relay/common/relay_utils_test.go`：打开文件，保留本地已有用例，并把上游新增的 `TestTaskDurationBounds` 整段追加到文件末尾（删除 `<<<<<<<`/`=======`/`>>>>>>>` 标记）；若本地文件缺少 `httptest`/`strings`/`gin` import 则补上。
```bash
git add AGENTS.md relay/common/relay_utils_test.go && GIT_EDITOR=true git cherry-pick --continue
```

- [ ] **Step 2: 编译验证**

```bash
go build ./... && go test ./common/ ./relay/common/ -count=1 2>&1 | tail -5
```
Expected: `ok`。

- [ ] **Step 3: cherry-pick c9943d37a**

```bash
git cherry-pick -x c9943d37a
git checkout --ours AGENTS.md && git add AGENTS.md && GIT_EDITOR=true git cherry-pick --continue
go build ./... 2>&1 | tail -3
```

- [ ] **Step 4: cherry-pick 621927f71 并解决冲突**

```bash
git cherry-pick -x 621927f71
```
冲突处理（三个文件都以上游版本为基础）：
- `relay/helper/price.go`：确保四处 `common.QuotaFromFloat(...)` 改为 `common.QuotaFromFloatChecked(...)` + `clamp != nil` 时 `return types.PriceData{}, preConsumeQuotaRangeError(...)`；tiered 分支用 `billingexpr.QuotaRoundChecked`。新增 `preConsumeQuotaRangeError` 函数。本地若有 Tokenki 自加的字段/逻辑（供应商成本价等）保留。
- `relay/helper/price_test.go` 与 `service/quota_saturation_test.go`：取上游版本 `git checkout --theirs <file>`，再 `go vet ./relay/helper/ ./service/` 看是否引用了本地不存在的符号，缺什么按本地名字改。
```bash
git add -A relay/helper service pkg && GIT_EDITOR=true git cherry-pick --continue
```

- [ ] **Step 5: 跑相关测试**

```bash
go build ./... && go test ./common/ ./relay/helper/ ./service/ ./relay/common/ ./pkg/billingexpr/ -count=1 2>&1 | grep -E '^(ok|FAIL|---)'
```
Expected: 全 `ok`；若 `service` 包有失败，对比 `/tmp/baseline-test.txt` 确认是否预先存在。

- [ ] **Step 6: 回归验证漏洞已堵**

```bash
go test ./relay/helper/ -run 'Saturat|Overflow|Quantity' -v -count=1 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)'
```
Expected: 上游带来的饱和/数量校验用例 PASS。

---

### Task 2: 安全 — GHSA-j6gc PUT /api/user/self 覆盖 Redis 额度缓存（3 个 cherry-pick）

**Files:**
- 修改: `controller/user.go`, `model/user.go`, `model/user_cache.go`, `controller/subscription.go`, `router/api-router.go`
- 测试: `model/user_update_test.go`（上游新增）

- [ ] **Step 1: cherry-pick dfc0d6324**

```bash
git cherry-pick -x dfc0d6324
```
冲突处理 `model/user.go`（本地有供应商/防提权改动，必须手工合）：
1. 新增函数（放在 `SetSetting` 之后）：
```go
func UpdateUserSetting(userId int, setting dto.UserSetting) error {
	if userId == 0 {
		return errors.New("id 为空！")
	}
	settingBytes, err := common.Marshal(setting)
	if err != nil {
		return err
	}
	settingValue := string(settingBytes)
	if err = DB.Model(&User{}).Where("id = ?", userId).Update("setting", settingValue).Error; err != nil {
		return err
	}
	return updateUserSettingCache(userId, settingValue)
}
```
2. `UpdateWithTx`：把 `tx.First(&user, user.Id); tx.Model(user).Updates(newUser)` 改为
```go
	current := User{}
	if err = tx.First(&current, user.Id).Error; err != nil {
		return err
	}
	if err = tx.Model(&current).Omit("quota", "used_quota", "request_count").Updates(newUser).Error; err != nil {
		return err
	}
	return tx.First(user, user.Id).Error
```
3. `EditWithTx` 同样改为 `current := User{}` 模式（不加 Omit）。
4. 文件内 `json.Marshal/Unmarshal` 若仍是 `encoding/json` 直调，改为 `common.Marshal/Unmarshal`（Rule 1）。
`model/user_cache.go` 若冲突：取上游版本（`updateUserCache` 改名 `populateUserCache`，新增 `updateUserSettingCache`），再全局 `grep -rn 'updateUserCache(' model/` 把本地残留调用改名。
```bash
git add -A controller model && GIT_EDITOR=true git cherry-pick --continue
go build ./... 2>&1 | tail -3
```

- [ ] **Step 2: cherry-pick 0cd9dc85e**

```bash
git cherry-pick -x 0cd9dc85e
```
冲突处理：
- `model/user.go`：新增 `UpdateUserAccessToken(id int, token string) error`；`inviteUser` 改为 `gorm.Expr("aff_count + ?", 1)` 三字段原子更新；`UpdateWithTx` 的 Omit 列表扩为 `"access_token","quota","used_quota","request_count","aff_count","aff_quota","aff_history"`（本地无 `auth_version` 字段则不要写它）。
- `router/api-router.go`：仅把 `selfRoute.GET("/token", ...)` 加上 `middleware.CriticalRateLimit()`；本地无 `middleware.DisableCache()` 则不加。
- `model/user_update_test.go`：`git checkout --theirs`，然后 `go vet ./model/`，把引用本地不存在的字段（如 `AuthVersion`）的断言删掉。
```bash
git add -A model router controller && GIT_EDITOR=true git cherry-pick --continue
```

- [ ] **Step 3: cherry-pick bfddc5fea（预演无冲突）**

```bash
git cherry-pick -x bfddc5fea && go build ./... 2>&1 | tail -3
```

- [ ] **Step 4: 测试**

```bash
go test ./model/ ./controller/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
go test ./model/ -run 'UserUpdate|UserSetting|AccessToken|Invite' -v -count=1 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: 除基线已知失败外全过；新用例 PASS。

- [ ] **Step 5: 人工核对漏洞路径**

```bash
grep -n 'UpdateUserSetting\|user.Update(false)' controller/user.go | head
```
Expected: `UpdateSelf` 中语言/侧边栏两处已是 `model.UpdateUserSetting(user.Id, currentSetting)`，不再 `user.Update(false)`。

---

### Task 3: 稳定性 — 无冲突的两个提交

**Files:** `main.go`, `middleware/rate-limit.go`, `common/*`（由上游提交决定）

- [ ] **Step 1: cherry-pick 并编译**

```bash
git cherry-pick -x 986d90ae0 && git cherry-pick -x d3874db61
go build ./... && go test ./middleware/ ./common/ -count=1 2>&1 | grep -E '^(ok|FAIL)'
```
Expected: 全 `ok`。若 986d90ae0 意外冲突于 `main.go`（本地 NDJSON logger 改动），保留本地 logger 初始化，同时加入上游的 `signal.NotifyContext` + `srv.Shutdown` 优雅关闭段。

---

### Task 4: 稳定性 — ResponseHeaderTimeout 与 SQLite WAL

**Files:** `common/constants.go`, `common/init.go`, `service/http_client.go`, `.env.example`, `README.md`, `common/database.go`

- [ ] **Step 1: cherry-pick b518d0033**

```bash
git cherry-pick -x b518d0033
```
冲突处理 `service/http_client.go`：在 `InitHttpClient()` 里 `transport := &http.Transport{...}` 之后加入上游的
```go
	if seconds := common.RelayResponseHeaderTimeout; seconds > 0 {
		if seconds > maxTimeoutSeconds {
			seconds = maxTimeoutSeconds
		}
		transport.ResponseHeaderTimeout = time.Duration(seconds) * time.Second
	}
```
并在文件顶部加 `const maxTimeoutSeconds = int(math.MaxInt64 / int64(time.Second))` 与 `"math"` import。本地 `NewProxyHttpClient` 内两处 `transport := &http.Transport{` 也各补同一段（代理客户端同样需要上限）。`README.md` 冲突保留本地版本（Tokenki README）。
```bash
git add -A && GIT_EDITOR=true git cherry-pick --continue
```

- [ ] **Step 2: cherry-pick 1751f43ee**

```bash
git cherry-pick -x 1751f43ee
```
冲突处理 `common/database.go`：保留本地内容，加入上游对 SQLite DSN 的 `_pragma=journal_mode(WAL)&_pragma=busy_timeout(...)&_txlock=immediate` 拼接。
```bash
git add -A && GIT_EDITOR=true git cherry-pick --continue
go build ./... && go test ./common/ ./service/ -count=1 2>&1 | grep -E '^(ok|FAIL)'
```

---

### Task 5: 稳定性 — PostgreSQL prepared statements 与重复迁移

**Files:** `go.mod`, `go.sum`, `model/main.go`, 新增 `model/migration_dialector.go`(+_test)

- [ ] **Step 1: cherry-pick 66031a09d**

```bash
git cherry-pick -x 66031a09d
git status --short | grep -E '^(UU|DU|UD|AA)'
```
冲突处理：
- `model/gorm_logger.go` / `model/gorm_logger_test.go`：本地不存在 → `git rm --cached` 上游新增或 `git checkout --ours` 后删除，即**跳过**（本地 GORM logger 在 `logger/` 包，NDJSON 版；上游那 5 行是针对其自有 logger 的）。
- `model/main.go`：在 PostgreSQL 打开处，把 `postgres.Open(dsn)` 改为
```go
	postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
```
- `go.mod`：接受上游 `gorm.io/gorm v1.25.12`；随后 `go mod tidy`。
```bash
go mod tidy && git add -A && GIT_EDITOR=true git cherry-pick --continue
go build ./... 2>&1 | tail -3
```

- [ ] **Step 2: cherry-pick 9a8674425**

```bash
git cherry-pick -x 9a8674425
```
冲突处理：
- `AGENTS.md`：`git checkout --ours AGENTS.md`。
- `model/user_session_migration_test.go`：本地不存在 → 跳过。
- `model/main.go`：接受上游新增的 `migrationDialector`/`newMigrationDialector` 包装（新文件 `model/migration_dialector.go` 直接进来），把 `DB.AutoMigrate(...)` 之前的 dialector 包装按上游写法接上；保留本地 Tokenki 自建表（supplier/settlement 等）的 AutoMigrate 列表。
- `go.mod`：接受 `github.com/glebarez/sqlite v1.11.0`；`go mod tidy`。
```bash
go mod tidy && git add -A && GIT_EDITOR=true git cherry-pick --continue
go build ./... && go test ./model/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
```
Expected: `model/migration_dialector_test.go` PASS；其余与基线一致。

- [ ] **Step 3: 真库启动验证（PG）**

```bash
docker ps --format '{{.Names}} {{.Ports}}' | grep -i postgres
go build -o /tmp/tokenki-sync . && \
SQL_DSN='postgres://root:123456@127.0.0.1:5432/new-api' PORT=5002 /tmp/tokenki-sync > /tmp/tokenki-sync.log 2>&1 &
sleep 8; curl -fsS http://127.0.0.1:5002/api/status | head -c 200; echo; \
grep -iE 'error|panic|migrat' /tmp/tokenki-sync.log | head -20; kill %1
```
Expected: `/api/status` 返回 `success:true`；日志无 `panic`、无迁移错误。（PG 端口以 `docker ps` 实际映射为准；库名 `new-api`，用户 `root`，密码 `123456`。）

---

### Task 6: 稳定性 — Request.GetBody（HTTP/2 上游 stream reset 透明重试）

**Files:** `common/body_storage.go`, `relay/channel/api_request.go`, `relay/common/outbound_body.go`, `relay/common/relay_info.go`, 各 `relay/*_handler.go`, 测试文件

- [ ] **Step 1: cherry-pick d6b5ce99d**

```bash
git cherry-pick -x d6b5ce99d
git status --short | grep -E '^(UU|DU|UD|AA)'
```
冲突处理：
- `relay/alpha_search_handler.go`：本地不存在 → `git rm` 跳过。
- `relay/channel/api_request.go`：以本地为基础，加入上游在构造 `*http.Request` 后设置 `req.GetBody` 的段（用 `common.BodyStorage` 的可重读接口）。
- `relay/common/relay_info.go`(+_test)：加入上游新增的字段/方法（用于缓存 outbound body 以支持 GetBody），保留本地 Tokenki 字段。
- 若本地缺少 `relay/chat_completions_via_responses.go` 等文件同样跳过。
```bash
git add -A && GIT_EDITOR=true git cherry-pick --continue
go build ./... && go test ./relay/... ./common/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
```
Expected: `relay/channel` 新增 `api_request_getbody_test.go` PASS；其余与基线一致。若冲突面过大（>1 小时无法收敛），改为只移植 `api_request.go` 中"为 `*bytes.Reader` 类型 body 设置 `GetBody`"的最小实现并写一条测试，记录在 WORKLOG。

---

### Task 7: 稳定性 — Bedrock 客户端断开即取消

**Files:** `relay/channel/aws/relay-aws.go`, `relay/channel/aws/relay_aws_test.go`

- [ ] **Step 1: cherry-pick bd585d78e**

```bash
git cherry-pick -x bd585d78e
```
冲突处理：
- `service/billing_usage.go`、`service/text_quota_test.go`：本地不存在/不相关 → 丢弃这两处（`git checkout --ours service/text_quota_test.go`；`billing_usage.go` 跳过）。
- `relay/channel/aws/relay-aws.go`：本地有多区域/Global 改动，手工合：`newAwsInvokeContext(parent context.Context)` 用 `c.Request.Context()` 作父 ctx；新增 `newAwsInvokeError`（客户端已断开时带 `types.ErrOptionWithSkipRetry()`）；流式循环改为 `select { case <-requestContext.Done(): ...; case event, ok := <-events: ... }`。
- `relay_aws_test.go`：取上游，`go vet ./relay/channel/aws/` 后修正本地符号差异。
```bash
git add -A && GIT_EDITOR=true git cherry-pick --continue
go test ./relay/channel/aws/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
```

---

### Task 8: 手工移植 4442bb302 — Claude 请求不注入空 tools（TDD）

**Files:**
- Modify: `relay/channel/claude/relay-claude.go:124-129`
- Test: `relay/channel/claude/relay_claude_test.go`

- [ ] **Step 1: 写失败测试**

在 `relay/channel/claude/relay_claude_test.go` 末尾追加：
```go
func TestRequestOpenAI2ClaudeMessage_NoToolsOmitsToolsField(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:    "claude-sonnet-4-6",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &relaycommon.RelayInfo{}
	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, info, req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if claudeReq.Tools != nil {
		t.Fatalf("expected Tools to be nil when no tools provided, got %#v", claudeReq.Tools)
	}
}
```
（函数名/签名以文件内现有 `TestRequestOpenAI2ClaudeMessage_*` 用例为准，照抄它们构造 `info` 与调用方式。）

- [ ] **Step 2: 运行确认失败**

```bash
go test ./relay/channel/claude/ -run NoToolsOmitsToolsField -count=1
```
Expected: FAIL（`Tools` 是空切片 `[]any{}` 而非 nil）。

- [ ] **Step 3: 最小实现**

`relay/channel/claude/relay-claude.go` 中把
```go
		Temperature:   textRequest.Temperature,
		Tools:         claudeTools,
	}
```
改为
```go
		Temperature:   textRequest.Temperature,
	}
	if len(claudeTools) > 0 {
		claudeRequest.Tools = claudeTools
	}
```

- [ ] **Step 4: 测试通过并提交**

```bash
go test ./relay/channel/claude/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
git add relay/channel/claude && git commit -m "fix(relay): stop injecting empty tools into Claude requests

Ported from upstream 4442bb302 (relaykit) to relay/channel/claude."
```
Expected: 新用例 PASS；`TestRequestOpenAI2ClaudeMessage_*` 中基线已知失败不变。

---

### Task 9: 最新模型 — 上游无冲突提交 + GPT-5.5/5.6/6-astra 手工定价

**Files:**
- cherry-pick: 2f5f6ba84（`setting/ratio_setting/model_ratio.go` 完成倍率规则）, 16bfae175（Realtime GA）, 8b41defbe（gemini-3 image GA）
- Modify: `setting/ratio_setting/model_ratio.go`, `setting/ratio_setting/cache_ratio.go`, `relay/channel/openai/constant.go`, `relay/channel/codex/constants.go`

- [ ] **Step 1: cherry-pick 三个无冲突提交**

```bash
git cherry-pick -x 2f5f6ba84 && git cherry-pick -x 16bfae175 && git cherry-pick -x 8b41defbe
go build ./... && go test ./setting/... -count=1 2>&1 | grep -E '^(ok|FAIL)'
```

- [ ] **Step 2: 写失败测试（定价存在性）**

新建 `setting/ratio_setting/latest_models_test.go`：
```go
package ratio_setting

import "testing"

func TestLatestModelsHaveDefaultPricing(t *testing.T) {
	cases := []struct {
		model      string
		ratio      float64
		completion float64
		cacheRead  float64
	}{
		{"gpt-5.5", 2.5, 8, 0.1},
		{"gpt-5.6-sol", 2.5, 8, 0.1},
		{"gpt-5.6-terra", 1.25, 8, 0.1},
		{"gpt-5.6-luna", 0.5, 8, 0.1},
		{"gpt-6-astra", 5, 5, 0.1},
		{"claude-opus-5", 2.5, 5, 0.1},
		{"claude-opus-5-high", 2.5, 5, 0.1},
		{"claude-sonnet-5", 1, 5, 0.1},
		{"claude-sonnet-4-6", 1.5, 5, 0.1},
		{"claude-fable-5-1", 5, 5, 0.025},
		{"claude-fable-5", 5, 5, 0.1},
		{"claude-haiku-4-5", 0.5, 5, 0.1},
	}
	for _, c := range cases {
		ratio, ok := GetModelRatio(c.model)
		if !ok || ratio != c.ratio {
			t.Errorf("%s: model ratio = %v (ok=%v), want %v", c.model, ratio, ok, c.ratio)
		}
		if got := GetCompletionRatio(c.model); got != c.completion {
			t.Errorf("%s: completion ratio = %v, want %v", c.model, got, c.completion)
		}
		if got, _ := GetCacheRatio(c.model); got != c.cacheRead {
			t.Errorf("%s: cache ratio = %v, want %v", c.model, got, c.cacheRead)
		}
		if got, _ := GetCreateCacheRatio(c.model); got != 1.25 {
			t.Errorf("%s: create cache ratio = %v, want 1.25", c.model, got)
		}
	}
}
```
（`GetModelRatio`/`GetCompletionRatio` 的实际签名以 `model_ratio.go` 为准，若返回值不同按实际改。）

- [ ] **Step 3: 运行确认失败**

```bash
go test ./setting/ratio_setting/ -run TestLatestModelsHaveDefaultPricing -count=1
```
Expected: FAIL，列出缺失模型。

- [ ] **Step 4: 补定价**

`setting/ratio_setting/model_ratio.go` `defaultModelRatio` 加入（$/1M 输入 ÷ 2 = ratio）：
```go
	"gpt-5.5":                                   2.5, // $5 / 1M tokens
	"gpt-5.6-sol":                               2.5, // $5 / 1M tokens
	"gpt-5.6-terra":                             1.25,
	"gpt-5.6-luna":                              0.5,
	"gpt-6-astra":                               5, // $10 / 1M tokens (standard tier; long-context >272K 需管理员用表达式定价)
	"claude-haiku-4-5":                          0.5, // $1 / 1M tokens (alias)
	"claude-sonnet-4-6":                         1.5, // $3 / 1M tokens
	"claude-sonnet-4-6-max":                     1.5,
	"claude-sonnet-4-6-high":                    1.5,
	"claude-sonnet-4-6-medium":                  1.5,
	"claude-sonnet-4-6-low":                     1.5,
	"claude-sonnet-5":                           1, // $2 / 1M tokens
	"claude-sonnet-5-max":                       1,
	"claude-sonnet-5-xhigh":                     1,
	"claude-sonnet-5-high":                      1,
	"claude-sonnet-5-medium":                    1,
	"claude-sonnet-5-low":                       1,
	"claude-sonnet-5-thinking":                  1,
	"claude-opus-5":                             2.5, // $5 / 1M tokens
	"claude-opus-5-max":                         2.5,
	"claude-opus-5-xhigh":                       2.5,
	"claude-opus-5-high":                        2.5,
	"claude-opus-5-medium":                      2.5,
	"claude-opus-5-low":                         2.5,
	"claude-opus-5-thinking":                    2.5,
	"claude-fable-5":                            5, // $10 / 1M tokens
	"claude-fable-5-xhigh":                      5,
	"claude-fable-5-high":                       5,
	"claude-fable-5-medium":                     5,
	"claude-fable-5-low":                        5,
	"claude-fable-5-1":                          5, // $10 / 1M tokens
	"claude-fable-5-1-xhigh":                    5,
	"claude-fable-5-1-high":                     5,
	"claude-fable-5-1-medium":                   5,
	"claude-fable-5-1-low":                      5,
	"claude-mythos-5":                           5,
	"claude-mythos-5-1":                         5,
```
完成倍率规则（`model_ratio.go` 约 552 行）改为：
```go
	if strings.Contains(name, "claude-3") {
		return 5, true
	} else if strings.Contains(name, "claude-sonnet-") || strings.Contains(name, "claude-opus-") ||
		strings.Contains(name, "claude-haiku-4") || strings.Contains(name, "claude-fable-") ||
		strings.Contains(name, "claude-mythos-") {
		return 5, true
	}
	if strings.HasPrefix(name, "gpt-6") {
		return 5, true // gpt-6-astra: $10 in / $50 out
	}
```
（GPT-5.5+ 的 8 倍已由 2f5f6ba84 的 `!strings.Contains(name, ".")` 规则覆盖 —— 先跑测试确认，若 `gpt-5.6-*` 返回不是 8，则在 gpt-5 分支补 `strings.HasPrefix(name, "gpt-5.5") || strings.HasPrefix(name, "gpt-5.6")` → 8。）

`setting/ratio_setting/cache_ratio.go`：`defaultCacheRatio` 对上面每个 claude 新模型加 `0.1`（`claude-fable-5-1*` 与 `claude-mythos-5-1` 为 `0.025`），`gpt-5.5`/`gpt-5.6-*`/`gpt-6-astra` 加 `0.1`；`defaultCreateCacheRatio` 对全部新模型加 `1.25`。

- [ ] **Step 5: 补内置模型列表**

`relay/channel/openai/constant.go` 在 `"gpt-5.4", "gpt-5.4-2026-03-05",` 后加一行：
```go
	"gpt-5.5", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-6-astra",
```
`relay/channel/codex/constants.go` `baseModelList` 末尾加：
```go
	"gpt-5.4-mini", "gpt-5.5", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna",
```

- [ ] **Step 6: 测试通过并提交**

```bash
go test ./setting/ratio_setting/ ./relay/channel/openai/ ./relay/channel/codex/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
git add setting relay/channel/openai relay/channel/codex && git commit -m "feat(models): add GPT-5.5/5.6/6-astra and Claude 5 family default pricing and model lists"
```

---

### Task 10: 最新模型 — Claude 5 系能力适配（effort/thinking 白名单泛化，TDD）

**Files:**
- Create: `setting/reasoning/claude.go`, `setting/reasoning/claude_test.go`
- Modify: `relay/channel/claude/relay-claude.go:156-190`, `relay/claude_handler.go:55-90`, `relay/channel/claude/constants.go`, `relay/channel/aws/constants.go`, `relay/channel/vertex/adaptor.go:33-49`
- Test: `relay/channel/claude/relay_claude_test.go`

- [ ] **Step 1: 写能力表失败测试**

新建 `setting/reasoning/claude_test.go`：
```go
package reasoning

import "testing"

func TestClaudeCapabilitiesFor(t *testing.T) {
	cases := []struct {
		model                  string
		adaptive, manual       bool
		strictSampling         bool
		supportsXHigh, supportsMax bool
	}{
		{"claude-fable-5-1", true, false, true, true, false},
		{"claude-fable-5", true, false, true, true, false},
		{"claude-mythos-5-1", true, false, true, true, false},
		{"claude-opus-5", true, false, true, true, true},
		{"claude-sonnet-5", true, false, true, true, true},
		{"claude-opus-4-8", true, false, true, true, true},
		{"claude-opus-4-7", true, false, true, true, true},
		{"claude-opus-4-6", true, true, false, false, true},
		{"claude-sonnet-4-6", true, true, false, false, true},
		{"claude-sonnet-4-5-20250929", false, true, false, false, false},
		{"claude-haiku-4-5", false, true, false, false, false},
	}
	for _, c := range cases {
		got := ClaudeCapabilitiesFor(c.model)
		if got.Adaptive != c.adaptive || got.ManualThinking != c.manual || got.StrictSampling != c.strictSampling ||
			got.SupportsXHigh != c.supportsXHigh || got.SupportsMax != c.supportsMax {
			t.Errorf("%s: got %+v", c.model, got)
		}
	}
}

func TestNormalizeClaudeEffort(t *testing.T) {
	if got := NormalizeClaudeEffort("claude-fable-5-1", "max"); got != "xhigh" {
		t.Errorf("fable max -> %q, want xhigh", got)
	}
	if got := NormalizeClaudeEffort("claude-opus-4-6", "xhigh"); got != "max" {
		t.Errorf("opus-4-6 xhigh -> %q, want max", got)
	}
	if got := NormalizeClaudeEffort("claude-opus-5", "max"); got != "max" {
		t.Errorf("opus-5 max -> %q, want max", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./setting/reasoning/ -count=1 2>&1 | tail -3
```
Expected: 编译失败 `undefined: ClaudeCapabilitiesFor`。

- [ ] **Step 3: 实现能力表**

新建 `setting/reasoning/claude.go`：
```go
package reasoning

import "strings"

// ClaudeCapabilities describes which thinking/effort controls a Claude model
// accepts. Mirrors upstream relaykit/relayconvert/reasoning/claude.go.
type ClaudeCapabilities struct {
	Adaptive       bool // accepts thinking.type="adaptive" + output_config.effort
	ManualThinking bool // accepts thinking.type="enabled" + budget_tokens
	StrictSampling bool // rejects non-default temperature/top_p/top_k with 400
	SupportsXHigh  bool
	SupportsMax    bool
}

func ClaudeCapabilitiesFor(model string) ClaudeCapabilities {
	model = strings.ToLower(model)
	caps := ClaudeCapabilities{ManualThinking: true}
	switch {
	case strings.HasPrefix(model, "claude-fable-5"),
		strings.HasPrefix(model, "claude-mythos-5"):
		caps.Adaptive, caps.ManualThinking, caps.StrictSampling = true, false, true
		caps.SupportsXHigh, caps.SupportsMax = true, false
	case strings.HasPrefix(model, "claude-mythos-preview"):
		caps.Adaptive, caps.StrictSampling, caps.SupportsMax = true, true, true
	case strings.HasPrefix(model, "claude-opus-5"),
		strings.HasPrefix(model, "claude-sonnet-5"),
		strings.HasPrefix(model, "claude-opus-4-8"),
		strings.HasPrefix(model, "claude-opus-4-7"):
		caps.Adaptive, caps.ManualThinking, caps.StrictSampling = true, false, true
		caps.SupportsXHigh, caps.SupportsMax = true, true
	case strings.HasPrefix(model, "claude-opus-4-6"),
		strings.HasPrefix(model, "claude-sonnet-4-6"):
		caps.Adaptive, caps.SupportsMax = true, true
	}
	return caps
}

// NormalizeClaudeEffort maps an effort suffix onto a level the model accepts:
// xhigh and max are equivalent, so swap when only one of them is supported.
func NormalizeClaudeEffort(model, effort string) string {
	caps := ClaudeCapabilitiesFor(model)
	switch effort {
	case "max":
		if !caps.SupportsMax && caps.SupportsXHigh {
			return "xhigh"
		}
	case "xhigh":
		if !caps.SupportsXHigh && caps.SupportsMax {
			return "max"
		}
	}
	return effort
}
```

- [ ] **Step 4: 测试通过**

```bash
go test ./setting/reasoning/ -count=1 2>&1 | tail -3
```
Expected: `ok`。

- [ ] **Step 5: 写转换层失败测试**

在 `relay/channel/claude/relay_claude_test.go` 追加（构造方式照抄文件内现有 `TestRequestOpenAI2ClaudeMessage_*`）：
```go
func TestRequestOpenAI2ClaudeMessage_Claude5EffortSuffix(t *testing.T) {
	temp := 0.3
	req := dto.GeneralOpenAIRequest{
		Model:       "claude-opus-5-high",
		Temperature: &temp,
		Messages:    []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &relaycommon.RelayInfo{}
	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, info, req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if claudeReq.Model != "claude-opus-5" {
		t.Fatalf("model = %q, want claude-opus-5", claudeReq.Model)
	}
	if claudeReq.Thinking == nil || claudeReq.Thinking.Type != "adaptive" {
		t.Fatalf("thinking = %+v, want adaptive", claudeReq.Thinking)
	}
	if string(claudeReq.OutputConfig) != `{"effort":"high"}` {
		t.Fatalf("output_config = %s", claudeReq.OutputConfig)
	}
	if claudeReq.Temperature != nil {
		t.Fatalf("temperature should be cleared for strict-sampling model, got %v", *claudeReq.Temperature)
	}
}

func TestRequestOpenAI2ClaudeMessage_Claude5ThinkingSuffixUsesAdaptive(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:    "claude-sonnet-5-thinking",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &relaycommon.RelayInfo{}
	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, info, req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if claudeReq.Model != "claude-sonnet-5" {
		t.Fatalf("model = %q", claudeReq.Model)
	}
	if claudeReq.Thinking == nil || claudeReq.Thinking.Type != "adaptive" {
		t.Fatalf("thinking = %+v, want adaptive (sonnet-5 rejects type=enabled)", claudeReq.Thinking)
	}
}

func TestRequestOpenAI2ClaudeMessage_FableMaxBecomesXHigh(t *testing.T) {
	req := dto.GeneralOpenAIRequest{
		Model:    "claude-fable-5-1-max",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &relaycommon.RelayInfo{}
	claudeReq, err := RequestOpenAI2ClaudeMessage(nil, info, req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if string(claudeReq.OutputConfig) != `{"effort":"xhigh"}` {
		t.Fatalf("output_config = %s, want xhigh", claudeReq.OutputConfig)
	}
}
```

- [ ] **Step 6: 运行确认失败**

```bash
go test ./relay/channel/claude/ -run 'Claude5|FableMax' -count=1 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: 3 个 FAIL（现在只认 opus-4-6/4-7/4-8 前缀）。

- [ ] **Step 7: 重写 relay-claude.go 的白名单**

`relay/channel/claude/relay-claude.go` 156-190 行改为：
```go
	caps := reasoning.ClaudeCapabilitiesFor(textRequest.Model)
	if baseModel, effortLevel, ok := reasoning.TrimEffortSuffix(textRequest.Model); ok && effortLevel != "" && caps.Adaptive {
		claudeRequest.Model = baseModel
		claudeRequest.Thinking = &dto.Thinking{Type: "adaptive"}
		effortLevel = reasoning.NormalizeClaudeEffort(baseModel, effortLevel)
		claudeRequest.OutputConfig = json.RawMessage(fmt.Sprintf(`{"effort":"%s"}`, effortLevel))
		if caps.StrictSampling {
			// Opus 4.7+ / Claude 5 reject non-default temperature/top_p/top_k with 400
			// and default display to "omitted"; keep the visible summary.
			claudeRequest.Thinking.Display = "summarized"
			claudeRequest.Temperature = nil
			claudeRequest.TopP = nil
			claudeRequest.TopK = nil
		} else {
			claudeRequest.TopP = nil
			claudeRequest.Temperature = common.GetPointer[float64](1.0)
		}
	} else if model_setting.GetClaudeSettings().ThinkingAdapterEnabled &&
		strings.HasSuffix(textRequest.Model, "-thinking") {
		trimmedModel := strings.TrimSuffix(textRequest.Model, "-thinking")
		if !reasoning.ClaudeCapabilitiesFor(trimmedModel).ManualThinking {
			// These models reject thinking.type="enabled"; use adaptive at high effort.
			claudeRequest.Thinking = &dto.Thinking{Type: "adaptive", Display: "summarized"}
			claudeRequest.OutputConfig = json.RawMessage(`{"effort":"high"}`)
			claudeRequest.Temperature = nil
			claudeRequest.TopP = nil
			claudeRequest.TopK = nil
		} else {
			// ...原有 budget_tokens 分支保持不变...
```
（把原来三处 `strings.HasPrefix(..., "claude-opus-4-7") || strings.HasPrefix(..., "claude-opus-4-8")` 全部替换为上面的能力判断；`-thinking` 分支里原本设置 `claudeRequest.Model = trimmedModel` 的位置保持不动。）

- [ ] **Step 8: 同步改 relay/claude_handler.go**

`relay/claude_handler.go` 55-90 行做同样替换（原生 `/v1/messages` 入口）：`caps := reasoning.ClaudeCapabilitiesFor(request.Model)`；effort 分支条件 `&& caps.Adaptive`；`NormalizeClaudeEffort`；StrictSampling 时清 temperature/top_p/top_k；`-thinking` 分支用 `!reasoning.ClaudeCapabilitiesFor(baseModel).ManualThinking`。

- [ ] **Step 9: 补三处模型映射/列表**

`relay/channel/claude/constants.go` `ModelList` 末尾追加：
```go
	"claude-haiku-4-5",
	"claude-sonnet-4-6-max",
	"claude-sonnet-4-6-high",
	"claude-sonnet-4-6-medium",
	"claude-sonnet-4-6-low",
	"claude-sonnet-5",
	"claude-sonnet-5-max",
	"claude-sonnet-5-xhigh",
	"claude-sonnet-5-high",
	"claude-sonnet-5-medium",
	"claude-sonnet-5-low",
	"claude-sonnet-5-thinking",
	"claude-opus-5",
	"claude-opus-5-max",
	"claude-opus-5-xhigh",
	"claude-opus-5-high",
	"claude-opus-5-medium",
	"claude-opus-5-low",
	"claude-opus-5-thinking",
	"claude-fable-5",
	"claude-fable-5-xhigh",
	"claude-fable-5-high",
	"claude-fable-5-medium",
	"claude-fable-5-low",
	"claude-fable-5-1",
	"claude-fable-5-1-xhigh",
	"claude-fable-5-1-high",
	"claude-fable-5-1-medium",
	"claude-fable-5-1-low",
	"claude-mythos-5",
	"claude-mythos-5-1",
```
`relay/channel/aws/constants.go` `awsModelIDMap` 追加（Bedrock 官方 ID）：
```go
	"claude-haiku-4-5":  "anthropic.claude-haiku-4-5",
	"claude-sonnet-5":   "anthropic.claude-sonnet-5",
	"claude-opus-5":     "anthropic.claude-opus-5",
	"claude-fable-5-1":  "anthropic.claude-fable-5-1",
```
`relay/channel/vertex/adaptor.go` `claudeModelMap` 追加（Google Cloud 官方 ID）：
```go
	"claude-sonnet-4-6": "claude-sonnet-4-6",
	"claude-haiku-4-5":  "claude-haiku-4-5@20251001",
	"claude-sonnet-5":   "claude-sonnet-5",
	"claude-opus-5":     "claude-opus-5",
	"claude-fable-5-1":  "claude-fable-5-1",
```

- [ ] **Step 10: 全部测试通过并提交**

```bash
go build ./... && go test ./setting/reasoning/ ./relay/channel/claude/ ./relay/channel/aws/ ./relay/channel/vertex/ ./relay/ -count=1 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'
git add setting/reasoning relay && git commit -m "feat(claude): Claude 5 family model list, provider ID maps and adaptive-thinking capability table

Generalizes the opus-4.6/4.7/4.8 prefix whitelist into reasoning.ClaudeCapabilitiesFor
(mirrors upstream relaykit reasoning/claude.go) so -high/-max/-thinking suffixes work
for claude-opus-5, claude-sonnet-5, claude-fable-5(.1) and claude-mythos-5(.1)."
```
Expected: 新用例全 PASS；基线已知 `TestRequestOpenAI2ClaudeMessage_*` 失败数量不增加。

---

### Task 11: 全量验证

- [ ] **Step 1: 全量构建、vet、测试并与基线比对**

```bash
go build ./... && go vet ./... 2>&1 | tail -5
go test ./... 2>&1 | grep -E '^(FAIL|--- FAIL)' > /tmp/after-test.txt; diff /tmp/baseline-test.txt /tmp/after-test.txt && echo 'NO NEW FAILURES'
```
Expected: `NO NEW FAILURES`（或 diff 只显示消失的失败）。

- [ ] **Step 2: 真库启动再验一次（含全部 DB 改动）**

同 Task 5 Step 3 的命令，确认 `/api/status` 200、日志无 panic/迁移错误；第二次启动日志里**不应再出现** `ALTER TABLE`/`CREATE INDEX` 类 DDL（9a8674425 目标）。

- [ ] **Step 3: 溢出漏洞端到端（可选，需本地服务 + 一个 token）**

```bash
curl -s http://127.0.0.1:5002/v1/images/generations -H "Authorization: Bearer $TK" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-1","prompt":"x","n":18446744073686646784}' | head -c 300
```
Expected: 4xx 参数错误，而非 200/500；用户额度不变。

---

### Task 12: 记录与汇报

**Files:** `docs/superpowers/WORKLOG.md`, 本计划文件

- [ ] **Step 1: WORKLOG 追加条目**

在 `docs/superpowers/WORKLOG.md` 末尾追加 `### [2026-09-17] 上游必要修复同步 + 最新模型列表（分支 chore/upstream-sync-2026-09，未合入 main / 未 push）`，按既有格式写：需求、方法（merge-tree 预演 → cherry-pick -x 分层）、逐提交清单与冲突处理、手工移植与新增（Claude 5 能力表、定价）、验证结果（基线对比、真库启动）、**未包含项**（057f71c23 延后原因；Azure Claude 用 Anthropic 渠道即可）、提交状态。

- [ ] **Step 2: 提交计划文件与 WORKLOG（plans 目录被 .gitignore 命中，需 -f）**

```bash
git add -f docs/superpowers/plans/2026-09-17-upstream-sync-security-models.md docs/superpowers/WORKLOG.md
git commit -m "docs: upstream sync plan + worklog (security fixes, latest models)"
git log --oneline main..HEAD
```

- [ ] **Step 3: 汇报**

向用户汇报：分支名、提交列表（含上游哈希）、验证结果、未包含项、下一步需用户指令：合入 main / push / `/tke-release`。

---

## 执行结果与偏差（2026-09-17 执行完毕）

- Task 1 依赖链比计划多 3 个提交：`bae799ccb`、`3fbad6a72`、`d9595831b`（621927f71 依赖 QuotaClamp）；`fc1259f58` 只移植 3 个 PriceData 方法。
- Task 5：`66031a09d` 落地但 **gorm 保持 1.25.2**（1.25.12 的 Scan 清零语义弄坏 `SumSupplierStat`）；`9a8674425` 实测在 gorm 1.25.2 上无效（两次启动 DDL 48 条 = main 基线），**已摘除**，待 gorm/pg 驱动升级时重做。sqlite 驱动不升。
- Task 6：采纳上游"relay 不跟随 3xx"行为（`keepUpstreamRedirectResponse`），保留本地 client 选择逻辑。
- Task 7：上游测试的 BillingUsage 断言改为本地顶层 usage 字段；取消时 CompletionTokens 断言 ≥1（本地按已流出文本估算）。
- Task 9：gpt-5.5/5.6 完成倍率为 6（非计划里写的 8，按 2f5f6ba84 规则）；本地无 builtin billing expr 表，gpt-6-astra 只给平价倍率。
- 其余按计划执行；全量测试与基线一致，PG 真库两次启动验证通过。
