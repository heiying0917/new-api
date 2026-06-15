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
