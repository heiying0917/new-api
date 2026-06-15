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

---

## 2026-06-15 · v6 首页重做 + 登录注册 Tab 焦点

### [2026-06-15] v6 设计阶段：调研 + 多版 mockup + 设计方案（brainstorm，未改生产码）
- **做了什么**：
  - 调研 classic 主题：布局壳 `PageLayout`（全局固定 HeaderBar + 仅 /console 的 SiderBar + 全局 FooterBar）、`Home/index.jsx` 空内容分支=待替换的硬编码网关落地页、`StatusContext`←`/api/status` 状态通道、全站 Semi 令牌配色（无自定义主题包=Semi 默认蓝）、`context/Theme` 明暗跟随系统机制（`auto`+`prefers-color-scheme`）、设置页 `OtherSetting.jsx`。
  - 出 3 版整页 mockup（A 深色高端 / B 浅色 SaaS / C 科技渐变）→ 用户选 **B**。
  - 用 Playwright 从运行容器抓控制台真实 Semi 令牌（浅 `#0064FA`/深 `#54A9FF` 等）→ 精修 B（b2：真实令牌、浅+深、跟随系统+手动切换、克制蓝→靛蓝渐变）。
  - 出主色对比版（b3：当前蓝/深宝蓝/靛紫/祖母绿 实时切换）→ 用户选 **深宝蓝 Sapphire**（浅 `#2052CC`/深 `#7AA2FF`）作**全局主色**（首页+控制台一起换）。
  - 敲定：可配数字=后台可配 + 未配置时定性默认（不编造）；文案接 i18next；Tab 修复并入；落地页复用全局 Header/Footer（不自建 nav/footer，品牌红线自动保留）。
- **产出**：设计方案 `docs/superpowers/specs/2026-06-15-v6-home-redesign-design.md`；mockup `docs/superpowers/mockups/2026-06-15-home-v6/`（version-b2-refined / version-b3-primary-compare 等）。
- **下一步**：用户审 spec → 写实现计划 `plans/2026-06-15-v6-home-redesign.md` → 落地实现 + 测试。

### [2026-06-15 上午] v6 实现 + 浏览器充分验证（代码完成，**未提交**）
- **做了什么**：按实现计划落地全部 6 个任务（多 agent + 本人）。
  - 全局主色 → 深宝蓝 Sapphire：`index.css` 覆盖 Semi `--semi-color-primary/link` 系列（浅 `#2052CC`/深 `#7AA2FF`，两套），首页与控制台统一、明暗跟随系统。
  - 供应商招募落地页：新增 `web/classic/src/pages/Home/landing/`（`SupplierLanding/Hero/Advantages/Process/Security/Channels/CtaBand.jsx` + `landing.css`，全程 Semi 令牌取色、i18n、响应式），替换 `Home/index.jsx` 空内容分支；复用全局 Header/Footer（品牌红线 `New API` 自动保留）。
  - 可配数字：后端 `model/option.go` 注册 `HomeSupplierStats`、`controller/misc.go` GetStatus 只读透出；前端 Hero 解析、未配置回退定性默认（不编造数字）；`OtherSetting.jsx` 新增编辑卡片（JSON 校验 + 复用 updateOption）。
  - Tab 焦点：classic `LoginForm/RegisterForm` 用受控 `type` + 自定义后缀眼睛（`tabIndex=-1`）替换 Semi `mode='password'`；default `password-input.tsx` 眼睛加 `tabIndex=-1`。
- **改了哪些文件**：`web/classic/src/index.css`、`pages/Home/index.jsx` + `pages/Home/landing/*`（新增 8 文件）、`components/settings/OtherSetting.jsx`、`components/auth/{LoginForm,RegisterForm}.jsx`、`web/default/src/components/password-input.tsx`、`controller/misc.go`、`model/option.go`。
- **验证（运行容器 localhost:5001，本地交叉编译 linux/arm64 内嵌前端 + docker cp 部署）**：
  - 落地页：浅色/深色/移动端(390) Playwright 截图逐板块核对，全部 OK；信任条未配置时回退定性默认。
  - 主色：登录页/Dashboard/设置页 均为 Sapphire；明暗跟随系统正常。
  - Tab：登录页 `用户名→密码→继续`、注册页 `用户名→密码→确认密码→提交`，眼睛 `tabIndex=-1` 被跳过、鼠标点击仍可切明文（程序化核验 `modebtnDivs=0`、自定义眼睛 tabIndex=-1）。
  - 可配数字：管理员设置写入 JSON → 首页信任条实时联动（800+/99.9%/T+1/全链路）→ 清空回退默认，端到端打通。
  - 后端回归：`go build ./...` 通过；`go test ./model/ ./controller/` 118 passed，唯一失败 `TestListModelsTokenLimitIncludesTieredBillingModel` 为**预先存在**、与本次无关。
