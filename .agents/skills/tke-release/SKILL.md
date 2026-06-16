---
name: tke-release
description: >-
  一键发版 tokenki 服务到腾讯云 TKE 集群(cls-tokensolo-tke-japan / ns tokenki):
  打 tag → 等 GitHub Actions 镜像构建 → 本地 kubectl 滚动部署 → 健康验证 → 生成发布报告。
  当用户要"发版/发布/部署 tokenki 到 TKE 或 k8s"、"打 tag 发新版本"、"上线新版本到集群"时使用。
  tag 可选，缺省按命名规则自动生成下一个最新版本号。
# 仅人工手动触发(/tke-release),禁止模型自动调用——发版为高副作用操作。
disable-model-invocation: true
# 参数占位提示:tag 可选。
argument-hint: "[tag]"
# 最小权限:仅发版所需工具。git/gh/kubectl/curl/tccli + 读文档/写报告/询问 commit。
allowed-tools:
  - Bash(git *)
  - Bash(gh *)
  - Bash(kubectl *)
  - Bash(curl *)
  - Bash(tccli *)
  - Bash(go build *)
  - Bash(go test *)
  - Bash(cd *)
  - Bash(export *)
  - Bash(date *)
  - Read
  - Write
  - Edit
  - AskUserQuestion
  - Agent
---

# TKE 一键发版 (tokenki)

自动化 TKE 发版全流程：`前置检查 → git tag → push → 镜像构建(Actions) → kubectl 滚动部署 → 冒烟测试 → 监控`。

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `tag` | 否 | 缺省按下方规则自动生成下一个版本号；格式 `vYYYY.MM.DD.N`。 |

调用示例：`/tke-release`、`/tke-release v2026.06.16.2`。

> 仅 **prod** 环境，无 staging。生产发版会在 Phase 1 弹窗二次确认。

## ⚠️ 授权与约束

- 平时对线上 TKE 集群是**只读**约束。**本 skill 是用户主动发版，流程内的 `kubectl set image / rollout / undo` 等集群写操作在此次发版中被授权**——仅限本发版流程，不扩展到其他场景。
- **不自动 `git commit`**：预检若发现未提交改动，用 `AskUserQuestion` 询问用户是否提交及 commit message，得到确认后再 commit；tag / push / 部署属发版核心，用户调用即视为授权，自动执行。

## 🚨 致命红线（高于所有 Phase）

**在 P3 部署后或 P4 冒烟测试中发现以下任一情况，必须立即 `kubectl rollout undo` 止血**：

- panic（`app_panic` / `runtime error` / `nil pointer dereference` 等）
- Pod 反复 CrashLoopBackOff / ImagePullBackOff
- 5xx 高频率出现（不只 1-2 个偶发）
- 100% 请求失败 / 健康检查持续异常 / 关键接口完全不可用

**生产恢复前严禁以下行为**：
- ❌ 拉 panic 栈分析、grep 源码、读 commit diff
- ❌ 解释"为什么会 panic"、推断根因、看 git log
- ❌ "再测一次看看"、"等几秒看是不是偶发"

**正确顺序**（不可颠倒）：
1. **回滚** — 先 slave 再 master（见 Phase 5 回滚命令）
2. **复测验证恢复** — 至少用一发真实请求确认 200，并查 `kubectl logs` 无 panic
3. **进入 Phase 5 事故响应** — Pod logs / CLS 日志 / GHCR 镜像回滚后照样能拿，不会丢

> 每多 1 分钟分析时间 = 1 分钟全量流量挂掉。先止血，再分析。

---

## 进度面板格式

每个 Phase **开始前**和**完成后**各输出一次：

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 TKE 发版进度  <tag>  [prod]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 ✅ P1  前置检查     完成
 🔄 P2  镜像构建     进行中（约 8 分钟）
 ⬜ P3  集群部署     等待
 ⬜ P4  功能冒烟测试  必做
 ⬜ P5  事故响应     仅异常时触发
 ⬜ P6  部署后监控   3 分钟异步
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

图标含义：`✅` 完成 · `🔄` 进行中 · `⬜` 等待 · `❌` 失败/中止

tag 在 Phase 1 确认版本号之前用 `待定`。

---

## 服务参数

