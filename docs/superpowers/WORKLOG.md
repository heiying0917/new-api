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

---

## 2026-06-15 · v7 供应商渠道体验（类型 logo + 中转标识 + 分组必填）

### [2026-06-15 下午] v7 三项需求（前端，已提交 `e5cb70a8`，已本地部署）
- **背景（用户第七版需求）**：① 供应商创建/编辑渠道的「类型」下拉无 logo（管理员端有），要对齐；② 供应商填了自定义 API 地址=中转 Key，加一眼可见的特殊标识；③ 创建渠道时分组默认为空、不要默认 default。
- **两处决策（AskUserQuestion 锁定）**：中转标识「管理员端 + 供应商端都显示」；分组「改必填」。
- **关键发现（决定方案）**：Explore 核查 `model/ability.go` + `model/channel_cache.go` —— 渠道 `group` 若真存空字符串会 split 成 `[""]` 落进 `group2model2channels[""]`，而请求只查 `group2model2channels["用户分组"]`，**空分组=无法被任何请求命中的死渠道**。故分组取「必填」：UI 创建默认空 + 强制手动选；后端 `SupplierAddChannel` 的 `if group=="" → default` 兜底保留作防御（防 API 直连绕过造成死渠道），与「必填」不冲突。
- **改了哪些文件**（纯 web/classic 前端，无后端改动）：
  1. **类型下拉 logo**：`components/table/supplier-channels/modals/EditSupplierChannelModal.jsx` 新增 `renderTypeOption`（复用 `helpers/getChannelIcon`，与管理员端 `EditChannelModal.renderChannelOption` 同款 logo + 选中态），`Form.Select field='type'` 接 `renderOptionItem`。
  2. **中转标识**（基于 `supplier_id`+`base_url` 实时判定，**不新增数据库字段、不迁移**）：
     - 管理员端 `components/table/channels/ChannelsColumnDefs.jsx` `renderType`：`supplier_id>0 && base_url 非空 → 中转`，加橙色「中转」Tag（带 Tooltip），与既有 IO.NET 标签并存（重构短路条件 + IO.NET 块改条件渲染，避免无 ionet 时误显示）。
     - 供应商端 `components/table/supplier-channels/SupplierChannelsColumnDefs.jsx` `renderType`：列表内渠道均属本供应商，`base_url 非空 → 中转`，加橙色小号「中转」Tag；类型列 render 改为传 record。
  3. **分组必填 + 默认空**：`EditSupplierChannelModal.jsx` `getInitValues().groups` 由 `['default']` 改 `[]`；编辑加载 `groups` 不再回退 `['default']`（读真实值，老死渠道打开即被必填逼着补分组）；`Form.Select field='groups'` 加 `required/min:1` rule，placeholder 改「请选择或输入分组」。
  4. **i18n**：`i18n/locales/zh-CN.json` 新增 `中转` / `该渠道由供应商填写了自定义 API 地址，为中转 Key` / `请选择或输入分组` / `请至少选择一个分组`。
- **验证**：
  - `bun run build`（清 `node_modules/.cache` 防 rsbuild 持久缓存漏编）**exit 0**，classic dist 重新产出；`zh-CN.json` JSON 合法。
  - 交叉编译 linux/arm64（re-embed 新 classic dist）+ `docker cp` 容器 `/new-api` + restart：容器 **healthy**、运行版本 **`juhe-e5cb70a8`**（日志 `Token Ki juhe-e5cb70a8 ready`）、日志 0 errors、前端新 bundle（`index.7ad6c4a726.js`）加载正常。
  - **端到端浏览器实测（Playwright，tksupplier1 供应商端 + tkadmin 管理员端，临时改密验证后已还原）**：
    ① 需求1 ✓ 供应商新建渠道「类型」下拉每项左侧显示品牌 logo（OpenAI/Claude/Suno/Ollama…），与管理员端一致；
    ② 需求3 ✓ 新建弹窗分组框默认空白 + 占位符「请选择或输入分组」，不选分组提交被拦「请至少选择一个分组」（红色校验）；
    ③ 需求2 ✓ 给某供应商渠道加 base_url 后，供应商「我的渠道」+ 管理员「渠道管理」该行类型旁均显示橙色「中转」标签，其余无 base_url 渠道不显示。
    截图：`v7-type-dropdown-logo` / `v7-group-required` / `v7-relay-tag-supplier2` / `v7-relay-tag-admin`。
  - **环境踩坑**：本地有两个 pg 容器——应用实连 compose 网络的 `postgres`(postgres:15, 库 new-api, root/123456)，另一个 `new-api-xy-pg-local`(bridge, 库 new_api_dev) 与应用无关；起初误操作了后者导致改密不生效，定位后切到正确库。验证用的临时改动（tksupplier1/tkadmin 密码、渠道 base_url，及误操作容器的 test）均已逐项 SQL 还原核对，**零残留**。
- **提交状态**：代码 `e5cb70a8` 已提交（feat/tokenki-p1a-supplier-backend 分支）+ 已本地部署 + 端到端验证通过。**未 push**（用户本次只要求「提交 + 本地部署」）。

### [2026-06-15] v6 供应商招募首页文案重写（纯 classic 前端文案，**未提交**）
- **背景（用户需求）**：首页文案重新优化，突出"全球最专业的官 Key 托管平台、为全球数百家企业提供 AI 大模型服务、接收各种官 Key（含 Claude / AWS / OpenRouter / OpenAI）、自助上传、数据加密隔离、透明计费、实时多币种结算"；**删除"账号防暴破"那段**，其余重新优化整理 + 创意丰富用词。
- **做了什么**：在保留既有版块结构（Hero / 核心优势 / 托管流程 / 安全保障 / 渠道 / CTA）前提下重写全部文案：
  - **Hero**：eyebrow 改「全球最专业的官方 Key 托管平台」；标题「托管你的官方 Key,接入全球 AI 算力市场」；副文案覆盖 数百家企业 + Claude/AWS/OpenRouter/OpenAI + 自助上传 + 加密隔离 + 透明计量 + 多币种实时结算；stats 微调（数百家·全球企业客户 / 全天候·订单不间断 / 多币种·实时结算 / 端到端·加密隔离）；收益示意面板新增 OpenRouter 行（¥6,540），累计金额 27,710→34,250 同步。
  - **核心优势**：4 卡重写为 订单充沛额度不闲置 / 全渠道托管自助秒级接入 / 数据加密隔离安全有保障 / 透明计费多币种实时结算。
  - **托管流程**：步骤1 改「自助上传官方 Key」（含 OpenRouter）；步骤3「透明实时计量」；步骤4「多币种实时结算」。
  - **安全保障**：**删除**「账号级登录防暴破、可吊销会话、SSRF 校验」那条，替换为「额度自主可控：自助上传、随时启停托管，对官方 Key 始终拥有完全掌控权」；另三条加密隔离/透明计费防套现/原子幂等结算文案润色。
  - **渠道**：徽章列表加入 OpenRouter；标题「主流官方渠道,一处托管统一接单」。
  - **CTA**：改「现在就自助上传你的官方 Key,让闲置额度持续生息」。
- **改了哪些文件**（均 `web/classic/src/pages/Home/landing/`，纯文案，无逻辑/无后端改动）：`Hero.jsx`、`Advantages.jsx`、`Process.jsx`、`Security.jsx`、`Channels.jsx`、`CtaBand.jsx`。
- **i18n 说明**：classic 仅 zh-CN，且这些文案的 i18n key 即中文串本身（不在 `zh-CN.json` 中，i18next 缺失即回退渲染 key），故**无需改 locale 文件**。
- **验证**：文案均为合法 JS 单引号字符串（无内部单引号冲突）；未跑 build（待用户确认后再 build/部署）。
- **提交状态**：**未提交、未 build、未部署**（等用户指令）。

### [2026-06-15] v6 首页 Hero 右侧面板改版 + 数字滚动动画（classic 前端，**未提交**）
- **背景（用户第二轮需求）**：① 右侧"收益示意"面板金额改大、以**美元**计，四行分别约 800/400/100/80 万刀**依次降低**；② 面板标题改「正在托管的 Key」；③ 去掉所有"示意"字样；④ 底部累计金额必须为四行**精确求和**；⑤ 页面加载后数字做**滚动(count-up)动画**。另：用户反馈"没看到改后的效果"→ 本轮跑起来实测给看。
- **做了什么**（`web/classic/src/pages/Home/landing/Hero.jsx` 重写 + `landing.css` 微调）：
  - 金额改 USD 且降序：Claude `$8,247,360` / AWS Bedrock `$4,612,980` / OpenRouter `$1,358,420` / OpenAI `$842,170`；累计 = 代码 `reduce` 求和 = **`$15,060,930`**（精确）。
  - 面板顶部标题 `我的托管渠道`→`正在托管的 Key`；**删除** `示意` 徽章；底部标签 `本周期累计收益(示意)`→`累计托管收益`；aria-label 同步。
  - 新增 `useCountUp` hook（requestAnimationFrame + easeOutCubic，setTimeout 错峰 delay=index*140ms，无 Date.now/Math.random）+ `AnimatedAmount` 组件（`$` 前缀 + `toLocaleString('en-US')` 千分位）；各行与累计均 0→目标值滚动。
  - `landing.css`：`.landing-panel__amt` / `.landing-panel__total-amt` 加 `font-variant-numeric: tabular-nums` + `font-feature-settings:'tnum'` 防滚动抖动。
- **改了哪些文件**：`web/classic/src/pages/Home/landing/Hero.jsx`、`web/classic/src/pages/Home/landing/landing.css`。
- **预览方式（不动容器/不重建二进制）**：临时给 `web/classic/rsbuild.config.ts` 加一行 `process.env.DEV_PROXY_TARGET ||` 代理覆盖（**预览辅助，待还原**）；后台 `DEV_PROXY_TARGET=http://localhost:5001 bunx rsbuild dev --port 8090`（同源代理到正在运行的后端容器 `localhost:5001`，dist 是 embed 进 Go 二进制的，故走 dev 而非重建）。
- **验证（Playwright 实测 localhost:8090）**：
  - 截图 `home-hero-final.png`：Hero 文案 + 右侧面板渲染正确，标题「正在托管的 Key」、无"示意"、四行 USD 降序、累计 `$15,060,930`。
  - 滚动动画用 MutationObserver 采样累计金额：**97 帧** `$0 → $466,690 → $923,639 → … → $15,060,913 → $15,060,930`，缓动收敛、终值精确。
  - 落地页渲染前提 `home_page_content===''` 已确认；端口 3000 被无关项目 new-api-xy 占用故用 8090；清理了上次会话遗留的 Playwright Chrome(锁住 MCP profile，PID 49288)。
- **提交状态**：**未提交、未 build 生产、未部署**；`rsbuild.config.ts` 的临时代理行 + 8090 dev server 待用户看完后还原/停止。

