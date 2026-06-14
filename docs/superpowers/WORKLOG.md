# TokenKi 工作日志 (WORKLOG)

> **全局规则（用户 2026-06-14 确立）**：每完成一部分任务都必须在此追加一条记录——**何时 / 做了什么 / 改了哪些文件 / 如何验证**，以便日后追溯"什么时间做了什么、改了什么"。追加式、按时间顺序，最新在底部。
>
> 关联：安全架构方案 `docs/superpowers/specs/2026-06-14-tokenki-security-architecture.md`；核心整改实施计划 `docs/superpowers/plans/2026-06-14-tokenki-security-core-money-key.md`。
> 约定：未经用户明确指令，不 commit / push / 发布 / 部署。

---

## 2026-06-14 · v5 安全工作流

### [2026-06-14] 安全审计 + 长期架构方案（调研，未改码）
- **做了什么**：6 路 agent 安全审计（39 发现）；8 路首席架构师 agent 长期/规模化深挖 + 综合执行层。
- **产出**：`docs/superpowers/specs/2026-06-14-tokenki-security-architecture.md`（执行层 + 8 域 + 附录，~46 页）。
- **验证**：交叉核验，亲手复核命门项（官key 明文 `model/channel.go:27`、结算活取现价套现 `model/settlement.go`、PG/Redis 弱口令）。

### [2026-06-14] P0 安全加固（代码，已实测，**未提交**）
- **做了什么**：① 账号维度登录限流+渐进锁定（免疫 IP 轮换）；② `SetTrustedProxies`；③ 可吊销会话（authHelper 回查 GetUserCache）；④ 供应商渠道 base_url SSRF 校验；⑤ Cookie Secure 可配 + session fixation 修复；⑥ 注册自动登录；⑦ 管理员「解锁登录」+ `UNLOCK_ALL_ON_START`；登录日志注入修复；用户表角色标签补 role=5「供应商」。
- **改了哪些文件**：`common/{constants,init,login_throttle}.go`（新增 login_throttle）、`model/{user,user_cache,main}.go`、`controller/{user,supplier_channel}.go`、`middleware/auth.go`、`main.go`、`web/classic`（RegisterForm、用户表解锁/角色标签、UserArea/SiderBar）。
- **验证**：PG+Redis 环境实测——暴破第 10 次锁定、锁定挡正确密码、DB+Redis 双写、管理员解锁恢复、注册自动登录进控制台、管理员登录无回归；`go build`/`go test ./model` 通过；浏览器实测。

### [2026-06-14] 核心整改实施计划 + 设计精化回写（文档）
- **做了什么**：产出实施计划 `docs/superpowers/plans/2026-06-14-tokenki-security-core-money-key.md`（9 条零差错铁律 + 5 阶段 A–E）；在架构方案两域回写「设计精化」（计费逐条快照、隔离≠加密）；回写用户确认的 3 决策（不回填存量、无退款、日志库同库）。

### [2026-06-14 ~23:50] Phase B 计费逐条单价快照 + 结算累加（代码 + 测试，**未提交**）
- **做了什么**：成交价在消费写日志那一刻冻结，结算按条累加，拆掉"事后改价套现"。统一全部 4 处算钱口径为「Σ(official_usd × cost_price_snapshot)」。
- **改了哪些文件**：
  - `model/log.go`：Log + RecordConsumeLogParams 加 `CostPriceSnapshot`，写入时赋值。
  - `service/text_quota.go`：供应商渠道消费时冻结 `ch.CostPrice` 到日志。
  - `model/settlement.go`：`CreateSettlement.ComputedCNY` 改为 `SUM(official_usd*cost_price_snapshot)`，删除活取现价循环。
  - `model/settlement_query.go`：`GetSupplierPendingStat`、`GetSettlementChannelBreakdown` 改快照口径（明细单价=有效加权价）。
  - `model/supplier_stats.go`：新增 `GetUnsettledReceivableByChannels`（快照应收）。
  - `controller/supplier_channel.go`：渠道列表应收改用快照应收。
  - 测试：`model/settlement_test.go`（新增套现回归/期间改价/删渠道）、`settlement_query_test.go`、`supplier_aggregation_test.go` 同步设置快照。