| 项 | 值 |
|----|-----|
| 仓库目录 | `tokenki/`（当前工作区） |
| GitHub repo | `heiying0917/new-api` |
| 分支 | `main` |
| tag 格式 | `vYYYY.MM.DD.N`（当日序号） |
| 构建 workflow 名 | `Publish Docker image (GHCR for TKE)` |
| 镜像 | `ghcr.io/heiying0917/tokenki:<tag>` |
| Deployment | `tokenki-api-master`(×1) + `tokenki-api-slave`(×2) |
| 容器名(set image) | `tokenki-api` |
| rollout timeout | master 120s / slave 300s |
| prod 健康 URL | `https://tokenki.com/api/status` |
| 健康检查成功标志 | HTTP 200 + JSON `success: true` |
| KUBECONFIG | `~/.kube/tokensolo.config`（复用 tokensolo） |
| Namespace | `tokenki` |

---

## Phase 1 — 前置检查

**开始时**输出进度面板（P1 标记 🔄，其余 ⬜，tag=待定）。

### Step 1.0 — 生产发版二次确认

```bash
HEALTH_URL="https://tokenki.com/api/status"
BASE_URL="https://tokenki.com"
echo "发版环境: prod | HEALTH_URL: $HEALTH_URL"
```

用 `AskUserQuestion` 弹窗：

```
question: "即将发布到生产环境（tokenki.com），请再次确认"
header: "⚠️ 生产发版确认"
options:
  - label: "确认，发布到生产（prod）"
    description: "流量将切换到新版本"
  - label: "取消，不发版"
    description: "退出 skill"
multiSelect: false
```

用户选"取消"则退出 skill。

### Step 1.1 — 代码预检

每一步失败立即停止，不跳过：

```bash
# 1. 仓库当前路径就是工作区
cd "$(pwd)"

# 2. 确认分支
git branch --show-current        # 必须是 main，否则提示用户确认

# 3. 拉最新代码
git fetch origin
git pull origin main             # 拉 main 最新代码

# 4. 拉最新远端 tag（关键！本地 tag 可能落后，会导致版本号推算错误）
git fetch --tags

# 5. 确认工作区干净
git status --short               # 有未提交改动 → AskUserQuestion 询问是否处理（不自动 commit）

# 6. 构建检查
go build ./...

# 7. 测试检查
# ⚠️ controller / logger 包必须覆盖——任何路径若引入 nil pointer / panic 都会全量挂掉。
go test ./model/... ./service/... ./controller/... ./logger/... 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
```

**测试失败处理**（避免被陈旧测试卡住发版）：

1. 先 `go test ./xxx/ -run <TestName> -v` 看错误**性质**：
   - **代码演进、断言陈旧**（模型列表多/少一个、迁移规则改了等）→ 直接改测试 assertion 与代码对齐
   - **业务代码新引入失败** → ❌ 停止，对比 HEAD~1 验证，报告用户
2. **白名单兜底**：若维护了 `docs/known-stale-tests.txt`：
   ```bash
   FAILS=$(go test ./model/... ./service/... ./controller/... ./logger/... 2>&1 \
       | grep "^--- FAIL:" | awk '{print $3}')
   NEW_FAILS=$(echo "$FAILS" | grep -vFxf docs/known-stale-tests.txt 2>/dev/null | grep -v "^$" || true)
   if [ -n "$NEW_FAILS" ]; then
       echo "❌ 新失败（不在白名单内）："; echo "$NEW_FAILS"; exit 1
   fi
   ```

**pull origin main 冲突处理原则**：
- **不确定如何解决的冲突**：任何**无法确信正确解法**的冲突，**立即停止，绝不自作主张**——把冲突的文件和冲突段落清楚列给用户，等用户确认解法后再继续。
- 冲突全部解决、commit 完成后，才进入打 tag。**带未解决冲突或脏工作树绝不打 tag。**

### Step 1.2 — 确定 tag

```bash
git tag --sort=-creatordate | grep -E '^v[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+$' | head -1
```

格式 `vYYYY.MM.DD.N`，日期取**今天**（`date +%Y.%m.%d`）。N = 今天已有 tag 的最大序号 +1；今天还没有则 N=1。

用户指定了 tag 则直接用。

**用 `AskUserQuestion` 弹窗确认版本号**（用户指定 tag 时跳过）：