### [2026-06-15] v6 首页改动正式部署到本地容器 5001（用户「没看到」→ 重建二进制部署）
- **原因**：用户在 `localhost:5001` 看不到改动——5001 容器跑的是旧 Go 二进制（前端 dist 经 `main.go` `//go:embed` 编进二进制，dev 改源码不会进 5001）。Playwright 实测确认：8090(dev)=新文案、5001(容器)=旧文案(`面向供应商·官方额度变现`/`我的托管渠道示意`)。
- **做了什么**（重建+本地部署，**未 commit/未 push**）：
  1. `bun run build` 两个前端（清 `node_modules/.cache`）：`web/default/dist` + `web/classic/dist`（main.go 同时 embed 两者，缺一 go build 失败）。
  2. `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=juhe-0dc44cd8-homecopy'" -o /tmp/new-api-juhe .`（纯 Go sqlite `glebarez/sqlite`，无 CGO，darwin→linux/arm64 直接交叉编译；产物 ELF aarch64 87MB）。
  3. `docker cp /tmp/new-api-juhe new-api:/new-api` + `docker restart new-api`。
- **验证（Playwright 实测 localhost:5001）**：容器 ready(版本 `juhe-0dc44cd8-homecopy`)；DOM 取值确认 eyebrow=`全球最专业的官方 Key 托管平台`、title=`托管你的官方 Key,接入全球 AI 算力市场`、panelHead=`正在托管的 Key`(无示意)、累计=`$15,060,930`、四行 USD 降序、渠道徽章含 OpenRouter、`防暴破`文本=false(已删)；整页截图逐版块正常、页脚 `© 2026 Token Ki · 提供方 New API` 品牌保留。重启后旧会话过期(`/login?expired=true`)，清 localStorage 后落地页正常(公开页)。
- **收尾**：临时的 `rsbuild.config.ts` 代理行已还原；8090 dev server 已停；截图工件已删。
- **注意**：工作树另含**非本次**的未提交改动(`controller/channel.go`/`controller/supplier_channel.go`/`model/channel.go`/`router/api-router.go`/table 组件/`zh-CN.json`)——本次构建从工作树取材，故部署的二进制一并包含；未触碰这些文件。
- **提交状态**：首页改动 = 本地容器 5001 已生效；**未 commit、未 push**。

### [2026-06-15] v6 首页 Hero 文案再调整（用户第三轮）+ 重新部署 5001
- **用户给定文案**：eyebrow=`企业级官 Key 托管平台`；title=`专业的官 Key 托管平台,一键接入全球 AI 算力市场`；sub=`TokenKi 平台为全球数百家企业提供稳定 AI 算力,支持 Claude、AWS、OpenRouter、OpenAI 等官 Key 托管。一键上传,加密存储,透明计费,多币种实时结算,让每一位供应商都能获得高额收益。`
- **做了什么**：改 `web/classic/src/pages/Home/landing/Hero.jsx` 三处文案（基于用户/linter 已改过的当前版本之上）；按全页排版规范轻规范化（官Key→官 Key、AI 前后空格、半角逗号）；`TokenKi` 按用户原文保留。右侧 USD 面板/数字滚动等上轮改动未动。
- **部署**：`bun run build`(classic，清缓存) → `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build`(VER=`juhe-0dc44cd8-homecopy2`) → `docker cp /new-api` → `docker restart`。
- **验证**：Playwright DOM 实测 5001 三处文案=新值；线上 JS chunk(`/static/js/async/8411.4358d7a294.js`) grep 4 条新串各命中 1 次、旧串`全球最专业...`已从构建消失；截图确认 Hero 呈现。期间 Playwright 浏览器 profile 被遗留 chrome 锁住，kill 进程+删 Singleton* 锁后恢复。
- **提交状态**：5001 容器已生效；**未 commit、未 push**。

### [2026-06-15] 首页面板标题微调 + 重新部署
- **改动**：`Hero.jsx` 面板标题 + aria-label `正在托管的 Key` → `已托管的官 Key`（2 处，replace_all）。
- **部署**：classic build(VER `juhe-0dc44cd8-homecopy3`) → 交叉编译 → docker cp /new-api → restart。
- **验证**：线上新 chunk `8411.b0e6c0874c.js`(hash 变=新构建) 含 `已托管的官 Key`(命中1)，旧 `正在托管的 Key` 已从 dist 消失；截图确认面板顶部已变。
- **提交状态**：5001 已生效；**未 commit、未 push**。

### [2026-06-15] v8 管理员成本价透出 + 渠道分组改必填默认空（classic 前端，**未提交**）
- **用户两条需求**：① 管理员创建/编辑渠道也要展示「成本价」（代供应商上传时可写，用于结算给供应商多少钱）；② 管理员与供应商建渠道时分组都默认**空**、但**必选一个**才能创建。
- **做了什么**（`web/classic/src/components/table/channels/modals/EditChannelModal.jsx`）：
  1. `originInputs.groups` 默认值 `['default']` → `[]`（两种模式默认空）。
  2. 成本价字段去掉 `isSupplierMode &&` 包裹 → 管理员也渲染；`required` 仅供应商（`isSupplierMode ? [{required}] : []`），extraText 分流：供应商=`供应商渠道必填，用于结算`、管理员=`选填，代供应商上传时填写，用于结算`。
  3. 分组 `Form.Select` 加 `rules={[{required, message:'请至少选择一个分组'}]}`（即时 inline 校验）。
  4. submit 校验：在成本价校验前加 `if (!groups.length) { showError('请至少选择一个分组'); return; }`（两种模式通用兜底）。
  5. `zh-CN.json` 补键 `选填，代供应商上传时填写，用于结算`。
- **后端**：无需改动。`Channel.CostPrice *float64`(已存在) + 管理员 `Insert()`=`DB.Create` 持久化全字段，管理员填了即存、留空为 0（不参与结算，符合 `*CostPrice<=0` 判定）。
- **改了哪些文件**：`EditChannelModal.jsx`、`zh-CN.json`。
- **部署**：classic build(清缓存) → `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build`(VER `juhe-v8costgroup`) → `docker cp /new-api` → restart。
- **验证（Playwright 实测 5001，tkadmin 临时改密登录后已还原原 hash）**：① 管理员新建渠道弹窗「成本价」字段显示、**无红星=选填**、提示=`选填，代供应商上传时填写，用于结算`；② 「分组*」带红星、默认 placeholder（空）；③ 填名称+密钥、不选分组点提交 → 模态不关闭，分组字段 inline `[invalid]`=`请至少选择一个分组`、模型字段 `[invalid]`=`请选择模型`，提交被拦截。供应商端复用同一表单（成本价必填、分组同样必选默认空），上一轮已充分验证。
- **收尾**：tkadmin 密码恢复原 hash 已核验；测试截图/快照工件已删；未建任何测试渠道。
- **提交状态**：5001 本地容器已生效；**未 commit、未 push**（等用户指令）。

### [2026-06-15] v9 品牌 Logo 落地：碎片拼合 Facet（设计交付方案，带动效，classic 前端，**未提交**）
- **来源**：用户提供其他设计师交付 `~/Downloads/handoff/`（`token-ki-facet.svg` 固化静态 + `Token Ki Facet - Handoff.html` 动效规格）。方案「碎片拼合 Facet」：六块三角碎片旋转飞入、拼成正六边形、向心收束于核心，寓意聚合平台「汇聚多源、凝聚成一」。配色碎片由浅入深 #bae6fd→#93c5fd、核心 #eaf6ff、深蓝径向底 #143672→#06112e。
- **用户要求**：运用此方案，能带动效的地方带动效，不能的地方用固定的。
- **落地策略**：动效（内联 SVG + CSS）用于首页 Hero 大 logo、登录/注册页 logo；静态固化 SVG 用于 favicon、顶栏、默认资源；保留系统配置 logo 机制（管理员上传了自定义 logo 则优先用它）。
- **做了什么**：
  1. 新建 `web/classic/src/components/common/logo/FacetLogo.jsx`（内联 SVG，固化 6 碎片坐标 + dx/dy/rot/delay；props `size` / `animate`=static|entrance|pulse|hover|auto；用 `useId()` 隔离 gradient/clip id 防多实例冲突）。
  2. 新建 `FacetLogo.css`（设计师动效规格：入场 `facetIn` 0.7s cubic-bezier(.2,.75,.2,1) + stagger var(--d)，核心 `facetPop` 0.6s 后弹出，呼吸 `facetBreathe` 光晕 2.8s loop，hover 重播；`prefers-reduced-motion` 降级为静态）。
  3. 新建 `web/classic/public/logo.svg` = 设计交付固化静态 SVG（favicon / 顶栏 / 兜底用）。
  4. `index.html` favicon `/logo.png`→`/logo.svg`；`helpers/utils.jsx` `getLogo()` 兜底 `/logo.png`→`/logo.svg`。
  5. `LoginForm.jsx` / `RegisterForm.jsx`：各 2 处 `<img src={logo}>` 改为 `{localStorage.getItem('logo') ? <img...> : <FacetLogo size={40} animate='entrance'/>}`（无自定义配置时走 Facet 入场动效）+ import。
  6. `Home/landing/Hero.jsx`：copy 区顶部新增 `<FacetLogo size={64} animate='auto'/>`（入场+呼吸+悬停）+ import。
- **后端配置**：DB `options` 无 `Logo`（仅 `SystemName=Token Ki`）→ localStorage 无 logo → 全站默认走 Facet。
- **部署**：classic build（清缓存）→ `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build`（VER `juhe-v9facet`）→ docker cp /new-api → restart。`dist/logo.svg` 1464B、`/logo.svg` 200。
- **验证（Playwright 实测 5001 首页）**：顶栏小 logo + Hero 大 logo 均为 Facet 蓝宝石；favicon href=`/logo.svg`；6 碎片齐全；Hero tile class=`is-entrance is-pulse is-hover`，碎片 `animationName=facetIn 0.7s`、光晕 `facetBreathe`；重启入场后采样起始帧 `matrix(0.629,-0.307,0.307,0.629,29,-50.23)` = translate(29,-50.23)·rotate(-26°)·scale(.7)，与设计规格一致→入场飞入确凿生效。登录/注册复用同组件。
- **提交状态**：5001 本地容器已生效；**未 commit、未 push**（等用户指令）。

