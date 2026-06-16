# TokenKi 第十版设计：查看秘钥 2FA 弹窗 / 管理员重置 2FA / 日志应付金额 / 结算确认弹窗

- 日期：2026-06-16
- 分支：feat/tokenki-p1a-supplier-backend（沿用）
- 前端主题：**classic only**（`web/classic/`，React 18 + Vite + Semi Design）。所有 UI 改动只在 classic。
- 关联：[[tokenki-aggregation-project]]、v8 供应商体系增强、v9 供应商渠道复用。

## 背景与目标

第十版围绕供应商体系的安全与对账体验，落地 4 个需求：

1. 查看秘钥的 2FA 提示从 toast 改为引导弹窗，并按角色区分强制策略。
2. 超级管理员可重置（关闭）指定用户的 2FA，保障丢失 2FA/备用码的用户恢复密码登录。
3. 使用日志展示「应付供应商金额」= 该请求官方美金价 × 渠道冻结成本价。
4. 供应商申请结算时弹窗确认当前实际账单金额。

## 已确认的产品决策

- 需求1-供应商：查看自己 key **只要求账号已开启 2FA/Passkey**（不做每次输码挑战）；未开启则弹引导弹窗。
- 需求1-管理员：超级管理员查看**所有渠道** key 都跳过 2FA。
- 需求3-可见性：「应付」**仅供应商（看自己日志）+ 管理员**可见，普通终端用户不可见（成本价敏感）。
- 需求3-展示：日志新增**单列「应付(¥)」**= official_usd × cost_price_snapshot，旧日志/非供应商渠道（快照为 0）显示「-」。

## 现状结论（调研）

- 错误文案「您需要先启用两步验证或 Passkey 才能执行此操作」来自 `web/classic/src/hooks/common/useSecureVerification.jsx:93`：无 2FA 时先 `showError` toast 并 `return false`，导致 `SecureVerificationModal.jsx:84-121` 已存在的"未启用验证"弹窗分支成为死代码，且该分支无"去设置"跳转按钮。
- 超管查看 key 走 `POST /api/channel/:id/key`（`RootAuth` + `SecureVerificationRequired`）。供应商相关取 key 走供应商接口（不强制 2FA）。
- 需求2 已基本具备：后端 `DELETE /api/user/:id/2fa`、`DELETE /api/user/:id/reset_passkey`；classic 前端 `ResetTwoFAModal.jsx`/`ResetPasskeyModal.jsx` 已接入用户管理表。
- 日志表已有 `official_usd` 与 `cost_price_snapshot`（成交时冻结），结算用 `Σ(official_usd × cost_price_snapshot)`；但仅文本中继路径记录这两字段，前端日志表未展示。
- 结算待结算金额接口 `GET /api/supplier/self/pending` 已就绪（`payable_cny` / `official_usd` / `log_count`）。

## 设计

### 需求1：查看秘钥 2FA 弹窗 + 角色化强制

**后端**
- `router/api-router.go`：从 `POST /api/channel/:id/key` 移除 `SecureVerificationRequired()` 中间件 → 超管查看任意渠道 key 不再需要 2FA。
- 新增轻量中间件 `RequireTwoFAEnabled()`（`middleware/`）：仅校验当前用户已启用 2FA 或已注册 Passkey；未启用返回 `403 { success:false, code:"TWO_FA_NOT_ENABLED", message:... }`。**不做** session 级输码挑战。
- 供应商查看 key 改为受控动作：渠道详情不再直吐明文 key；新增 `GET /api/supplier/channel/:id/key`（`SupplierAuth` + `RequireTwoFAEnabled`）返回明文 key。具体改造点以调研确认的现有供应商取 key 路径为准。

**前端（classic）**
- `helpers/secureApiCall.js`：`isVerificationRequiredError` 增补识别 `TWO_FA_NOT_ENABLED`，或新增 `isTwoFANotEnabledError`。
- `hooks/common/useSecureVerification.jsx`：无 2FA/Passkey 时不再 toast，而是打开引导弹窗（设置 `isModalVisible=true` + 一个"未启用"标志）。
- `components/common/modals/SecureVerificationModal.jsx`：在"未启用验证"分支新增「去设置 2FA」按钮，点击导航至个人设置 → 安全设置（路由以调研确认为准）。
- 供应商"查看密钥"流程：先 `checkVerificationMethods()`；已开启→直接调供应商 key 接口显示；未开启→打开引导弹窗。
- 超管"查看密钥"：后端去掉验证后点击直接显示。

### 需求2：管理员重置用户 2FA（以验证为主）

- 后端与 classic 前端均已具备。开发阶段实测：重置 2FA 后目标用户下次登录不再强制验证码、可纯密码登录并重新设置 2FA；如发现链路缺口再补（如重置后清理 backup code、并提示是否同时重置 passkey）。

### 需求3：日志「应付供应商金额」

**后端**
- 抽公共 helper（基于 `service/supplier_billing.go::ComputeOfficialUsd` 与渠道成本价快照）在所有消费记账路径统一写入 `official_usd` + `cost_price_snapshot`：text（已做）、audio、realtime(wss)、task、违规扣费等。
- 日志查询接口：确保把 `official_usd`、`cost_price_snapshot` 返回给供应商与管理员；普通终端用户的日志响应不含这两个字段（或前端不展示）。

**前端（classic）**
- `components/table/usage-logs/UsageLogsColumnDefs.jsx`：新增单列「应付(¥)」= `official_usd × cost_price_snapshot`，保留 2 位小数；该列仅供应商+管理员渲染；快照为 0/非供应商渠道显示「-」。

### 需求4：申请结算确认弹窗显示金额

**前端（classic）**（后端无需改）
- 供应商点"申请结算"→ 先 `GET /api/supplier/self/pending` → 弹窗显示「应付 ¥X（官方价 $Y · 共 N 条）」→ 确认后 `POST /api/supplier/self/settlement/`。

## 测试

- 后端 Go 单测：`RequireTwoFAEnabled`（开启/未开启/有 passkey）、各消费路径写入 official_usd/cost_price_snapshot、pending 金额。
- 前端：`bun run build` 通过；手动验证供应商无/有 2FA 看 key、超管直接看、日志应付列、结算确认弹窗金额。
- 全程兼容 SQLite/MySQL/PostgreSQL，JSON 走 `common/*`。

## 风险与边界

- 移除超管 key 验证是安全降级，但为已确认的产品决策。
- 供应商 key 接口的"受控化"改造需确认现有前端是否依赖详情接口直读明文 key，避免回归。
- 旧日志 `cost_price_snapshot=0` 与非供应商渠道一律显示「-」，不回填历史。
- 保护信息（new-api / QuantumNous 标识）不改动。