```
question: "即将发布 <推算版本号> 到 prod，请确认或输入自定义版本号"
header: "确认发布版本"
options:
  - label: "<推算版本号>（推荐）"
    description: "自动推算的下一个版本号，直接使用"
  - label: "自定义版本号（点 Other 输入）"
    description: "若需要跳号或特殊格式，手动输入"
multiSelect: false
```

**完成后**输出进度面板（P1 ✅，P2 🔄，其余 ⬜，tag 替换为实际值）。

---

## Phase 2 — 镜像构建

**开始时**输出进度面板（P1 ✅，P2 🔄，其余 ⬜）。

```bash
# 打 tag 前双重确认（两条都必须满足，否则停止）

# 1. 工作区必须干净——有未提交改动直接中止
if [ -n "$(git status --short)" ]; then
  echo "❌ 工作区有未提交的改动，必须先提交后再发版，中止"
  exit 1
fi

# 2. 本地 main 必须已 push 到 origin/main——有未 push commit 先推
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/main)
if [ "$LOCAL" != "$REMOTE" ]; then
  echo "发现本地有未 push 的 commit，先 push..."
  git push origin main
fi

git tag <tag>
git push origin <tag>        # 推 tag → 触发 GitHub Actions 镜像构建
```

```bash
gh run list -R heiying0917/new-api -L 5
gh run watch <run-id> -R heiying0917/new-api --exit-status   # 阻塞到完成；失败则终止发版并报告

# ⚠️ 二次校验（必做）：gh run watch 遇到 GitHub API 502 时会无重试 exit 0 假装成功
# 必须用 gh run view JSON 状态确认 status=completed AND conclusion=success
for i in {1..30}; do
  STATUS=$(gh run view <run-id> -R heiying0917/new-api --json status,conclusion \
    --jq '.status + "/" + (.conclusion // "null")' 2>/dev/null)
  echo "[$i/30] $STATUS"
  [[ "$STATUS" == "completed/success" ]] && break
  [[ "$STATUS" == completed/* ]] && { echo "❌ 非 success: $STATUS"; exit 1; }
  sleep 10
done
```

取最近一次由该 tag 触发的 `Publish Docker image (GHCR for TKE)` workflow。构建失败 → 停止，不部署。
amd64-only 后平均约 5 分钟（multi-arch 是 25 分钟）。

**关键 SOP**：watch + 二次校验**必须都成功**才进 Phase 3。watch exit 0 单独不能证明 build 完成——
2026-06-16 v2026.06.16.3 发版踩过坑：watch 因 502 假退出，我提前进了 P3，幸运的是镜像那时已 push
完成，pod 拉得到；如果 build 早期失败，pod 会 ImagePullBackOff 服务挂掉。

**完成后**输出进度面板（P1 ✅，P2 ✅，P3 🔄，其余 ⬜）。

---

## Phase 3 — 集群部署

**开始时**输出进度面板（P1 ✅，P2 ✅，P3 🔄，其余 ⬜）。

```bash
export KUBECONFIG=~/.kube/tokensolo.config
IMAGE=ghcr.io/heiying0917/tokenki:<tag>

# master 先滚动，等收敛
kubectl set image deployment/tokenki-api-master tokenki-api=$IMAGE -n tokenki
kubectl rollout status deployment/tokenki-api-master -n tokenki --timeout=120s

# slave 再滚动
kubectl set image deployment/tokenki-api-slave  tokenki-api=$IMAGE -n tokenki
kubectl rollout status deployment/tokenki-api-slave  -n tokenki --timeout=300s

# 健康检查（验证 HTTP 200 + success:true）
curl -fsS --retry 6 --retry-delay 5 --retry-connrefused https://tokenki.com/api/status \
  | grep -o '"success":\s*true' || { echo "❌ 健康检查未返回 success:true"; exit 1; }
```

> 零停机：`maxUnavailable=0` + `terminationGracePeriodSeconds=3660`，`set image` 为平滑滚动，不中断在途请求。

**部署后立即验证（两步都要做）**：

```bash
KC=~/.kube/tokensolo.config

# 1. 日志巡检（部署后 2 分钟内）——抓 panic / nil pointer
kubectl --kubeconfig $KC logs -n tokenki deployment/tokenki-api-master --tail=50 2>&1 \
  | grep -iE "panic|nil pointer|runtime error" | head -20

# 2. Pod 状态确认
kubectl --kubeconfig $KC get pods -n tokenki -o wide
```