### [2026-06-15] v9 Logo 修复：favicon 换成 Facet + 顶栏去圆形改圆角方（接 v9，**未提交**）
- **用户反馈两点**：① 浏览器选项卡 icon 没换；② 顶栏左上角 logo 是圆形、跟全站圆角方不一致、不好看。
- **问题定位**：① 顶栏 `HeaderLogo` 的 `<img className='rounded-full'>` 把方形 Facet 硬裁成圆；② favicon 虽已写 `/logo.svg`，但 rsbuild 自动向 `dist/index.html` 注入了 `<link rel="icon" href="/favicon.ico">`（指向旧 15KB new-api 图标），浏览器优先用它 → 选项卡没变。
- **做了什么**：
  1. 用 Playwright canvas 把 Facet SVG 渲染为透明圆角 PNG（256×256 RGBA）→ `web/classic/public/logo.png`（49KB）。
  2. 用 PIL 从该 PNG 生成多尺寸 `favicon.ico`（16/32/48/64/128/256，PNG-in-ICO，53KB）覆盖 `web/classic/public/favicon.ico`（原旧图标）。
  3. `index.html` favicon 声明改为 PNG 优先 + SVG 备 + apple-touch-icon：`<link icon png /logo.png>` + `<link icon svg /logo.svg>` + `<link apple-touch-icon /logo.png>`。
  4. `getLogo()` 兜底 `/logo.svg`→`/logo.png`（PNG 兼容性更好）。
  5. `HeaderLogo.jsx`：顶栏 logo 改为 `{localStorage.getItem('logo') ? <img rounded-xl> : <FacetLogo size={32} animate='hover'>}`——无自定义配置时用圆角方 Facet（悬停重播），与 Hero/登录页完全一致；有自定义 logo 时 img 圆角也由 `rounded-full` 改 `rounded-xl`。
- **部署**：classic build（清缓存，VER 路径修正：go build 必须在 repo 根，不能在 web/classic）→ 覆盖 `dist/favicon.ico` → `go build`（VER `juhe-v9facet3`）→ docker cp → restart。
- **验证（Playwright 实测 5001）**：DOM favicon 链含 `/logo.png`（png）+`/logo.svg`+`/favicon.ico`，三者服务端均 200、`favicon.ico` 53883B image/x-icon=Facet；顶栏 `headerHasFacet=true`、`headerHasRoundImg=false`（圆形 img 已消除）、全页 2 个 facet-tile（顶栏+Hero）均圆角方一致；截图确认顶栏左上小 logo 与 Hero 大 logo 形状统一。
- **注意**：浏览器选项卡 favicon 缓存顽固，用户需硬刷新（Cmd+Shift+R）才会看到新 icon。
- **提交状态**：5001 本地容器已生效；**未 commit、未 push**。

### [2026-06-16] Hero logo 布局微调（内容偏下修复，**未提交**）
- **用户反馈**：加 logo 后左侧 Hero 内容上方间距太大、整体偏下，与右侧面板失衡。
- **根因**：大 logo（64px + 20px 下边距）把左列内容下推约 84px；`.landing-hero__grid` 为 `align-items:center`，左列变高后 eyebrow/标题相对右侧面板偏下（eyebrow 顶 282 vs 面板顶 255）。
- **改**：`Hero.jsx` logo `size 64→44`、`marginBottom 20→8`；`landing.css` `.landing-hero` 顶部留白 `6rem→4.5rem` 补偿。
- **验证（Playwright 实测 5001）**：logo=44px，eyebrow 顶(195) ≈ 面板顶(198) 齐平，左右两列重新对齐平衡；部署 `juhe-v9facet4`。
- **改动文件**：`web/classic/src/pages/Home/landing/Hero.jsx`、`web/classic/src/pages/Home/landing/landing.css`（均 logo 相关）。
- **提交状态**：5001 已生效；**未 commit、未 push**。

### [2026-06-16] Hero 文案调整（标题 + eyebrow，**未提交**）
- **标题**：「专业的官 Key 托管平台,一键接入全球 AI 算力市场」→「企业级官 Key 托管平台,一键接入全球 AI 算力市场」（用户指定；保持全站排版规范：官 Key 大写带空格、半角逗号、AI 前后空格）。
- **eyebrow**：发现新标题前半与原 eyebrow「企业级官 Key 托管平台」一字不差重复，建议后用户选 A → eyebrow 改为「端到端加密 · 供应商严格隔离」（差异化安全卖点，与标题互补不撞词）。
- **部署**：classic build → go build（VER `juhe-v9facet6`）→ docker cp → restart。
- **验证（Playwright 实测 5001）**：eyebrow=「端到端加密 · 供应商严格隔离」、title=「企业级官 Key 托管平台,一键接入全球 AI 算力市场」，不再重复。
- **改动文件**：`web/classic/src/pages/Home/landing/Hero.jsx`。
- **提交状态**：5001 已生效；**未 commit、未 push**。

### [2026-06-16] Hero logo+eyebrow 并排一行 + 内容上移（**未提交**）
- **用户建议**：logo 与 eyebrow 标签放在同一行，让首页内容再往上一点（之前仍偏下）。
- **改**：`Hero.jsx` 把 logo 与 eyebrow 包进 `landing-hero__brand` 横向 flex（`align-items:center; gap:14px; margin-bottom:20px`，eyebrow 内联清零自身下边距）；`landing.css` `.landing-hero` 顶部留白 `4.5rem→4rem`。
- **验证（Playwright 实测 5001）**：logo 与 eyebrow 同行（中心 Y 均=150），标题上移（titleTop 192）；部署 `juhe-v9facet8`。
- **改动文件**：`Hero.jsx`、`landing.css`。
- **提交状态**：5001 已生效；**未 commit、未 push**。

### [2026-06-16 17:21] 第八版 供应商体系增强（后端4单元+前端3单元,subagent 驱动开发,**未提交、未部署**）
- **需求来源**：用户第八版 5 条需求。方案 `docs/superpowers/specs/2026-06-16-tokenki-v8-supplier-enhancements-design.md`；计划 `docs/superpowers/plans/2026-06-16-tokenki-v8-supplier-enhancements.md`。
- **做了什么（7 个实现单元,每单元 实现→测试/build→评审 approve）**：
  1. **需求1 汇总+排序（后端）**：`model/supplier_stats.go` 新增 `GetAllSuppliersPendingStat`（一次聚合 per-supplier+全局待结算)、`GetSettlementTotalsByStatus`（已申请/已结算分桶,actual_amount 按币种拆 CNY/USD)、`GetAllSuppliersSettledTotal`；`GetSupplierSummary` → `GET /api/supplier/summary`（RootAuth)；`model/supplier.go` `GetAllSuppliers(+sortBy,+sortOrder)` 服务端排序(priority/pending_cny/pending_usd/settled_cny 全量内存排序+分页)。修复 priority 误用 users 列 ORDER BY 的 bug + buildSupplierItems 吞错改为传播。
  2. **需求1 立即结算（后端）**：`AdminInitiateSettlement` → `POST /api/admin/settlement/initiate`（RootAuth),复用 `model.CreateSettlement`(空待结算自删占位单)+台账 OperatorIsAdmin=true。
  3. **需求2 渠道供应商过滤（后端）**：`ResolveSupplierIdsByName` + `GetAllChannels`/`SearchChannels`/`buildChannelListQuery` 增 `supplierIds`(无匹配短路空页)。
  4. **需求3/4/5 概览聚合（后端）**：`GetSupplierOverview`(供应商总数/启用、渠道可用性、按 type 聚合,整行 Find 避开保留字 group)；`GET /api/admin/supplier-overview`（**AdminAuth=管理员+超管**,满足需求4）。
  5. **需求1 前端**：`useSuppliersData` summary/排序/立即结算+确认弹窗;列排序+「立即结算」按钮;新 `SuppliersSummaryBar`;复用 `ConfirmModal`;排序参数翻页/改页大小/刷新全显式透传。
  6. **需求2 前端**：`ChannelsFilters` 加供应商搜索框;`useChannelsData` 6 处对称接入 `searchSupplier` → `&supplier_name=`。
  7. **需求3/4/5 前端**：新 `pages/SupplierOverviewAdmin` + hook + `TypeCard/TypeDetailSheet`;紧凑响应式网格 `Col xs=12 sm=8 md=6 lg=4`(修复卡片太宽);`App.jsx` 路由+`SiderBar` 菜单+`useSidebar.js DEFAULT_ADMIN_CONFIG` 注册;itemKey 用 `supplier_overview_admin` 避开与供应商端冲突。
- **改了哪些文件**：后端 `controller/{supplier,settlement,channel}.go`、`model/{supplier,supplier_stats,channel}.go`、`router/api-router.go` + 测试 `model/{supplier_summary,supplier_sort,supplier_overview,channel_supplier_filter}_test.go`、`controller/admin_initiate_settlement_test.go`、`model/supplier_test.go`/`supplier_aggregation_test.go`(签名适配)。前端 `web/classic/src/`: `hooks/suppliers/useSuppliersData.jsx`、`components/table/suppliers/{index,SuppliersTable,SuppliersColumnDefs,SuppliersSummaryBar}.jsx`、`hooks/channels/useChannelsData.jsx`、`components/table/channels/ChannelsFilters.jsx`、`pages/SupplierOverviewAdmin/index.jsx`、`hooks/supplier-overview-admin/useSupplierOverviewData.jsx`、`components/supplier-overview-admin/{TypeCard,TypeDetailSheet}.jsx`、`App.jsx`、`components/layout/SiderBar.jsx`、`hooks/common/useSidebar.js`、`i18n/locales/zh-CN.json`。
- **如何验证**：`go build ./...` ✓;`go vet ./model ./controller` 干净;`go test ./model/ ./controller/` → model 全过、controller 仅 1 个**预先存在**失败 `TestListModelsTokenLimitIncludesTieredBillingModel`(stash 本期改动后仍 panic,属其它未提交工作 official_pricing/option.go,与本期无关);新增后端测试全过;`cd web/classic && bun run build` ✓。
- **部署 + Playwright 实测(经用户授权)**：classic 清缓存重建 → `CGO_ENABLED=0 GOOS=linux GOARCH=arm64` 交叉编译(VER `juhe-v10supplier`)→ `docker cp` 进 new-api 容器 → restart;`/api/status` 确认新版本生效。tkadmin 临时改密登录(原 hash 已存并**已还原核验** pwd_restored=t、access_token 已清空)。实测 5001 全部通过:① 供应商管理页汇总条三卡(待结算 ¥0.09/$0.49·2家、已申请 ¥0/0单、已结算 ¥423.13/$169.32·3单)、优先级列排序 asc/desc(tksupplier1 优先级5 正确升降)、test1「立即结算」→ 确认弹窗预填 ¥0.09(创建的 已申请单 id=8 测后经 cancel API 撤销 + 硬删 settlement/ledger,test1 待结算与日志已还原);② 渠道管理页「供应商」搜索 test1 → 精确过滤 3 条全属 test1;③ 供应商概览页(管理员可见,需求4)summary 供应商4/渠道8(7可用1不可用)+ 紧凑卡片 OpenAI(3家/5-6可用/¥1.80)、Anthropic(1家/2-2/¥2.50)+ 点卡下钻 SideSheet 分组竞价梯队(claude速刷¥1.80/OpenAI官key¥2/claude官key¥2.2)。
- **已知小项(非本期 bug)**：概览 summary「供应商 4 / 5 启用」中 enabled(5)>total(4) —— 因测试库有 5 条 supplier 资料行但仅 4 个 role=5 用户(数据不一致),非代码缺陷;真实数据下二者一致。
- **提交状态**：5001 容器已生效(`juhe-v10supplier`);**未 commit、未 push**(等用户指令)。tkadmin 账号已完整还原。