- **踩坑记录**：① Docker 首次 `--build` 缓存了 go build 层（前端 `go:embed` 进二进制，改动没生效）；`--no-cache` 重建失败 → 改用本地交叉编译 + `docker cp` 部署，绕开 Docker 构建。② **classic 用 rsbuild,其持久缓存 `node_modules/.cache` 漏编 RegisterForm**（登录新、注册旧的诡异不一致根因）；`rm -rf node_modules/.cache` 后入口哈希变化、注册改动才真正编入。**正式 Docker 镜像构建是全新容器、`bun install` 全新,无此缓存问题。**
- **交付状态**：运行容器已是新代码（cp 二进制存活于容器可写层，`docker restart` 可保持；若 `docker compose up` 重建容器需重新 build 镜像）。`HomeSupplierStats` 已清空=定性默认。**未 git commit / 未 push / 未重建发布镜像**——等用户验收后明确指令。

---

## 2026-06-15 · 提权面安全审计 + 纵深防御（应用户「确保无法注入提权」要求）

### [2026-06-15] 越权/提权审计（3 路对抗 agent + 本人逐行复核，结论：无提权路径）
- **背景**：用户转来同行被盗 key 案例——「`PUT /api/channel/` 只要 `AdminAuth()`(role≥10)，对方把管理员账号发给客户 → 客户改 base_url 收割上游 key」(CWE-862)。要求确认我们供应商(role=5)无法拿别人 key、无法提权。
- **做了什么**：① 亲手扒鉴权链（`middleware/auth.go` `authHelper`：role 从缓存实时回查、`role<minRole` 拒）；② 供应商自助控制器逐函数核对归属（`controller/supplier_channel.go` 每个写/读先 `GetChannelById` 再 `SupplierId != 自己 → forbidden`；列表/搜索 `model/channel.go:1138/1152` `Where(supplier_id=?)`+`.Omit("key")`）；③ 派 3 个只读 agent 分别扫 OAuth/passkey 建号 role、所有 role≥5 可达的 users 写入口、raw SQL 注入 + 全部 Insert/Update 调用方。
- **结论（三方独立一致）**：**当前不存在任何注入/旁路提权路径**。`Register`/`UpdateSelf`/`CreateUser` 均用字段白名单 `cleanUser`（role 硬编码或 DB 校验封顶）；`UpdateUser` 走 `Edit` 白名单 map（不含 role/status）；`ManageUser` 的 `promote` 仅超管、`canManageTargetRole` 封顶；供应商可达路径 SQL 全参数化、`ORDER BY` 走 `channelSortColumns` 白名单。**唯一铁律仍是运营层：供应商/客户账号一律 role=5，绝不发 role≥10。**

### [2026-06-15] 纵深防御代码 + 回归测试（**未提交**）
- **做了什么**：
  - **修一处卫生缺口**：`controller/discord.go`、`controller/oidc.go` 新建 OAuth 用户此前没显式设角色（靠 GORM `default:1` 兜底，fail-safe 但隐式）→ 显式 `user.Role = common.RoleCommonUser` + `user.Status = common.UserStatusEnabled`，与 GitHub/WeChat/LinuxDo 一致。
  - **加 4 个回归测试**把「注入提权被拒」永久钉死：
    - `model/user_privesc_test.go`：`TestEdit_DoesNotEscalateRoleStatusQuota`（Edit 白名单不改 role/status/quota）、`TestUpdate_CleanStructLeavesPrivilegeFieldsIntact`（复刻 UpdateSelf 的 cleanUser→Update 机制，零值字段被 GORM 跳过）。
    - `controller/user_privesc_test.go`：`TestRegister_IgnoresInjectedRoleStatusQuota`（httptest+session 端到端 POST `{"role":100,"status":1,"quota":999999,"group":"root"}` → 落库恒为供应商(5)/默认额度/非 root 分组）、`TestCanManageTargetRole`（角色层级不变式）。
- **改了哪些文件**：`controller/discord.go`、`controller/oidc.go`、`model/user_privesc_test.go`（新增）、`controller/user_privesc_test.go`（新增）。
- **验证**：`go build ./controller/... ./model/...` 通过；4 个新测试全过；`go test ./model/ ./controller/` = 122 passed / 1 failed（`TestListModelsTokenLimitIncludesTieredBillingModel`，隔离 `-count=1` 单跑亦 nil 指针 panic，**预先存在、与本次无关**）/ 2 skipped，**零新增失败**。
- **未做（用户当前未要求）**：D 密钥读时打码 / E 官key 静态加密（属「降低泄露后果」，非提权面）；恶意供应商把自己渠道 base_url 指向外部服务器截获终端请求体（市场固有风险，靠准入审核兜底，SSRF 已堵内网）。**未 commit / 未 push。**