- 健康检查 200 + success:true 且日志无 panic → 继续 Phase 4
- 发现 `panic` / `nil pointer` / Pod 反复 CrashLoopBackOff / 高频 5xx → **立即执行致命红线（见 Phase 5 回滚命令）→ 复测 → 进入 Phase 5。禁止先分析日志详情。**
- 零星普通错误（认证失败、模型不支持等业务错误）属正常波动，继续 Phase 4

**完成后**输出进度面板（P1-P3 ✅，P4 🔄 即将启动）。

---

## Phase 4 — 功能冒烟测试（必做）

> ⚠️ **本 Phase 必做，不存在"按需触发"。** 发版风险不能由 commit message 字面判断（"看上去无害的 helper 改动"也曾 100% 挂掉生产）。所有改动都必须用真实请求过一遍。

**开始时**输出进度面板（P1-P3 ✅，P4 🔄，P5-P6 ⬜）。

### Step 4.0 — 获取测试 token（必须）

**第一步**：优先从环境变量 `TOKENKI_SMOKE_API_KEY` 读取 token：

```bash
TOKEN="${TOKENKI_SMOKE_API_KEY:-}"
if [ -n "$TOKEN" ]; then
  echo "✅ 已从环境变量 TOKENKI_SMOKE_API_KEY 获取测试 token，直接使用"
fi
```

- **环境变量有值**：直接使用，跳过弹窗询问，继续 Step 4.1
- **环境变量为空**：用 `AskUserQuestion` 弹窗向用户索要：

```
question: "Phase 4 功能冒烟测试需要一个真实的 API Token（未找到环境变量 TOKENKI_SMOKE_API_KEY，可在 .claude/settings.local.json 中预配置以后自动跳过此步）"
header: "测试 Token"
options:
  - label: "粘贴 token（点 Other 输入）"
    description: "推荐：用真实 token 跑一发，唯一能发现 P0 级 panic 的方式"
  - label: "跳过冒烟测试（高风险，需二次确认）"
    description: "仅在确认本次发版纯文档/纯前端/已完整 staging 验证过时选择。跳过 = 你为线上挂掉负全责"
multiSelect: false
```

- 用户选**跳过**：在进度面板标 `P4 ⚠️ 跳过（高风险）`，必须再次显式确认"我接受跳过冒烟测试的全部风险"才能进入 P6
- 用户选**粘贴**：用提供的 token 继续 Step 4.1+

### Step 4.1 — 执行测试

```bash
BASE_URL="https://tokenki.com"
TOKEN=<用户提供的 token>

# --- 健康检查（先确认服务可达）---
curl -s $BASE_URL/api/status | grep -o '"success":\s*true'

# --- OpenAI Chat Completions（基线必测）---
curl -s $BASE_URL/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":10}'

# --- Anthropic Messages（/v1/messages，Claude Code 使用的接口）---
curl -s $BASE_URL/v1/messages \
  -H "x-api-key: $TOKEN" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}],"max_tokens":10}'
```

**各接口成功标志**：

| 接口 | 成功标志 | 核心字段 |
|------|---------|---------|
| `GET /api/status` | HTTP 200 + `success: true` | `data.system_name = "Tokenki"` |
| `POST /v1/chat/completions` | `object: "chat.completion"` | `choices[0].message.content` |
| `POST /v1/messages` | `type: "message"` | `content[0].text` |

> 模型可用性取决于已配置渠道。如某模型未配置返回 `model_not_found`，换一个已开通的模型重试，不算 P0 级失败。

从本次 commit message 提炼测试重点，追加对应场景（正向 + 边界 + 错误语义）。

**输出测试报告**（每次必须包含此表格）：

| 场景 | 请求模型/接口 | 预期 | 实际响应 | HTTP | 结论 |
|------|-------------|------|---------|------|------|
| 基线正常请求 | gpt-4o-mini | 200 正常 | ... | 200 | ✅ |

附：**未覆盖场景**（列出 + 说明无法覆盖的原因）

### ⚠️ 测试中发现致命错误的处理（强制顺序，不可颠倒）

任一冒烟请求返回 panic / nil pointer / 持续 5xx / 100% 失败时：

1. **立即停止后续测试**，不要"再试一次"也不要"换个接口看看"
2. **立即回滚止血**（见 Phase 5 回滚命令）— 此刻线上全量挂掉，**禁止做任何代码分析 / 看 panic 栈 / 查日志详情**
3. **复测验证恢复**：用同样的 curl 请求重发，确认 200 + `kubectl logs` 无 panic
4. **进入 Phase 5 事故响应** — 那里才是分析根因的地方