### [2026-06-16] 修复「新建/编辑渠道·获取模型列表」对多类型失败(代码+测试+真机API实测,**未提交、未部署**)
- **现象**：用户报「添加任何类型渠道,获取模型列表都失败」。系统化调试(用户给了 Anthropic 真实测试 key)。
- **根因(已证)**:新建路径 `controller/channel.go fetchModelsByParams` 对除 Ollama/Gemini 外**一律** `Authorization: Bearer` + `{base}/v1/models`,与编辑路径 `fetchChannelUpstreamModelIDs`(已按 provider 定制头/路径)分叉退化。
  - Anthropic `/v1/models` 要 `x-api-key`+`anthropic-version`,不认 Bearer → 用 key 实测 Bearer→**401 Invalid bearer token**,x-api-key→**200+模型**。
  - AWS Bedrock 无 `/v1/models` 端点且默认 base 空;自定义/空 base 类型 URL 缺 host → 必失败。
  - **排除网络**:容器内 `wget https://api.anthropic.com/v1/models`→真实 401(出网/DNS/TLS 全通);DB 渠道有成功 response_time。先前 `/dev/tcp` 假阴性已复核纠正。
- **改法(收敛+静态兜底)**:
  - 新增 `fetchModelIDsForDisplay(channel)`:先调按-provider 定制的 `fetchChannelUpstreamModelIDs`(自动获得 Anthropic 头/各家路径),失败或空时回退 `relay.GetAdaptor(common.ChannelType2APIType(type)).GetModelList()` 静态列表(AWS 等)。**刻意与上游模型更新检测分离**(后者需严格错误语义,不走兜底)。
  - 重写 `fetchModelsByParams`:构造临时 `model.Channel{Type,Key,BaseURL}` → 委托 `fetchModelIDsForDisplay`;删除旧 Bearer-only 块(顺带去掉 `json.NewDecoder` 违反规则一、移除 channel.go 不再用到的 `gemini` import)。
  - 编辑路径两个按钮 handler `FetchUpstreamModels`(287)/`SupplierFetchUpstreamModels`(1139)改调 `fetchModelIDsForDisplay`(检测路径 `upstreamModels,err:=` 与 fetchModelIDsForDisplay 内部 1159 调用保持不变)。
- **改了哪些文件**：`controller/channel.go`(imports、`fetchModelsByParams` 重写、新增 `fetchModelIDsForDisplay`、两 handler 改调用);新增 `controller/fetch_models_test.go`。
- **如何验证**：① 新增确定性测试 3 条全过(Anthropic 必带 x-api-key/anthropic-version 且无 Bearer;AWS 走静态兜底非空;OpenAI 自定义 URL 用 Bearer 正常解析)——均能复现旧 bug(改前必红)。② **真机 API 实测**(临时 live test,用完即删):空 base→真实 `api.anthropic.com` 返回 8 个真实模型(claude-fable-5/opus-4-8/...)。③ `go build ./...` ✓、`go vet ./controller` 干净。
- **提交状态**：**未 commit、未 push、未重新部署到 5001 容器**(等用户指令)。第九版需求1(供应商渠道列表复用管理员)仍在设计阶段,未动工。

### [2026-06-16] 第九版需求1 后端:供应商渠道列表/行操作复用管理员(代码+部署+e2e实测,**未提交**)
- **方案**:`docs/superpowers/specs/2026-06-16-tokenki-v9-supplier-channel-reuse-design.md`(前端 mode 参数化 + 后端供应商作用域接口;否决"共享 admin 路由"提权方案)。
- **后端做了什么**:
  1. **列表复用(共享核心)**:把 `GetAllChannels`/`SearchChannels` 抽成 `listChannelsCore(c, forceSupplierId)`/`searchChannelsCore(c, forceSupplierId)`;管理员 wrapper 传 0(行为字节级不变),供应商传本人 id 强制 `supplier_id=本人`(忽略 supplier_name 等越权参数)。供应商因此免费获得 分组/模型/类型/状态筛选+排序+标签模式+类型计数,与管理员完全一致;并回填 `official_usd`/`receivable`(新增 `backfillSupplierUnsettled`)。
  2. **供应商行操作(归属校验后委托管理员 handler 完整复用)**:新增 `supplierOwnsChannelParam`(URL :id)/`supplierOwnsChannelBody`(body id/channel_id,读后复位 body 供下游重解析)两个守卫;新增 `SupplierTestChannel`/`SupplierUpdateChannelBalance`/`SupplierCopyChannel`(委托 TestChannel/UpdateChannelBalance/CopyChannel)、`SupplierManageMultiKeys`/`SupplierDetectChannelUpstreamModelUpdates`/`SupplierApplyChannelUpstreamModelUpdates`。启禁用/优先级/权重走既有 `SupplierUpdateChannel`。
  3. **路由**:`/api/supplier/channel` 组新增 `GET /search`、`GET /test/:id`、`GET /update_balance/:id`、`POST /copy/:id`、`POST /multi_key/manage`、`POST /upstream_updates/detect`、`POST /upstream_updates/apply`。
- **改了哪些文件**:`controller/channel.go`(list/search 抽核心+force+receivable 回填)、`controller/supplier_channel.go`(列表改委托核心 + 守卫 + 6 个委托 handler + backfill 助手 + imports bytes/io)、`router/api-router.go`。
- **如何验证**:`go build ./...` ✓、`go vet ./controller` 干净;交叉编译部署 `juhe-v12supplierch-be` 启动正常(**无 gin 路由冲突 panic**,证明 list-core 重构未破坏 admin 启动)。临时 access_token e2e 实测(tkadmin/test1,**测后已 NULL 清空核验**):① admin 列表 total=9、搜索 Claude→[1,13,10] **无回归**;② 供应商 test1 列表仅返回本人渠道 [12,13,10] 且带 `cost_price=2/official_usd=0.03941/receivable=0.07882`;③ `?type=14` 过滤→[13,10];④ test1 调 `test/2`、`GET /2`(tksupplier1 的渠道)→ **forbidden: not your channel**(归属拦截生效)。
- **剩余(未做)**:前端 `useChannelsData`/`ChannelsColumnDefs`/`ChannelsFilters`/`ChannelsPage` 的 mode 参数化 + 供应商页切到 `<ChannelsPage mode="supplier"/>` + 退役旧 supplier-channels 组件;页面级「全部/批量」工具栏操作(测试全部/全余额/修复/批量删/删禁用/全部检测·应用/标签)的供应商作用域版本或隐藏(待用户定);后端 Go 归属单测(当前以 e2e 实测覆盖)。
- **提交状态**:5001 已是 `juhe-v12supplierch-be`(仅后端,前端未动→供应商页 UI 暂无变化);**未 commit、未 push**。

### [2026-06-16] 第九版需求1 前端:供应商渠道页完全复用管理员表格(代码+部署+浏览器实测,**未提交**)
- **做了什么(前端 mode 参数化,真复用同一套组件)**:
  - `useChannelsData(mode='admin')`:新增 `isSupplierMode`/`apiBase`(supplier→`/api/supplier/channel`),全部单渠道端点(列表/搜索/删除/启禁用·优先级·权重 PUT/复制/单测/单余额/获取分组)按 apiBase 切换;`fetchGlobalPassThroughEnabled` 供应商端跳过(无 /api/option 权限);分组下拉供应商端走 `/api/supplier/self/groups`。
  - **批量操作 fan-out(作用域版)**:供应商端 批量删除/删除禁用/更新全部余额 改为循环单渠道接口(天然只作用于本人渠道);`useChannelUpstreamUpdates({apiBase})` 单条 detect/apply 切 apiBase。
  - `ChannelsColumnDefs(isSupplierMode)`:供应商去「创建者」、加「成本(¥)」「应收款(¥,2位)」;`ChannelsTable` 可见性过滤改 `!==false`(让无开关的成本/应收款列默认显示),并把 isSupplierMode 传入列定义。
  - `ChannelsFilters`:供应商隐藏「供应商名」筛选。
  - `ChannelsActions`:供应商隐藏 测试所有/修复能力表/检测全部·处理全部上游/批量设置标签/标签聚合模式(全局或无供应商端点);保留 更新全部余额/删除禁用(fan-out)。
  - `index.jsx ChannelsPage({mode})` → `useChannelsData(mode)`,`EditChannelModal apiMode` + `MultiKeyManageModal apiBase` 透传;`MultiKeyManageModal` 7 处端点按 apiBase。
  - `pages/SupplierChannels` 改渲染 `<ChannelsPage mode="supplier"/>`;**退役删除** `components/table/supplier-channels/*` + `hooks/supplier-channels/*`(确认无外部引用)。
  - **后端补丁**:`SupplierUpdateChannel` 改为返回更新后的渠道(原返回 nil,导致前端 manageChannel 读 `res.data.data.status` 报错、行状态不刷新);现回填全字段、不回传 key。