- **验证**：`go test ./model/` 全绿（含 `TestSettlement_AntiCashOut`：改价到 10.0 后结算仍按冻结价 2.0=3.0）；`go build ./...` 通过；全量 `go test ./...` 仅 `relay/channel/claude` 2 个**预先存在、与本次无关**的失败（stash 基线复验证实）。
- **剩余**：使用日志逐行「¥收益」前端列（纯展示，后端数据已就绪）——并入前端统一处理。

### [2026-06-15 ~00:07] Phase C 结算原子幂等 + 资金账本（代码 + 测试，**未提交**）
- **做了什么**：① 确认/撤销改条件原子 `UPDATE ... WHERE status=applied` + `RowsAffected==1` 校验，杜绝两超管并发确认导致的重复打款(TOCTOU)；② `CreateSettlement` 加每供应商进程内互斥锁，串行化并发发起（配合 DB 层 settlement_id=0 去重）；③ 新增 append-only 资金账本 `settlement_ledger`（记录 create/confirm/cancel 的金额快照 + 操作者 + 篡改检测哈希），在 4 个结算接口埋点。
- **改了哪些文件**：
  - `model/settlement.go`：`ConfirmSettlement`/`CancelSettlement` 条件原子化；`CreateSettlement` 加 `lockSupplierSettlement`（sync.Map 互斥）。
  - `model/settlement_ledger.go`（新增）：`SettlementLedger` 表、`RecordSettlementLedger`、`SettlementSnapshotHash`、`GetSettlementLedger`。
  - `model/main.go`：AutoMigrate 加 `&SettlementLedger{}`。
  - `controller/settlement.go`：Create/Confirm/Cancel(Supplier+Admin) 成功后写账本（operator=当前用户、是否管理员、快照哈希）。
  - 测试新增：`model/settlement_ledger_test.go`（账本追加、哈希篡改敏感、二次确认不覆盖金额、撤销后不能确认、**并发确认只成功一次** ×10 不 flaky）。
- **验证**：`go build` 通过；`go test ./model/` 全绿（26 结算相关用例 + 并发 ×10）。注：`-count=3` 跑全包会触发**预先存在**的非幂等测试 `payment_method_guard_test`（users.aff_code 唯一约束，与本次无关）。
- **剩余**：结算明细页展示账本/操作者（前端，可选）；更广义的管理动作审计（提权/封号等）属 P2 另议。

### [2026-06-15 ~00:20] 检查测试（P0 + B + C 复核）+ 修一处回归
- **做了什么**：全量 `go build` / `go vet` / `go test ./...`；stash 基线对比区分"我的失败 vs 预先存在"。
- **发现并修复回归**：`middleware/auth.go` 的可吊销会话回查在 `model.DB` 未初始化的纯中间件单测下 nil 指针 panic（`TestHeaderNavModulePublicOrUserAuthAllowsLoggedInWhenDisabled`）→ 加 `model.DB != nil` 守卫（数据层未就绪时降级信任会话值；生产路由注册晚于 InitDB 恒就绪）。改 `middleware/auth.go`。
- **验证**：修复后全量 `go test ./...` **零新增失败**，仅剩预先存在：controller `TestListModelsTokenLimitIncludesTieredBillingModel`、claude 3×（`TestRequestOpenAI2ClaudeMessage_*`）、go vet 4× `common/custom-event.go`（passes lock by value）。`relay/helper TestStreamScannerHandler_EmptyBody` 为满载并行 flaky（隔离 ×3 全过）。middleware 10/10 过、model 全过。
- **待办**：多 agent 对抗式代码审查 + 容器端到端实测（settlement 申请→确认→账本、套现回归在线验证、P0 冒烟）。