> panic 栈、CLS 日志、GHCR 镜像都不会因为回滚而消失，**回滚后照样能拿到全部证据**。每多分析 1 分钟 = 多 1 分钟全量挂掉。

**完成后**输出进度面板（P1-P4 ✅，P5 ⬜ 仅异常时触发，P6 🔄 即将启动）。

---

## Phase 5 — 事故响应（仅 P3/P4 发现致命错误时进入）

**进入前置条件**（必须全部满足）：
- ✅ 已回滚到上一个稳定版本（命令见下方）
- ✅ 已用真实请求复测确认集群恢复（HTTP 200 + 无 panic）
- ✅ 已在进度面板把 P3 / P4 标 ❌

如果以上任何一项未完成，**回到致命红线流程，先做完再来 Phase 5**。

### 回滚命令（先 slave 再 master，与滚动顺序相反）

```bash
export KUBECONFIG=~/.kube/tokensolo.config
kubectl rollout undo deployment/tokenki-api-slave  -n tokenki
kubectl rollout status deployment/tokenki-api-slave  -n tokenki --timeout=300s
kubectl rollout undo deployment/tokenki-api-master -n tokenki
kubectl rollout status deployment/tokenki-api-master -n tokenki --timeout=120s
# 验证恢复
curl -fsS https://tokenki.com/api/status | grep -o '"success":\s*true'
```

### Step 5.1 — 现场证据收集（集群已恢复，安全分析）

```bash
KC=~/.kube/tokensolo.config

# 1. 拉 panic Pod 的完整日志（回滚后旧 Pod 可能已终止，先取最近日志）
kubectl --kubeconfig $KC logs -n tokenki deployment/tokenki-api-master --previous --tail=200 2>&1 \
  | grep -A 50 "panic\|Recovery"

# 2. 查看 Pod 事件
kubectl --kubeconfig $KC describe pods -n tokenki -l app=tokenki-api,role=master | grep -A 10 "Events:"

# 3. 拉 GHCR 问题版本镜像本地复现
docker pull ghcr.io/heiying0917/tokenki:<panic-tag>
CID=$(docker create ghcr.io/heiying0917/tokenki:<panic-tag>)
docker cp $CID:/tokenki /tmp/tokenki-panic
docker rm $CID
echo "0xXXXXXXX" | go tool addr2line /tmp/tokenki-panic

# 4. 本地复现（核心）——切到 panic commit，本地 dev backend 用真实 token 触发
git checkout <panic-commit>
```

### Step 5.2 — 根因分析与修复

按以下顺序输出（不自作主张修代码前，先把 4 个块写完给用户看）：

```
**问题描述**
复现步骤：[具体请求 + 参数]
实际：[HTTP 状态码 + error code + message]
预期：[应该返回什么]

**影响范围**
受影响的用户组 / 场景：[...]
触发条件：[...]

**根因分析**
代码路径：[文件:行号]
关键变量：[...]
为什么 P1 测试没拦下：[...]（必填，用于反推 SOP 是否有缺口）

**修复方向**
[简要描述修复思路]
```

用户确认修复方向后才动手 Edit。

### Step 5.3 — 修复 + 测试覆盖 + SOP 加固

| 步骤 | 内容 |
|---|---|
| 1. 写测试**先于**写代码 | 先加一个能 catch 该 bug 的测试（regression test）。**反向验证**：把修复回退到 bug 版本，确认测试 FAIL；再恢复修复，确认 PASS |
| 2. 修复代码 | 最小化改动 |
| 3. 加防御兜底 | 同类型路径的全局护栏（如 logger 路径加 `defer recover`） |
| 4. 更新 SOP | 如果 P1 测试范围 / Phase 顺序 / 红线有缺口，**这次必须补**，不要"下次再说" |
| 5. 重新发版 | 跑完整 6 个 Phase（包括强制 P4 冒烟），tag 用同日下一序号 |

### Step 5.4 — 事故归档

在 `docs/report/` 写事故复盘 md（文件名 `YYYY-MM-DD-<short-desc>.md`），包含：
- 时间线（精确到分钟，含集群挂掉总时长）
- 根因 + 误判记录（SOP / 测试 / Skill 哪里失守）
- 改进项（已落地的具体 commit / SOP 行号）