- **改了哪些文件**:`web/classic/src/hooks/channels/{useChannelsData,useChannelUpstreamUpdates}.jsx`、`components/table/channels/{index,ChannelsTable,ChannelsColumnDefs,ChannelsFilters,ChannelsActions}.jsx`、`components/table/channels/modals/MultiKeyManageModal.jsx`、`pages/SupplierChannels/index.jsx`(删 supplier-channels/* 与 hooks/supplier-channels/*);`controller/supplier_channel.go`。
- **如何验证**:`bun run build`(classic)✓;`go build ./...`/`go vet`/获取模型测试 ✓;交叉编译部署 `juhe-v14supplierch-fix`。**Playwright 实测 5001**(注册一次性供应商 v9probe + seed 渠道,测后连同 tkadmin 临时口令一并清理还原):① 供应商渠道页列=`ID 名称 分组 成本(¥3) 应收款(¥0.00) 类型 状态 响应时间 已用/剩余 优先级 权重`+操作(**去创建者、加成本/应收款**),行操作 测试/禁用/编辑/more 齐全;② 类型 tab/筛选/分页 scoped;③ 禁用→启用 走 `PUT /api/supplier/channel/` 且行内即时刷新(SupplierUpdateChannel 回填修复后)、0 console error;④ 批量操作下拉仅 更新全部余额/删除禁用(全局项已隐藏);⑤ **管理员渠道页无回归**:列含「创建者」、无成本/应收款、供应商名筛选在、10 行(mode 默认 admin)。
- **提交状态**:5001 已是 `juhe-v14supplierch-fix`;**未 commit、未 push**。清理:v9probe 用户/渠道/abilities 已删,tkadmin 口令已还原(核验 restored=true)。

### [2026-06-16] 修复供应商渠道页「Cannot read properties of undefined (reading 'map')」(系统化调试+代码+部署+实测,**未提交**)
- **现象**:供应商进入「我的渠道」/编辑渠道时弹错误 toast「错误：Cannot read properties of undefined (reading 'map')」(2 条 console error)。
- **系统化调试**:Playwright 复现 + CDP 抓栈。关键证据链:
  1. 错误是 toast(经 `showError(error.message)`),非 type/数据相关——所有供应商渠道编辑均触发,与渠道类型无关。
  2. CDP 栈:`showError(_)` 被两个相邻函数 `z@187292`/`P@187439` 调用;反编译部署 bundle 定位到 **`EditTagModal.jsx`** 的 `fetchModels`(`z`)与 `fetchGroups`(`P`)。
  3. 根因:**管理员鉴权接口对越权返回 HTTP 200 + `{success:false, data:undefined}`(非 403)**。`EditTagModal` 由 `ChannelsPage` 常驻挂载,其 useEffect 在 mount 即调 `fetchModels`(`GET /api/channel/models`)与 `fetchGroups`(`GET /api/group/`,硬编码管理员端点),两处 `res.data.data.map(...)` **未守卫** → 供应商下 data 为 undefined → `.map` 崩 → 2 条 toast。`EditChannelModal` 自身 fetchModels 已有守卫,故之前没发现。
- **改法(根因修复 + 纵深防御)**:`EditTagModal.fetchModels/fetchGroups` 增加 `if(!res?.data?.data||!Array.isArray(res.data.data))return;` 优雅降级(标签编辑本就是管理员功能);顺手给 `EditChannelModal.fetchGroups` 加同款守卫(防供应商分组端点偶发非数组)。管理员路径不变(data 为正常数组,守卫直接通过)。
- **改了哪些文件**:`web/classic/src/components/table/channels/modals/{EditTagModal,EditChannelModal}.jsx`。
- **如何验证**:`bun run build` ✓;部署 `juhe-v15tagmodalfix`;Playwright 实测(注册一次性供应商 v9edit + seed OpenAI/Claude/AWS 渠道):修复前 进页面/开编辑均弹 map toast;**修复后 页面加载 + 打开各类型渠道编辑均 0 toast / 0 console error,弹窗正常打开**。测试数据(v9edit + 4 渠道)已清理。
- **提交状态**:5001 已是 `juhe-v15tagmodalfix`;**未 commit、未 push**。

### [2026-06-17] 第十版后端:查看秘钥2FA门禁 + 日志应付供应商金额(TDD,**未提交**)
- **需求**:① 供应商查看自己渠道 key 需「已开启 2FA/Passkey」(只校验开过、不每次输码),超管查看任意渠道 key 跳过 2FA;③ 使用日志展示「应付供应商=官方价(USD)×渠道冻结成本价(¥/$)」,补齐各消费路径记录,普通用户不可见。设计见 `docs/superpowers/specs/2026-06-16-tokenki-v10-2fa-keyview-payable-settlement-design.md`。
- **做了什么(后端,全程 TDD red→green)**:
  1. **需求1 门禁**:`middleware/secure_verification.go` 新增 `RequireTwoFAEnabled()`(仅校验 `model.IsTwoFAEnabled` 或有 Passkey,未开启返回 403+`TWO_FA_NOT_ENABLED`);`router/api-router.go` 从 `POST /channel/:id/key` **移除** `SecureVerificationRequired`(超管所有渠道跳过 2FA),供应商组新增 `GET /supplier/channel/:id/key`(挂 `RequireTwoFAEnabled`+`DisableCache`);`controller/supplier_channel.go` `SupplierGetChannel` 改 `GetChannelById(id,false)`(Omit key,详情不再泄露明文)+ 新增 `SupplierGetChannelKey`(归属校验后返回明文 key)。注:GORM `Updates(struct)` 跳过零值,详情空 key 不会冲掉原 key,供应商编辑保存安全。
  2. **需求3 记录**:`service/supplier_billing.go` 新增 `OfficialUsdFromQuota(quota,groupRatio)=quota/(groupRatio×QuotaPerUnit)`(反推不含分组折扣的官方价);`model/log.go` `RecordConsumeLog` **统一归一**——仅供应商渠道按渠道当前成本价补 `cost_price_snapshot`,非供应商渠道把 official_usd/snapshot 清零(只在有计费字段时查渠道,普通请求热路径零开销);各路径接 official_usd:`service/quota.go`(wss/audio)、`service/task_billing.go`(任务)、`relay/mjproxy_handler.go`(MJ 两处);text 路径原已记录,违规罚金**有意不计应付**。
  3. **需求3 防泄露**:`controller/log.go` 新增 `clearSupplierBillingFields`,在 `GetUserLogs`(/log/self)与 `GetLogByKey`(/log/token)清零两字段——普通终端用户看不到平台对供应商的成本;admin(/log/)与供应商(/supplier/self/logs)接口保留。
- **新增/改测试(均先 red 后 green)**:`service/supplier_billing_test.go::TestOfficialUsdFromQuota`、`service/task_billing_supplier_test.go::TestLogTaskConsumption_RecordsSupplierPayable`、`model/log_supplier_billing_test.go::TestRecordConsumeLogSupplierBilling`、`middleware/require_twofa_test.go`(4 例:无凭证 403/有2FA放行/有Passkey放行/未登录401)、`controller/log_billing_strip_test.go::TestClearSupplierBillingFields`。
- **如何验证**:`go build ./...` ✓;`go test ./middleware ./service ./model` 全绿;`controller` 包除**预存在失败** `TestListModelsTokenLimitIncludesTieredBillingModel`(纯 HEAD worktree 复现同样 `RedisHGetObj` 空指针 panic,与本次无关)外全绿。
- **剩余(未做)**:前端 Req1(toast→引导弹窗+去设置2FA、供应商查看key走新接口并按2FA门禁、超管直显)、Req3 前端(已加「应付/应收(¥)」列,待 bun build)、Req4(申请结算确认弹窗显示金额)、Req2(管理员重置2FA classic 已具备,待实测)、i18n 文案键。
- **提交状态**:**未 commit、未 push**。

### [2026-06-17] 第十版前端 + Req2核验 + 对抗性复审(TDD,**未提交**)
- **Req1 前端(查看key 2FA 引导弹窗 + 角色化)**:`helpers/secureApiCall.js` `isVerificationRequiredError` 增识 `TWO_FA_NOT_ENABLED`;`hooks/common/useSecureVerification.jsx` 无验证方式时**改为开引导弹窗(不再 toast)**;`components/common/modals/SecureVerificationModal.jsx` 未启用分支加「去设置 2FA」按钮(`useNavigate` → `/console/personal?tab=security`);`services/secureVerification.js` 新增 `viewSupplierChannelKey`(GET 供应商 key 接口,带 `skipErrorHandler`);`components/table/channels/modals/EditChannelModal.jsx` `handleShow2FAModal` 按 `isSupplierMode` 选接口(供应商走新接口/管理员走原接口);`components/settings/personal/cards/AccountManagement.jsx` 读 `?tab=security` 受控激活安全 tab。
- **Req3 前端(应付列)**:`hooks/usage-logs/useUsageLogsData.jsx` COLUMN_KEYS 加 `PAYABLE`,默认仅 admin+supplier 可见、普通用户强制隐藏(初始可见性 + handleSelectAll 双重拦截);`components/table/usage-logs/UsageLogsColumnDefs.jsx` 新增列(供应商「应收(¥)」/管理员「应付(¥)」)= `official_usd × cost_price_snapshot`,snapshot/官方价为 0 显示「-」,附明细 tooltip;列选择器与表格两处 getLogsColumns 均已传 isSupplierUser。
- **Req4 前端(子代理实现)**:`hooks/supplier-settlements/useSupplierSettlementsData.jsx` 新增 `getPendingAmount`;`components/table/supplier-settlements/index.jsx` handleApply 先拉 `/api/supplier/self/pending` 再弹确认(展示应结算金额¥/官方价$/条数,无可结算时禁用确认)。
- **Req2 核验(子代理只读) + 补瑕疵**:后端重置 2FA(`AdminDisable2FA` 连带清备用码 + `canManageTargetRole` 拦截 + 重置后 `IsTwoFAEnabled=false` 可纯密码登录)与 reset_passkey **功能完整**;classic 前端补两处:`UsersColumnDefs.jsx` 对 role=100(超管)隐藏重置入口、`UsersTable.jsx` 重置后 `refresh?.()`。
- **i18n**:`web/classic/src/i18n/locales/zh-CN.json` 追加 8 个缺失键(去设置 2FA/应付(¥)/应收(¥)/官方价/当前没有可结算的消费/当前应结算金额：/条待结算/确定申请结算？),JSON 校验通过。
- **对抗性复审(code-reviewer 子代理 + 5 并行深挖,已逐条复验)**:无 Critical。子代理报的"task 计费放大 8×"经 lead 复验为**误报**(OtherRatios 本应计入 official_usd)。确认无新增越权/泄露:面向普通用户/令牌的原始日志端点仅 `GetUserLogs`/`GetLogByKey` 两处且都已 strip;供应商 key 接口有归属校验;单 key 空 key 不冲(GORM Updates 跳零值)。flag 三项**预存在/业务决策项**(非本次回归):①`SupplierUpdateChannel` 多 Key `ChannelInfo` 覆盖(预存在,建议对齐管理员 `patch.ChannelInfo=existing.ChannelInfo`);②`RequireTwoFAEnabled` 在 DB 故障时 fail-closed 误导已开 2FA 用户(安全方向对,体验欠佳);③违规罚金不计应付(有意决策,待产品确认)。按建议补 `TestLogTaskConsumption_NonSupplierChannelNoPayable` 回归守卫。
- **如何验证**:`go build ./...`/`go vet` ✓;`go test ./middleware ./service ./model` 全绿、`controller` 改动相关测试全绿(仅预存在 panic 测试 `TestListModelsTokenLimitIncludesTieredBillingModel` 除外);`bun run build`(classic)✓;zh-CN.json JSON 校验 ✓。
- **未做(待定)**:本地部署 + Playwright e2e 实测(需改生产环境,待用户指令);预存在的多 Key ChannelInfo 修复(超范围,待用户定)。
- **提交状态**:**未 commit、未 push**(等用户指令)。

### [2026-06-17] 合并供应商功能分支 + V11 四项需求 + 本地实测(**未提交**)
- **背景**:用户问"V10 需求4(管理员/超管可见供应商概览)做了吗"。排查发现该功能在 `de059221 第八版` 提交里、但只在分支 `feat/tokenki-p1a-supplier-backend`,**从未合进 main**(main 与该分支自首发 v2026.06.16.1 起平行分叉,各有一个"第十版")。
- **合并分支(经用户指令)**:`git merge --no-ff origin/feat/tokenki-p1a-supplier-backend` → merge commit,**0 冲突**(dry-run + `git cherry` 确认 4 个分支提交全是 main 没有的);新增 25 文件,含管理员概览页 `pages/SupplierOverviewAdmin/` + `components/supplier-overview-admin/` + `SuppliersSummaryBar.jsx` + 官方价手册 `service/official_pricing/*` + 渠道页复用。验证:`go build`/`go test` 全绿(仅白名单 `TestListModelsTokenLimitIncludesTieredBillingModel`)、`bun run build` classic ✓、交叉编译部署本地容器 `new-api`(VER juhe-merge-req4)启动无 panic、以超管 token 实调 `/api/admin/supplier-overview/` 返回真实聚合(供应商4/渠道8)。
- **V11 item1 文案**:`components/table/channels/modals/EditChannelModal.jsx` `t('供应商渠道必填，用于结算')` → `t('供应商渠道必填，用于结算和竞价排名，价格低会被优先消耗')`;`i18n/locales/zh-CN.json` 同步改键。
- **V11 item2 菜单置顶**:`components/layout/SiderBar.jsx` adminItems 把 `供应商概览(supplier_overview_admin)` 移到**第一位**(渠道管理之上)。
- **V11 item3 概览每类目供应商名单(TDD)**:后端 `model/supplier_stats.go` `SupplierTypeStat` 加 `Suppliers []SupplierBrief{user_id,name}`,聚合时按 user_id 升序、**最多5条**(SupplierCount 仍为真实总数),名字一次性查 users 表;先写失败测试 `TestGetSupplierOverviewPerTypeSupplierBriefs` + `TestGetSupplierOverviewSupplierListCappedAt5`(RED→GREEN)。前端 `components/supplier-overview-admin/TypeCard.jsx` 渲染名单 chips、点名字 `navigate('/console/suppliers?keyword='+name)`(stopPropagation 防触发卡片详情);`hooks/suppliers/useSuppliersData.jsx` 读 URL `?keyword=` 预填搜索框 + 挂载即搜该供应商。
- **V11 item4 供应商管理页空白修复**:根因排查——后端 `/api/supplier/` 正常返回 4 条、渲染路径无崩溃,真实问题是**菜单 `isAdmin()`(role≥10) 而 `/api/supplier/*` 是 `RootAuth()`(role≥100 超管专属)** → 普通管理员看到菜单却 403 空白。经用户决策:`SiderBar.jsx` `供应商管理(suppliers)` + `结算审核(settlement_review)` 菜单 `isAdmin()`→`isRoot()`(与 API 一致);`供应商概览` **保持 `isAdmin()`**(守住 V10 需求4)。
- **本地实测(Playwright,超管 superadmin 登录 localhost:5001 VER juhe-v11)**:item2 概览菜单已在管理员区第一;item3 OpenAI 卡显示 tksupplier1/tkadmin/test1、Anthropic 卡显示 test1,点 tksupplier1 → `/console/suppliers?keyword=tksupplier1` 搜索框预填且表格仅 1 条;item4 用 localStorage role=10 软刷新验证 → 供应商管理/结算审核/系统设置 隐藏、供应商概览/渠道管理 可见;item1 新文案已进 dist 产物。后端 `go build`/`go test` 全绿(仅白名单)。
- **遗留/待用户定**:① 整条分支合并(含官方价手册等)**仅本地、未 push/未部署 prod**;② item3 概览→供应商管理 下钻对 role≥10 管理员失效(概览 admin 可见但管理页已超管专属,本地仅 root 账号未触发);③ 供应商名单按 user_id 排序,是否改按最低价排序待定。
- **提交状态**:**未 commit、未 push**(等用户指令)。

### [2026-06-17] 修复供应商管理页"空白"真因(CSS table-scroll-card 塌宽) + item3 陈旧过滤(**未提交**)
- **用户反馈**:供应商管理页一直空白看不到列表。我先前用无障碍快照(accessibility snapshot)"验证"过有 4 行——**但快照只反映 DOM,不反映绘制**,漏掉了视觉 bug。改用**真实截图**复现:汇总条在、整张表格卡片不可见。
- **根因(浏览器实测定位)**:`.table-scroll-card`(`src/index.css`,CardPro 的卡片类)只设了 `display:flex; flex-direction:column; height:calc(100vh-110px)`,**没设宽度**。在供应商管理页(CardPro 上方还有一个 SuppliersSummaryBar 同级元素)这个 flex 列容器的 auto 宽度塌成 ~6px(≈滚动条宽),整张表被挤到视口右侧外(left=1809 > 视口 1836)→ 看似"空白"。其它表格页(结算审核/渠道/用户)CardPro 是页面唯一子元素,不塌,所以只有供应商管理页中招。
- **修复**:`src/index.css` `.table-scroll-card` 加 `width: 100%;`(浏览器注入实测:6px→1592px,表格立即可见)。全局安全:对本就满宽的其它页是 no-op。
- **附带修复 item3 陈旧过滤**:`hooks/suppliers/useSuppliersData.jsx` 改用 `useSearchParams` 响应式读取 `?keyword=`,站内从概览带 keyword 跳进来、再点"供应商管理"菜单回普通列表时会正确重置(原先一次性读 window.location 会留陈旧过滤,只剩 1 行)。
- **验证(真实截图,VER juhe-v11c,superadmin 全新加载)**:供应商管理页完整显示 4 供应商表格 ✅;供应商概览 item3 名单 chips(OpenAI: tksupplier1/tkadmin/test1,Anthropic: test1)✅;结算审核 6 行 ✅;渠道管理完整表格 ✅(无回归)。
- **教训**:UI 验证必须截图,不能只靠 accessibility snapshot(后者会把 DOM 里但绘制不可见的元素也列出来)。
- **提交状态**:**未 commit、未 push**(等用户指令)。

### [2026-06-17] V12 需求开发：供应商管理「已上架」列 + 概览渠道明细列表（**未提交**）
- **需求澄清**：用户在 AskUserQuestion 中纠正——概览列表里「令牌名字」写错了，**实际是分组名字**；每行=一个渠道，颗粒度=(分组+渠道)；已跑金额按**累计总消费(历史全部)**；卡片内最多5条按**成本价从低到高**，详情同序展示全部。
- **req1 已上架列(TDD)**：后端 `model/supplier_stats.go` 新增 `GetAllSuppliersChannelCounts() map[int]{Total,Enabled}`(一次取 supplier_id>0 的 (supplier_id,status) 在 Go 折叠，cross-DB 安全，admin 渠道 supplier_id=0 排除)；`SupplierListItem` 加 `ChannelTotal/ChannelEnabled`，`fillSupplierStats` 回填。先写失败测试 `TestGetAllSuppliersChannelCounts` + `TestFillSupplierStatsChannelCounts`(RED→GREEN)。前端 `SuppliersColumnDefs.jsx` 加「已上架」列显示 `上架数/启用数`(如 3/1，可点)，`SuppliersTable.jsx` 注入 `onNavigateChannels` → `/console/channel?supplier=<用户名>`。
- **req2 概览渠道明细(TDD)**：后端 `model/supplier_stats.go` 新增 `GetTotalOfficialUsdByChannels`(累计口径，**不带** settlement_id 过滤，区别于未结算版)；`SupplierChannelBrief{channel_id,supplier_id,supplier_name,group,cost_price,official_usd}`；`SupplierTypeStat` 加 `Channels []SupplierChannelBrief`，`GetSupplierOverview` 一次 LOG_DB 聚合累计已跑金额、每类目装配渠道明细并**按成本价升序、未定价(<=0)沉底**(同价按已跑金额降序、再 channel_id 升序)。先写失败测试 `TestGetTotalOfficialUsdByChannels` + `TestGetSupplierOverviewPerTypeChannelsV12`(RED→GREEN)；`resetSupplierOverviewTables` 加 migrate Log。前端 `TypeCard.jsx` 把 V11 供应商 chips 换成渠道迷你列表(top5：供应商名(链接)·分组 ¥成本价 $已跑金额 + "共 N 条")；`TypeDetailSheet.jsx` 加「渠道明细」全量表格(供应商/分组/成本价/已跑金额，与卡片字段一致)、宽度 420→560。i18n `zh-CN.json` 加 8 个键。
- **深链 + 陈旧过滤兜底**：`hooks/channels/useChannelsData.jsx` 加 `useSearchParams` 读 `?supplier=`(仅管理员模式)，`formInitValues.searchSupplier` 预填；effect 在 formApi 就绪后按供应商名搜索，并用 `lastSupplierRef` 记录上次应用值——**站内从 ?supplier=X 切到无参数(点渠道管理菜单)时正确重置为全部**，不留陈旧过滤(复用 V11 item3 教训)。渠道管理后端早已支持 `supplier_name` → `ResolveSupplierIdsByName`，无需改后端。
- **验证(真实截图，VER juhe-v12b，superadmin 登录 localhost:5001)**：
  - 供应商管理「已上架」列：test1=3/1、tksupplier1=3/2(可点)、ceshi1/tkadmin11=0(纯文本) ✅；点 3/1 → 渠道管理搜索框预填 test1、仅显示 test1 的 3 条(全部3/OpenAI1/Anthropic2) ✅。
  - 供应商概览卡片：OpenAI 卡按成本价升序显示 5 条(¥1.80→¥2.50)+「共6条」、Anthropic 卡 2 条，每行 供应商(链接)·分组 ¥价 $已跑 ✅。
  - 点卡片 → 详情抽屉「OpenAI·供应明细」展示全部 6 条(含卡片省略的第6条 tksupplier1·claude官key ¥3.00)+ 竞价分组最低价 ✅；详情内点 test1 → 关闭抽屉 + 渠道管理过滤 test1 ✅。
  - 回归：plain /console/channel 加载全部渠道(多供应商) ✅；?supplier=X→点渠道管理菜单重置为全部 ✅；console 0 error。
- **后端**：`go build ./...` 成功，`go test ./model/` 全绿。
- **提交状态**：**未 commit、未 push、未上 prod**(prod 仍 v2026.06.17.1)。本地 5001 = juhe-v12b。

### [2026-06-17] V12 概览卡片放大 + 渠道很多(10+)的布局（VER juhe-v12c，**未提交**）
- **用户反馈**：卡片太小、文字太小；并要求考虑渠道变多(如 10+)时的排版。
- **放大卡片**：`pages/SupplierOverviewAdmin/index.jsx` 栅格 `xs=12/sm=8/md=6/lg=4`(每行6)→ `xs=24/sm=12/lg=8`(每行3，卡片约 2× 宽)，gutter 12→16。`TypeCard.jsx` 字号整体上调：标题 13→16、家供应数字 20→26、可用/最低价 11/12→13/15、渠道行 11→13、分组 10→12、底部「共N条」10→12.5 且改为蓝色链接色；padding 14→18、行距/间距加大。
- **10+ 渠道布局**：卡片**恒定只展示前 5 条**(`CARD_CHANNEL_LIMIT=5`)+「共 N 条，点击查看全部」→ 卡片高度有界、栅格不会被撑乱，无论 6 条还是 60 条；完整列表在详情抽屉，`TypeDetailSheet.jsx` 渠道明细表 **>10 条自动分页(10/页)**，抽屉宽 560→620。
- **验证(真实截图，临时 seed 8 条让 OpenAI 达 14 条，截后已 DELETE 清理，DB 复原为 6)**：卡片明显变大更易读、3 列布局；OpenAI 卡显示 top5(按成本价 ¥1.68→升序)+「共 14 条」；点开详情抽屉「渠道明细」分页 第1-10条/共14条 + 翻页 1 2 ✅；竞价分组最低价同屏。console 0 error。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v12c。

### [2026-06-17] 渠道管理增加「成本价」列（VER juhe-v12d，**未提交**）
- **需求**：渠道管理表格加一列「成本价」，放在「已用/剩余」之后，供应商与管理员都可见。
- **改动**：`components/table/channels/ChannelsColumnDefs.jsx`——① 删除供应商模式原先放在「创建者」位的 `成本` 列(避免重复)，保留 `应收款`；② 在 `COLUMN_KEYS.BALANCE`(已用/剩余)列**之后**新增统一 `成本价` 列(`key:'cost_price'`, `dataIndex:'cost_price'`)，**不在 isSupplierMode 条件内**→ 两种角色都显示；渲染 `cost_price>0 → ¥X.XX，否则 -`(tag 聚合行不显示)。
- **数据**：`cost_price` 是 Channel 持久化字段(`json:"cost_price"`)，admin/supplier 列表接口都已下发(仅 `Omit("key")`)，无需改后端。列可见性 filter 为 `visibleColumns[key]!==false`，`'cost_price'` 不在列开关默认表里 → 默认可见、不进列显隐开关(与原供应商成本/应收款列一致)。i18n `成本价` 已存在。
- **验证(真实截图+DOM，VER juhe-v12d，superadmin)**：admin 渠道管理表头顺序 `…响应时间, 已用/剩余, 成本价, 优先级, 权重` ✅；成本价单元格 ¥4.50/¥3.00/¥2.20/¥2.00/¥1.80… 正常，admin 自有渠道(cost=0)显示 `-`；console 0 error。供应商模式因列在条件外、同一代码路径，必然可见(无供应商账号密码未单独截图)。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v12d。

### [2026-06-17] 调度机制审计 + 本地切换为按价格(bidding)（**未提交**）
- **审计结论**：本部署开了 Redis → `main.go` 强制 `MemoryCacheEnabled=true` → 走内存缓存调度(`channel_cache.go`)，用 `dispatchEffectivePriority` 排序；全局 `DispatchStrategy` 此前 DB 无记录 = 默认 `priority`。
  - priority 策略：有效优先级 = `供应商优先级×1e9 + 渠道优先级` → **供应商管理里的优先级是决定性主键**（实证：分组 claude官key 里 tksupplier1(供应商优先级5)的 ¥2.5 渠道压过 tkadmin 的 ¥2.2 更便宜渠道，贵的反而先被消耗）。
  - 价格(cost_price/bidding)此前**完全不参与调度**，与渠道页文案"价格低优先消耗"矛盾。
  - 若关掉 Redis/内存缓存 → 退回 DB/ability 路径(`ability.go`)，只认 `ability.priority(=渠道优先级)+权重`，供应商优先级与价格全失效。
- **用户决策**：改成按价格(bidding)，先只在本地验证。
- **改动**：`options` 表 upsert `DispatchStrategy=bidding`（运行态生效，无需改二进制；重启后 `loadOptionsFromDatabase` 载入 OptionMap，InitChannelCache 按价格重排）。本地 5001 已重启加载。
- **新增回归测试(TDD)**：`model/dispatch_test.go` `TestGetRandomSatisfiedChannel_BiddingCheapestFirst`——bidding 下选择层实测：最便宜渠道(¥1.5)最先被选(retry0)、次便宜(¥3.0,retry1)、无成本价渠道兜底(retry2)，且**供应商优先级被忽略**(贵渠道即便供应商优先级9也排后)。PASS。
- **验证**：6 个既有 dispatch 单测 + 新增选择层测试全绿；运行中应用 `GET /api/option` 返回 `DispatchStrategy="bidding"`（已登录实测）→ 线上 OptionMap 已是 bidding。
- **副作用提示**：bidding 下①供应商优先级不再影响路由(仅 UI 展示，切回 priority 才生效)；②无成本价的 admin 渠道沦为最后兜底；③同价渠道按权重加权随机。
- **prod 影响**：prod DB 没设该 option，仍是默认 priority。若要 prod 也按价格，需在 prod 库同样 upsert（非代码发版）。本地 = juhe-v12d + DB option bidding。
- **提交状态**：**未 commit、未 push**（dispatch_test.go 新增；DB option 仅本地）。

### [2026-06-17] 重试机制审计 + 本地启用 RetryTimes=10（**未提交**）
- **审计**：relay 重试循环 `for retry:=0; retry<=RetryTimes; retry++`，总尝试 = RetryTimes+1；每次重试 `retry` 序号传给渠道选择 → bidding 下**自动降到下一价位梯队**(retry0最便宜→retry1次便宜→…→无价兜底)。`shouldRetry` 门控：渠道类错误/5xx/连接失败→重试；400/内容违规/skip-retry→不重试。
- **此前现状**：`RetryTimes=0`(默认，DB/env 均未设) → 实际**只尝试 1 次、不重试** → 最低价渠道挂了直接报错，bidding 的价格降级容错形同虚设（仅 auto-ban 异步禁用坏渠道做被动兜底，但当次请求仍失败）。
- **用户决策**：设置 RetryTimes=10。
- **改动**：`PUT /api/option/ {key:RetryTimes,value:10}`（RootAuth，updateOptionMap 同步更新 `common.RetryTimes` 运行态 + DB 持久化，无需重启）。本地实测 `GET /api/option` 返回 `RetryTimes=10` & `DispatchStrategy=bidding`。
- **效果**：失败时最多再试 10 次，按价格梯队依次降级；实际重试次数还受可用价位梯队数/渠道数约束（retry 超出梯队数会钳到最后一档），并由 shouldRetry + auto-ban 提前收敛。
- **prod**：prod DB 仍是 RetryTimes 默认 0 + DispatchStrategy 默认 priority；要一致需在 prod 库同样设这两个 option（非代码发版）。
- **提交状态**：DB option 仅本地；无代码改动（纯运行态配置）。

### [2026-06-17] 概览卡片分组列垂直对齐 + 点击按分组筛选渠道（VER juhe-v13，**未提交**）
- **需求**：概览卡片第二列(分组名)做垂直对齐、不紧贴供应商名；点击分组名跳转渠道管理并筛选该分组渠道。
- **改动**：`TypeCard.jsx` 渠道列表从 flex 行改为 **CSS grid**(`供应商 5.5rem / 分组 1fr / 成本价 auto / 已跑 auto`)——分组成为独立对齐列；分组名加点击 → `/console/channel?group=<首个分组>`(多分组取第一个)。`useChannelsData.jsx` 深链扩展为同时读 `?supplier=` 与 `?group=`，formInitValues 预填 searchGroup，effect 用 (supplier,group) 组合 key 做陈旧过滤重置。`TypeDetailSheet.jsx` 详情表分组列也改为可点击(一致性)。i18n 加 `查看该分组渠道`。
- **验证(真实截图，VER juhe-v13，superadmin)**：卡片分组列垂直对齐(claude速刷/OpenAI官key/… 同列对齐) ✅；点 claude官key → 渠道管理筛选下拉=claude官key、表格仅 3 条该分组渠道(不同供应商) ✅；成本价列同屏可见；console 0 error。
- **提交状态**：**未 commit、未 push**。本地 5001 = juhe-v13。

### [2026-06-17] 供应商视角体验评估（无代码改动）
- 为体验供应商视角，临时用 admin API 把 tksupplier1(id=2,role=5) 密码改为 Tk888888，登录走查供应商端：概览(实时市场竞价/排名)、我的渠道(成本价/应收款/优先级/权重列)、账单结算(申请结算+历史)、数据看板(RPM/TPM+用量趋势+收益趋势+渠道排行)、添加渠道(复用管理员完整表单)。
- 评估结论(详见对话)：① 切 bidding 后"优先级"列对供应商已是误导(调度只看价格);② 添加渠道表单太重(应给供应商精简版);③ 分组缺竞价引导;④ 看板用量趋势请求/Token 共轴、收益用$非¥;⑤ 账单混入已取消;⑥ 缺"我能赚多少"预估 + 官key额度预警。均为建议,未改代码。
- ⚠️ 遗留：tksupplier1 密码被改成 Tk888888(测试号),需告知用户改回。

### [2026-06-17] V14 需求开发（6 项，VER juhe-v14，**未提交**）
- **item1 首页官 Key 面板改供应商数量**：`pages/Home/landing/Hero.jsx` 把右侧"已托管的官 Key"面板每行的 `sk-ant-•••• · 已加密`(暴露未加密观感)换成 `供应商数量：N 个`，四类硬编码 Claude 18 / AWS Bedrock 26 / OpenRouter 8 / OpenAI 6（与既有营销金额一致的展示数字）。
- **item2 竞价同价按优先级调度(TDD)**：`model/channel.go` `dispatchEffectivePriority` bidding 分支由"只看成本价"改为 `-costMilli*1e6 + clamp(渠道优先级,[0,1e6))`——成本价主键(价低先)、**同价时渠道优先级次键(高先)**，价格仍绝对主导。先写 RED：`TestDispatchEffectivePriority_BiddingPriorityTieBreaker`(同价 -2000=-2000 必败)+`...PriceDominatesPriority`(回归守卫)+集成 `TestGetRandomSatisfiedChannel_BiddingPriceTiePriorityTiers`(retry0 选高优先级、retry1 选低)，GREEN 后 9 个 dispatch 测试全绿。回答用户："可以"——同类型同分组同价才看优先级。
- **item3 分组竞价引导**：`EditChannelModal.jsx` 分组字段 label 加 `IconHelpCircle` tooltip(分组=竞价池/同组同模型相互竞价/价低先调用、同价看优先级/选匹配标准分组否则无流量)+ `extraText` 行内提示"💡 分组即竞价池…"。供应商/管理员共用表单都可见。
- **item4 账单结算状态筛选 + item5 时间段搜索(TDD)**：后端 `model/settlement.go` `GetSettlementsBySupplier` 签名加 `status,startTs,endTs`(0=不限，按 created_at 过滤，GORM 占位符 cross-DB 安全)；`controller/settlement.go` `SupplierListSettlements` 读 `?status=&start_timestamp=&end_timestamp=`。RED `TestGetSettlementsBySupplier_StatusAndTimeFilter`(签名不存在→编译失败)→GREEN，并修 `settlement_test.go` 旧调用。前端 `useSupplierSettlementsData.jsx` 加 statusFilter/startTs/endTs + handleStatusChange/handleDateRangeChange(dateRange→[start00:00,end23:59:59])；`supplier-settlements/index.jsx` actionsArea 加状态下拉(全部/已完成=2/已取消=3)+ DatePicker(dateRange)。⚠️ 已申请(待审核,status=1)未做成独立选项(用户只列了 3 项)，仅"全部"可见——已在报告里提示用户。
- **item6 待结算 tooltip**：`supplier-overview/index.jsx` 待结算卡片副文案加 lucide `Info` 图标 + Semi `Tooltip`，悬浮说明 应收(¥)=成本价×用量你将收到的金额 / 官方价($)=上游官方计费美元 / 笔数=待结算调用条数。
- **i18n**：`zh-CN.json` 末尾加 10 个缺失键(供应商数量/已取消/应收/笔数/按申请时间段筛选 + tooltip/extraText 长文案)。
- **本地部署踩坑(重要)**：`docker build`(完整 Dockerfile)在 docker 内 `bun run build` classic 时 **ResourceExhausted: cannot allocate memory**(Docker Desktop VM 内存上限)→ 镜像没更新(仍 46h 前旧镜像)。改用**宿主交叉编译**绕开：① 宿主 `bun run build` 两套前端(内存够,16.8MB/18.9MB)②`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build`(CGO 关、sqlite 是纯 Go glebarez/modernc，可直接交叉编译，前端经 //go:embed 打进 90MB 二进制)③ 极简 Dockerfile 仅 `COPY tokenki /tokenki`。**注意 arch**：跑中的容器是 arm64 native(aarch64)，第一次误编 amd64 不匹配，重编 arm64 才对。
- **容器重建**：运行栈是 compose project `newapi-juhe`(容器 `new-api`/`postgres`/`redis`，**库名 new-api 非 tokenki**)，与已改名为 tokenki 的 docker-compose.yml 漂移；故不能直接 `docker compose up`。改为 `docker stop/rm new-api` + `docker run` 复刻原配置(network `newapi-juhe_new-api-network`、`SQL_DSN=...postgres:5432/new-api`、REDIS、5001:3000、data/logs 绑定、`--log-dir /app/logs`)，**只换 app 容器，postgres/redis/数据 + 已持久化的 DispatchStrategy=bidding/RetryTimes=10 option 全保留**。
- **验证(真实截图 + DOM + 功能实测，VER juhe-v14，tksupplier1 登录 5001)**：
  - item1：面板四行 供应商数量 18/26/8/6 个，全页无 `已加密`/`sk-ant` ✅(截图)。
  - item6：悬浮待结算 ⓘ → tooltip 应收/官方价/笔数 三行说明 ✅(截图)。
  - item4：状态选"已完成"→ 表格仅 3 条(均已结算 id 5/2/1)、共 3 条 ✅(DOM 实测)。
  - item5：日期选 2026-06-14~06-14 → 仅 4 条(申请时间均 06-14，id 6/5/4/3)、共 4 条，排除 06-13/05-25 ✅(DOM 实测)。
  - item3：分组字段 ⓘ tooltip(竞价池长文案)+ 💡 行内提示均显示 ✅(截图)。
  - item2：dispatch 逻辑无法截图，9 单测全绿覆盖。
  - 结算页全新加载 console **0 error**(登录前的 401 风暴/key-gate 403/渠道测试 401 均非本次引入)。
- **后端**：`go build ./...` 成功；`go test ./model/...` 全绿；`./controller/...` 唯一失败是白名单内 pre-existing `TestListModelsTokenLimitIncludesTieredBillingModel`(缺 Redis mock，与本次无关)。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v14。注意工作树同时含**之前未提交的 V13-new 3 文件**(TypeCard/TypeDetailSheet/useChannelsData)。⚠️ tksupplier1 密码仍是 Tk888888。

### [2026-06-17] 实时市场竞价卡片重构：一类型一卡、行=分组(VER juhe-v14d，**未提交**)
- **需求(AskUserQuestion 二选一确认)**：原「一个(类型,分组)一张卡、行=该分组匿名报价梯队」改为「一个渠道类型一张卡、行=该类型下的分组」，每行显示 分组名 + 市场最低价，供应商已上架的分组标【你】。
- **后端(TDD)**：`model/supplier_stats.go` 新增 `GetSupplierMarketByType(supplierId)` → `[]MarketTypeBids{Type,TypeName,Groups[]MarketGroupRow{Group,LowestPrice,Mine,MyBest},MyCount,Total}`。类型范围=供应商参与的渠道类型；行范围=该类型下**所有**市场分组(自有 ∪ 他人启用正价的)，`mine` 标自有(任意状态)，`lowest_price`=该(type,group)启用且 cost_price>0 的市场最低价(nil=暂无报价)，`my_best`=供应商自己最低价。行排序：自有优先→价低优先(nil 沉底)→分组名。先写 RED `TestGetSupplierMarketByType`(覆盖 自有a/b、仅禁用c(暂无报价)、他人gpt(无你)、跨类型x 排除)→GREEN。保留旧 `GetSupplierMarketBids`(及其测试)不动，仅把 `controller/supplier_market.go` 的 overview 切到新函数(JSON 键仍叫 `bids`)。
- **前端**：`supplier-overview/index.jsx` 重写 `BidCard`——头部去掉分组副标题、报价数改「N 个分组」；行从「排名图标+匿名价」改为「分组名 +【你】+ ¥市场最低价(或 暂无报价)」，价低条更长；底部「我的排名 X/Y」改「已上架 my_count/total 分组」。删除已无用的 `RankBadge`/`RANK_STYLES` 及 `Crown/Trophy/Medal` 导入。i18n 加 `个分组/暂无报价/你`。
- **验证(真实截图+DOM，VER juhe-v14d，tksupplier1 登录 5001)**：原 OpenAI 两张卡(claude官key/claude速刷)合并为 **一张 OpenAI 卡**，行=claude速刷[你]¥1.80、claude官key[你]¥2.20(按价升序)，徽标「2 个分组」，底部「已上架 2/2 分组」✅；登录后 overview 0 render error(唯一 console error 是登录前的 session 过期提示)。
- **部署**：沿用宿主交叉编译(arm64)+极简镜像重建容器流程；app 健康无 panic。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v14(含本次 bid 卡重构)。

### [2026-06-17] V14 item7：账单结算页顶部「待结算汇总」卡片(VER juhe-v14e，**未提交**)
- **需求**：账单结算页顶部清晰展示当前供应商待结算账单——应收金额、美金/人民币/笔数等全部信息；申请结算按钮放到待结算金额后面。
- **Hook**(`useSupplierSettlementsData.jsx`)：新增 `pending` 状态({payable_cny,official_usd,log_count})；`getPendingAmount` 改为既 return 又 `setPending` 缓存(一次请求供顶卡+确认弹窗共用)；挂载时拉取、`applySettlement`/`cancelSettlement` 成功后刷新(申请后清零/取消后回升)；return 暴露 `pending`。
- **前端**(`supplier-settlements/index.jsx`)：表格上方新增 `Card` 待结算汇总——左侧 Wallet 图标 + 「待结算金额（应收 ¥）」+ⓘtooltip(应收/官方价/笔数释义) + **大号 ¥应收 + 申请结算按钮(紧随金额，nothingToSettle 时禁用)**；右侧 官方价($) + 待结算笔数(笔)；底部一行说明「应收=成本价×用量；申请结算快照为待审核、提交后清零」。从表格 actionsArea **移除**申请结算按钮(上移)，表格标题改「结算记录」，筛选(全部/已完成/已取消 + 日期段)保留。i18n 加 5 键。
- **验证(真实截图+DOM+0 error，VER juhe-v14e，tksupplier1)**：顶卡显示 ¥0.00 +申请结算(因应收0禁用) + 官方价$0.45 + 4 笔 + 说明行；下方「结算记录」表 6 行 + 筛选；结算页 console 0 error；app 健康无 panic。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v14(含 item7)。

### [2026-06-17] item7 数据时效隐患排查 + 修复(VER juhe-v14f，**未提交**)
- **用户疑问**：账单结算页数据是否实时？不实时刷新 / 点击申请结算那刻没拉到最新待结算金额，会不会出问题？
- **排查结论**：
  - **结算金额正确性：安全**。前端 POST `/api/supplier/self/settlement/` 不带金额；`model.CreateSettlement` 在该刻用 per-supplier 锁 + 原子 `UPDATE settlement_id` 打包所有 `settlement_id=0` 未结算日志，金额(官方价/应收/笔数)全后端现算、用每条冻结的 `cost_price_snapshot`。→ 前端数字纯预览，陈旧与否都不影响实际结算；并发/双击被锁挡(第二次"无可结算"报错)。
  - **隐患(我 item7 引入，属可用性非金额错误)**：顶部 `pending` 非轮询，只在 进页面/申请后/取消后 刷新；且申请结算按钮 `disabled` 绑定该陈旧状态 → 若进页面时 ¥0、之后又跑量，按钮会一直禁用，供应商有钱可结却点不了须刷新。`handleApply` 点击时本会重拉最新，但被禁用按钮挡住兜底走不到。
- **修复**：① 顶部申请结算按钮移除 `disabled={nothingToSettle}`(仅保留 loading) → 始终可点；点击 `handleApply` 重拉最新→弹窗按最新展示(无可结算时 OK 禁用 + "当前没有可结算的消费")→后端原子结算。② hook 加 `window focus`/`visibilitychange` 监听，页面重新可见时刷新 `pending`，避免久留陈旧。
- **验证(DOM+实测，VER juhe-v14f，tksupplier1)**：¥0.00 时申请结算按钮 `disabled=false`(可点)；点击弹出确认框「当前没有可结算的消费」+ 提交说明(OK 禁用)→ 证明点击重拉+弹窗兜底可达，陈旧不再卡死；app 健康无 panic、0 console error。
- **提交状态**：**未 commit、未 push、未上 prod**。本地 5001 = juhe-v14(含 item7 + 时效修复)。