### [2026-06-15 ~00:40] 多 agent 对抗审查 + 修复 5 个真实问题
- **做了什么**：2 个对抗式审查 agent（钱/结算 + P0 安全）深挖改动面，发现并修复以下真实问题：
  1. **[CRITICAL] SSRF 域名绕过**（我的码）：`controller/supplier_channel.go` `validateSupplierChannelBaseURL` 之前 `applyIPFilterForDomain=false`，导致域名 base_url（如 A 记录指向 169.254.169.254）不解析 DNS 即放行 → 改为 `true`。
  2. **[HIGH] 用户名/邮箱双倍暴破预算**（我的码）：`controller/user.go` 登录限流按原始输入计数 → 账号存在时改按「规范用户名」计数，username 与 email 共享同一计数；成功时两标识都清零。
  3. **[MEDIUM] Redis INCR/EXPIRE 非原子**（我的码）：`common/login_throttle.go` 每次自增都续期，避免 crash 后计数键永不过期。
  4. **[LOW] 账本回查失败则漏记**（我的码）：`controller/settlement.go` 确认即打款时，回查失败也用手上已知数据落账本（杜绝"已确认无账本"）。
  5. **[LOW] migrateDBFast 漏 SettlementLedger**（我的码）：`model/main.go` 补上（active 路径 migrateDB 本就有）。
- **改了哪些文件**：`controller/supplier_channel.go`、`controller/user.go`、`common/login_throttle.go`、`controller/settlement.go`、`model/main.go`。
- **验证**：`go build` 通过；`go test ./model/ ./common/ ./middleware/ -count=1` 全绿。
- **审查标记为"待办/follow-up"（本批未做）**：
  - ⚠️ **非文本流量结算 ¥0**（**预先存在**，非本次回归）：`official_usd` 仅在文本/对话路径（`service/text_quota.go`）计算；audio/realtime(wss)/异步任务(MJ/video/suno)/mjproxy 的消费日志 `official_usd=0`，故供应商这些模态流量结算为 ¥0。属 P4 "OfficialUsd 仅文本路径"已知限制。**需单独任务**把 official_usd + 快照扩展到全模态计费路径（`service/quota.go` PostWss/PostAudio、`service/task_billing.go`、`relay/mjproxy_handler.go`）。
  - SSRF 使用时 TOCTOU（DNS rebinding）：建链时再校验 / SSRF 防护 transport（DialContext 拒私网）——较大，follow-up。
  - 全局撞库天花板 + 失败触发验证码：函数已写但未接线（需前端 Turnstile 按需渲染）——P1。
  - Redis 宕机时限流 fail-open（DB 列仅兜底硬锁）——可改内存回退，follow-up。
  - 注册即供应商 + 自动登录的滥用速率限制——P1。

### [2026-06-15 ~00:50] 端到端实测（PG+Redis 运行容器，含全部修复）
- **做了什么**：重建镜像并重启容器（迁移已落：`logs.cost_price_snapshot` numeric 列 + `settlement_ledgers` 表）；走完整 HTTP 流程实测。
- **实测结果（全过）**：
  1. 注册自动登录：新供应商注册 → 返回 session cookie + `role:5` 数据 ✓
  2. **SSRF 拦截**：base_url `http://169.254.169.254` → 拒（private IP）；**域名 `localhost` → 解析到 `::1` → 拒**（域名修复生效）✓；空 base_url → 建渠道成功 ✓
  3. 待结算快照口径：seed 2 条(official 1.0/0.5 @ snapshot 2.0) → `payable_cny=3.0` ✓
  4. **套现回归（在线）**：把渠道成本价改到 10.0 → 待结算仍 3.0 → 发起结算 `computed_cny=3.0`（非 1.5×10=15）✓✓
  5. 全流程：供应商发起 → 超管确认(实付¥3) 成功 ✓
  6. **资金账本**：create(operator=供应商,is_admin=f) + confirm(operator=超管,is_admin=t,actual=3) 各一条，含 snapshot_hash ✓
  7. **防重复打款**：二次确认被拒（"状态已变更"），实付不被 999 覆盖，账本仍 2 条 ✓
  8. P0 登录冒烟：管理员登录正常（同密码校验路径）。
- **清理**：删除全部 livetest 测试用户/供应商/渠道/日志/结算/账本，0 残留；cookie jar 已删。
- **结论**：P0 + B + C **已彻底检查 + 测试**（静态/单测/对抗审查/端到端），发现的问题已修。D、E 按用户指示暂不做。
- **备注**：tksupplier1 旧测试账号 test12345 登录失败（密码漂移，非代码问题——管理员同路径正常）；如需可重置。