**完成后**输出进度面板（P5 ✅ 已修复），并问用户是否继续重发新版本。

---

## Phase 6 — 部署后监控（3 分钟异步）

**触发时机**：P4 冒烟测试成功后立即启动，不阻塞用户交互。ScheduleWakeup 异步执行，有异常时主动通知。
（P4 发现致命错误已触发回滚 → P5 事故响应，此时 P6 不启动；新版本重发后再走完整流程。）

**开始时**输出进度面板（P1-P4 ✅，P5 ⬜，P6 🔄），告知用户监控已在后台运行，无需等待。

记录部署完成时刻为监控起始时间，用 TaskCreate 创建监控任务，启动 30 秒间隔循环检查。

### CLS 日志配置

- **Region**：`ap-tokyo`
- **TopicId**：`a31bee45-4b9e-4026-b747-5ca2f6656284` ✅
- **主题名**：`tke_tokenki`
- **过滤字段**：`service:tokenki`（应用通过 `LOG_SERVICE_NAME` env 暴露）
- **索引模式**：JSON 提取 + 键值索引已建（18 字段：status/level/type/service/path/method/client_ip/latency_ms/route_tag/user_id/model/channel_id/error_reason/elapsed_ms 等业务字段 SqlFlag=true；request_id/msg/sql 高基数 SqlFlag=false）

### 查询工具选择（关键）

> **默认 `tccli cls SearchLog` + SQL 聚合**——SQL 在 CLS 服务端聚合后只回几行结论
> （如 `status=200, cnt=268`），主对话直接消化，**无需子 agent 中转，token 消耗 ~200**。
>
> ❌ 不要用 CLS MCP `SearchLog` 工具拉原始日志条目——单次返回几 KB×N 条 raw log，
> 会污染主对话上下文，且 LLM 还得二次过滤聚合。
>
> ✅ MCP/子 agent 仅在以下场景用：
> - 需要拉某条具体 `request_id` 的完整日志原文（如 panic stack 全文 + 上下文）
> - SQL 无法表达的复杂检索（很少见）

### 查询原则（关键）

> **每轮必须从监控起点查到当前时间，不能只查最近 1 分钟。**
> CLS 有约 1 分钟采集延迟，滑动窗口会漏掉边界日志。固定 From 累计查才能覆盖所有延迟到达的日志。

### tccli 查询模板

时间戳获取（毫秒级）：
```bash
TOPIC_ID="a31bee45-4b9e-4026-b747-5ca2f6656284"
FROM=<监控起点毫秒，Phase 3 部署完成时刻 $(($(date +%s) * 1000)) 记录>
TO=$(($(date +%s) * 1000))
```

**步骤 1 — 按 status 分组统计（4xx/5xx 计数）**：
```bash
tccli cls SearchLog --region ap-tokyo \
  --TopicId $TOPIC_ID --From $FROM --To $TO \
  --Query 'service:tokenki AND type:access AND status:>=400 | SELECT status, count(*) AS cnt GROUP BY status ORDER BY cnt DESC LIMIT 20' \
  --Limit 20 2>&1 | python3 -c '
import json, sys
d = json.load(sys.stdin)
rows = [{kv["Key"]: kv["Value"] for kv in r["Data"]} for r in d.get("AnalysisResults", [])]
print(rows)'
```

**步骤 2 — 若有 5xx，拉明细看路径 / 错误原因 / 用户 / 渠道**：
```bash
tccli cls SearchLog --region ap-tokyo \
  --TopicId $TOPIC_ID --From $FROM --To $TO \
  --Query 'service:tokenki AND status:>=500 | SELECT level, status, path, model, channel_id, error_reason, user_id LIMIT 10' \
  --Limit 10 2>&1 | python3 -c '
import json, sys
d = json.load(sys.stdin)
rows = [{kv["Key"]: kv["Value"] for kv in r["Data"]} for r in d.get("AnalysisResults", [])]
[print(r) for r in rows]'
```

**步骤 3 — 与发版前同时段对比**（改 From/To 为发版前 1 小时的同窗口）：
```bash
tccli cls SearchLog --region ap-tokyo \
  --TopicId $TOPIC_ID --From $FROM_PREV --To $TO_PREV \
  --Query 'service:tokenki AND status:503 AND path:"/v1/chat/completions" | SELECT count(*) AS cnt' \
  --Limit 1 2>&1 | python3 -c '
import json, sys
d = json.load(sys.stdin)
print(d["AnalysisResults"][0]["Data"][0]["Value"] if d.get("AnalysisResults") else 0)'
```

> 若发版前 0 条、发版后突现 → 疑似引入；发版前后量级相当 → pre-existing。

**字段名提醒**：业务字段是 `status`（不是 status_code）、`type`（access/sql 区分）。
`request_id` / `msg` / `sql` 是 SqlFlag=false，只能在 Query 部分作过滤条件，**不能 SELECT/GROUP BY**——
如需拉某 request_id 的完整日志，用子 agent + CLS MCP，不要 SQL。

### 执行流程

1. 监控起始时间转为 **From timestamp 毫秒（固定不变，贯穿全部 6 轮）**：
   `FROM=$(($(date +%s) * 1000))` —— 在 Phase 3 部署完成 + 健康检查通过那一刻立即执行
2. 用 TaskCreate 创建监控任务，记录起始 timestamp 和初始累计状态（空）
3. 用 ScheduleWakeup（delaySeconds=30）启动循环，每轮 prompt 携带：
   - 监控起始 From timestamp（固定）
   - 上轮累计已发现的错误列表（含时间、状态码、路径、次数）
   - 当前轮次编号 / 总轮次（6）
4. 每轮唤醒后：
   - 计算当前 To timestamp：`TO=$(($(date +%s) * 1000))`
   - **主对话直接 Bash 跑步骤 1 的 tccli + python 解析**（From=固定起点，To=当前时间）
   - 拿到 1-5 行聚合结果（如 `[{'status': '200', 'cnt': '268'}]`），不会污染上下文
   - 与上轮累计结果对比，提取本轮**新增**条目
   - 若有新增 5xx：跑步骤 2 拉明细（仍 tccli）→ 按相关性准则判断
   - 疑似版本引入则立即报告并停止监控进入 Phase 5
   - **每轮必须输出一行状态行**（格式见下方）
   - 未到第 6 轮：ScheduleWakeup(delaySeconds=30)
   - 到第 6 轮：输出完整汇总报告，TaskUpdate 标记任务完成

### 每轮状态行格式（必须输出）

```
[P6 监控 轮次 N/6 | HH:MM] 累计 X 条 4xx/5xx — [正常 / ⚠️ 新增 Y 条，见下方]
```

### 相关性判断准则

| 错误特征 | 判断 |
|---------|------|
| 路径与改动模块完全无关 | 非本次引入，记录但继续监控 |
| 路径或模型与改动模块匹配 | ⚠️ 疑似本次引入，停止监控，进入 Phase 5 |
| 错误在发版前已存在（同路径同模式） | pre-existing，记录说明后继续 |
| 全渠道同时爆发（账号耗尽 / bootstrap 403） | 上游基础设施问题，非版本引入，继续监控 |
| 单用户 4xx（model_not_found / quota 不足） | 用户侧问题，非版本引入，继续监控 |

### 最终汇总报告格式

```
**TKE 部署后 3 分钟监控报告（HH:MM - HH:MM）**

| 时间 | 状态码 | 路径 | 次数 | 与本次变更相关 |
|------|--------|------|------|--------------|
| ...  | ...    | ...  | ...  | ✅ 无关 / ⚠️ 疑似 / 🔵 pre-existing |

总计：X 条（4xx: N，5xx: M）
结论：[正常 / 发现疑似版本引入问题，已进入 Phase 5 事故响应]
```

---

## 发布报告模板

**发版完成后写入 `docs/deploy/report/<YYYY-MM-DD>-tokenki-prod-<tag>.md`**：

```markdown
# 发布报告 — tokenki {{tag}}

> 环境:prod · 集群:cls-tokensolo-tke-japan / ns tokenki · 时间:{{YYYY-MM-DD HH:MM}}

## 概览
| 项 | 值 |
|----|----|
| 服务 | tokenki |
| 环境 | prod |
| 版本 tag | {{tag}} |
| 镜像 | ghcr.io/heiying0917/tokenki:{{tag}} |
| 发布结果 | {{✅ 成功 / ❌ 失败(已回滚)}} |

## 各阶段结果
| 阶段 | 状态 | 详情 |
|------|------|------|
| 合并 main | {{状态}} | {{无冲突 / 已解决冲突: 文件}} |
| commit | {{状态}} | {{commit hash / 无新提交}} |
| 打 tag & push | {{状态}} | {{tag}} |
| 镜像构建 (Actions) | {{状态}} | run-id {{id}}，耗时 {{x}}s |
| 集群部署 | {{状态}} | {{rollout 摘要}} |
| 健康检查 | {{状态}} | https://tokenki.com/api/status → HTTP {{code}} |
| 冒烟测试 | {{状态}} | {{接口 / 跳过原因}} |

## 部署详情
| Deployment | 旧镜像 | 新镜像 | rollout |
|-----------|--------|--------|---------|
| tokenki-api-master | {{old-tag}} | {{new-tag}} | {{状态}} |
| tokenki-api-slave  | {{old-tag}} | {{new-tag}} | {{状态}} |

## 验证
| 检查项 | 结果 |
|--------|------|
| Pod 全 Running | {{✅ N/N}} |
| 镜像 tag 已更新 | {{状态}} |
| 日志无 panic | {{✅ / ⚠️ 发现: ...}} |
| API 冒烟 | {{✅ / ⚠️ 跳过（高风险）}} |

## 回滚记录（仅失败时填，成功则写"无"）
| 项 | 值 |
|----|----|
| 回滚原因 | {{reason}} |
| 回滚 Deployment | {{list}} |
| 回滚结果 | {{✅ 已恢复到 prev-tag}} |
```

---

## 注意事项

1. **发版基于 main 分支**——打 tag 前确认本地 main 与 origin/main 一致（Phase 2 自动检查并 push）；workflow 触发条件为 `v*`。
2. tag 打在 **main 分支**。
3. **环境**：仅 prod。无 staging。
4. **发版路径**：主路径用本 skill（含 Phase 4 冒烟测试 + Phase 6 监控）。`tke-manual-release.yml` 仅保留 `workflow_dispatch` 作为紧急备用（本地 Claude Code / 本机环境不可用时在 GitHub 手动触发，无冒烟，用后补测）。
5. **共集群影响**：所有 kubectl 命令**必须**带 `-n tokenki`，不允许 `--all-namespaces`，禁止误操作 tokensolo / ingress-nginx / cert-manager 等命名空间。

---

## 易踩坑速查

| 坑 | 正确做法 |
|----|---------|
| 本地 tag 过时，版本号推算错 | Phase 1 必须先 `git fetch --tags` |
| 有未提交改动就打 tag | Phase 2 强制检查工作区干净，否则中止 |
| 本地 commit 未 push 就打 tag | Phase 2 自动 push main 后再打 tag |
| 合并 main 产生冲突自行处理 | 不确定的冲突立即停止，把冲突段落列给用户确认解法 |
| 未等 GitHub Actions 完成就部署 | watch 输出 + `gh run view --json status,conclusion` 二次校验都 success 才进 P3——watch 遇 502 会假 exit 0 |
| slave 先于 master 回滚 | 回滚顺序：先 slave → 再 master（与滚动顺序相反） |
| 健康检查只看 HTTP 200 | tokenki `/api/status` 必须额外验证 body 含 `success: true` |
| Phase 4 跳过冒烟测试 | 必做。用 `AskUserQuestion` 主动索要测试 token |
| 用 commit message 判断是否需要冒烟 | "看上去无害的改动"也曾 100% 挂掉生产。所有改动都必须真实请求过一遍 |
| 发现 panic 后先去拉栈分析 | **立即 rollout undo 止血**！证据回滚后照样能拿到 |
| CLS 监控用滑动时间窗口 | 必须从监控起点固定 From 查到当前时间 |
| CLS 忘记加服务名过滤 | 每条查询前置 `service:tokenki`（避免混入 tokensolo / crs 日志） |
| CLS 监控用 MCP SearchLog 拉原始日志 | 默认用 `tccli cls SearchLog + SQL 聚合`，单次回 ~200 tokens；MCP 仅排查单条 request_id 详情时用 |
| 误用 `status_code` 字段名 | 索引里字段叫 `status`（与 access log JSON 字段一致），SQL 用 `status:>=400` 而非 `status_code:>=400` |
| 误操作 tokensolo 命名空间 | kubectl 命令始终带 `-n tokenki`，绝不 `--all-namespaces` |
