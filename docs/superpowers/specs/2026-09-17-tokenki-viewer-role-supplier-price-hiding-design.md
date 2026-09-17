# TokenKi · 观察员角色 + 供应商端隐藏售价 + 角色下拉框 — 设计方案

- **日期**: 2026-09-17
- **分支**: `feat/viewer-role-supplier-price-hiding`（提交后待用户指令合入 main / 发版）
- **定位**: 新增只读"观察员"角色供客户核对平台资源；堵住供应商端能反推平台售价的三条泄露路径；用户角色改为编辑弹窗里的下拉框选择，替代原"提升/降级"。
- **前端主题**: 仅 `web/classic`（React 18 + Vite + Semi Design）。**不碰 `web/default`**。
- **关联**: [[tokenki-aggregation-project]]；前序 `2026-06-16-tokenki-v9-supplier-channel-reuse-design.md`（供应商渠道页 mode 参数化范式）。

---

## 用户确认的决策（2026-09-17 Q&A）

1. **三件事一起做**：观察员角色、供应商端隐藏售价、角色下拉框，一个规格、一次提交。
2. **观察员能力**：看全部渠道信息但不可编辑；看不到成本价/应收款/密钥等；**可以看消耗和余额**；可以看全平台使用日志；可以创建令牌、调用资源；具备钱包（充值/兑换/余额）。
3. **观察员看全平台日志时，其他用户的用户名、令牌名、IP 隐藏**（默认决定 1，用户已确认）。客户名单是商业信息，观察员只需看到"平台在跑什么、跑了多少"。
4. **删掉"提升/降级"**：前端按钮与两个弹窗、后端 `ManageUser` 的 `promote`/`demote` 分支一并删除，只保留编辑弹窗下拉框一条改角色路径（默认决定 2，用户已确认）。
5. **分组倍率不影响供应商结算**（已核实：`official_usd = quota ÷ (分组倍率 × QuotaPerUnit)`，倍率被除掉），本次只处理"供应商能看到"的问题。
6. **模型广场**靠配置不改代码：Azure Claude 等高倍率分组不进全局"用户可用分组"，只经"特殊可用分组"授权给客户组。

---

## 现状（file:line）

### 角色与鉴权

| 项 | 现状 | 位置 |
|---|---|---|
| 角色常量 | Guest 0 / Common 1 / Supplier 5 / Admin 10 / Root 100 | `common/constants.go:198-203` |
| 角色合法性 | `IsValidateRole` 枚举上述五值；`authHelper` 用它校验会话，非法角色直接拒 | `common/constants.go:205-208`；`middleware/auth.go:25-34,161` |
| 鉴权中间件 | `authHelper(c, minRole)` 线性 `role < minRole` 拒；`UserAuth`=1、`SupplierAuth`=5、`AdminAuth`=10、`RootAuth`=100 | `middleware/auth.go:36-216` |
| 会话角色回查 | 会话用户每次请求从 `GetUserCache` 回查 status/role/group，改角色即时生效 | `middleware/auth.go:125-145` |
| 前端角色判断 | `isAdmin()`≥10、`isRoot()`≥100、`isSupplier()`===5（严格） | `web/classic/src/helpers/utils.jsx:36-58` |
| 前端路由守卫 | `PrivateRoute` / `AdminRoute`(≥10) / `SupplierRoute`(===5) | `web/classic/src/helpers/auth.jsx:52-84` |
| 注册 | 公开注册即供应商 | `controller/user.go:243` |

### 渠道接口

| 项 | 现状 | 位置 |
|---|---|---|
| 管理员渠道列表核心 | `listChannelsCore(c, forceSupplierId)`：分页/筛选/排序/标签模式/类型计数，`Omit("key")`，`clearChannelInfo` 只清多 Key 禁用原因，`backfillChannelSupplierNames`，供应商模式回填 `official_usd`/`receivable`，直接 `ApiSuccess` | `controller/channel.go:129-258` |
| 管理员渠道搜索核心 | `searchChannelsCore(c, forceSupplierId)`，同上范式 | `controller/channel.go:331+` |
| 供应商复用 | `SupplierListChannels`/`SupplierSearchChannels` = core(c, 本人 id) | `controller/supplier_channel.go:34-41` |
| 分组名列表 | `GetGroups` 只返回分组名（无倍率）；管理员走 `/api/group/`，供应商走 `/api/supplier/self/groups` | `controller/group.go:14-24`；`router/api-router.go:196` |
| 渠道模型 | `Channel` 结构体 JSON 含 `key`(列表已 Omit)、`base_url`、`cost_price`、`setting`、`param_override`、`header_override`、`remark`、`weight`、`priority`、`other_info`、`supplier_name`、`created_by_name`、`channel_info`（多 Key 状态） | `model/channel.go:24-84` |

### 日志接口

| 项 | 现状 | 位置 |
|---|---|---|
| 管理员全站日志 | `GetAllLogs` → `model.GetAllLogs(...)` 返回整行（含 username/token_name/ip/other/official_usd/cost_price_snapshot/channel_name） | `controller/log.go:24-46`；`model/log.go:345` |
| 管理员统计 | `GetLogsStat` → `model.SumUsedQuota(...)` 返回 quota/rpm/tpm | `controller/log.go:112-137`；`model/log.go:484` |
| 终端用户日志脱敏 | `formatUserLogs`：清 `ChannelName`、删 `other.admin_info`/`stream_status`；`clearSupplierBillingFields`：official_usd/cost_price_snapshot 归零 | `model/log.go:74-88`；`controller/log.go:15-23` |
| 供应商日志 | `SupplierListLogs` 返回本人渠道整行日志，仅 `blankConsumerIdentity` 清 username/token_name；**`quota`（含分组倍率的售价）、`other.group_ratio`/`user_group_ratio`/`model_price` 原样返回** | `controller/supplier_logs.go:15-56,101-111` |
| 供应商统计 | `SupplierLogsStat` → `SumSupplierStat` = **SUM(quota)**（售价总额） | `controller/supplier_logs.go:58-84`；`model/log.go:615-634` |
| 供应商实时 | `SupplierRealtime` 只返回 rpm/tpm（quota 未输出，**无需改**） | `controller/supplier_dashboard.go:47-65` |
| 供应商趋势/排行 | 用 `SUM(official_usd)`（**无需改**） | `model/supplier_stats.go:497-541` |
| `other` 全部键 | admin_info, billing_mode, billing_preference, billing_source, cache_ratio, cache_tokens, claude, completion_ratio, frt, group_ratio, is_model_mapped, is_system_prompt_overwritten, matched_tier, model_price, model_ratio, po, reasoning_effort, request_conversion, request_path, stream_status, subscription_*(9 个), upstream_model_name, user_group_ratio, wallet_quota_deducted | `service/log_info_generate.go` |
| 前端日志钩子 | `useLogsData()` 内部用 `isAdmin()`/`isSupplier()` 决定 URL：supplier→`/api/supplier/self/logs`，admin→`/api/log/`，其他→`/api/log/self/`；统计同理 | `web/classic/src/hooks/usage-logs/useUsageLogsData.jsx:82-90,313,338,353,800-812` |
| 前端日志列 | `isAdminUser`/`isSupplierUser` 门控：渠道列(admin‖supplier)、用户列(admin)、IP、`admin_info` 内容(admin)、"花费"列(所有角色)、"应收(¥)/应付(¥)"列 | `web/classic/src/components/table/usage-logs/UsageLogsColumnDefs.jsx:522,592,807,836,875,922` |
| 前端日志统计标签 | `消耗额度: renderQuota(stat.quota)`（供应商也显示售价总额） | `web/classic/src/components/table/usage-logs/UsageLogsActions.jsx:58` |

### 用户管理

| 项 | 现状 | 位置 |
|---|---|---|
| 提升/降级 | `ManageUser` 的 `promote`（仅 root，→10）/`demote`（→1）分支 | `controller/user.go:980-999` |
| 编辑用户 | `UpdateUser` 用 `canManageTargetRole` 校验 origin/updated 两个角色后调 `Edit`，**但 `Edit` 的 updates 只含 username/display_name/group/remark/password，role 根本不落库** | `controller/user.go:640-672`；`model/user.go:570-602` |
| 自更新 | `UpdateSelf` 走 `cleanUser.Update()`，与 `Edit` 无关 | `controller/user.go:710+` |
| 前端按钮/弹窗 | 用户表操作列"提升/降级"按钮；`PromoteUserModal`/`DemoteUserModal`；`UsersTable` 状态与 handler；`useUsersData.manageUser` | `UsersColumnDefs.jsx:301-314`；`UsersTable.jsx:28-29,57-58,69-76,106-114,144-145,157-158,213-226` |
| 编辑弹窗 | `EditUserModal` 表单：username/password/display_name/remark/group/quota；无角色字段；`loadUser` 从 `/api/user/:id` 取到 `role` | `EditUserModal.jsx:110-120,298-372` |
| 角色标签 | `renderRole`：1 普通用户 / 5 供应商 / 10 管理员 / 100 超级管理员 | `UsersColumnDefs.jsx:44-70` |

---

## 需求 1 — 观察员角色（`RoleViewerUser = 3`）

### 1.1 后端

**常量与鉴权**
- `common.RoleViewerUser = 3`，加入 `IsValidateRole`（否则观察员登录后会话被 `validUserInfo` 拒）。
- `middleware/auth.go`：把 `authHelper(c, minRole)` 的角色比较抽成谓词版 `authHelperWithRoleCheck(c, allow func(role int) bool)`，原 `authHelper` 变为 `allow = role >= minRole` 的薄包装，行为不变。新增 `ViewerAuth()`：`allow = role == RoleViewerUser || role >= RoleAdminUser`。**不能**用线性 minRole=3，否则供应商（5）也能看全站渠道。

**路由组 `/api/viewer`（全部 GET，`ViewerAuth`）**

| 路径 | 处理器 | 复用 | 脱敏 |
|---|---|---|---|
| `/channel/` | `ViewerListChannels` | `listChannelsCore` | 白名单 DTO |
| `/channel/search` | `ViewerSearchChannels` | `searchChannelsCore` | 白名单 DTO |
| `/channel/models` | `EnabledListModels` | 直接复用（只有模型名） | 无 |
| `/groups` | `GetGroups` | 直接复用（只有分组名） | 无 |
| `/log/` | `ViewerListLogs` | `GetAllLogs` 的查询逻辑 | 日志脱敏 |
| `/log/stat` | `ViewerLogsStat` | `GetLogsStat` 的查询逻辑 | 无（quota 是售价，对客户不敏感） |

不开放：测试、刷余额、拉上游模型、渠道详情 `/:id`、任何写操作。

**渠道白名单 DTO** `viewerChannelView`（`controller/viewer_channel.go`）

输出字段（仅此）：`id, name, type, status, models, group, response_time, test_time, created_time, used_quota, balance, balance_updated_time, channel_info{is_multi_key, multi_key_size}`，标签模式下 `children` 同样映射，`tag` 仅在标签模式的父行保留（客户端按 tag 分组显示需要）。

**不输出**：`key, base_url, cost_price, official_usd, receivable, setting, settings, param_override, header_override, model_mapping, status_code_mapping, remark, weight, priority, auto_ban, other, other_info, openai_organization, test_model, supplier_id, supplier_name, created_by, created_by_name`，`channel_info` 里的状态列表/轮询下标/模式。

实现方式：`listChannelsCore`/`searchChannelsCore` 增加一个 `channelListOptions{forceSupplierId int; viewer bool}` 参数（现有调用方改为传结构体，行为不变）；`viewer=true` 时强制 `tag_mode` 仍按请求处理，但 `supplier_name` 过滤忽略、不回填供应商名与应收款，输出前 `items` 逐项映射为 `viewerChannelView`。类型计数、分页字段保持原结构，前端 `useChannelsData` 无需改响应解析。

**日志脱敏** `blankViewerLog(logs)`（`controller/viewer_log.go`）：
- `Username=""`、`TokenName=""`、`Ip=""`、`UserId=0`、`TokenId=0`
- `OfficialUsd=0`、`CostPriceSnapshot=0`（复用 `clearSupplierBillingFields`）
- `Other` 删 `admin_info`、`stream_status`（与 `formatUserLogs` 一致），保留 `channel_name`
- 查询参数只接受 `type / model_name / channel / group / start_timestamp / end_timestamp / request_id`；`username`、`token_name` 固定传空（身份已隐藏，不允许按身份筛）。

### 1.2 前端（`web/classic`）

- `helpers/utils.jsx`：`isViewer()` 严格 `role === 3`；`helpers/auth.jsx`：`ViewerRoute`（`role === 3` 放行）。
- `UsersColumnDefs.renderRole`：`case 3` → 紫色标签"观察员"。
- **渠道页**：`ChannelsPage mode='viewer'`，`useChannelsData('viewer')`：
  - `apiBase = '/api/viewer/channel'`；分组接口 `/api/viewer/groups`；`isViewerMode` 随 `isSupplierMode` 一并下传到 `ChannelsTable / ChannelsColumnDefs / ChannelsActions / ChannelsFilters / index.jsx`。
  - 列：保留 ID / 名称 / 分组 / 类型 / 状态 / 响应时间 / 已用·剩余；**隐藏** 创建者、应收款、成本价、优先级、权重、操作列；"剩余"标签不可点击刷新（`onClick` 在 viewer 模式为空）。
  - 动作区：只保留刷新、筛选、搜索；隐藏新增、批量操作、标签模式开关、删除禁用、修复数据库、全部测试、更新全部余额、上游更新检测。
  - 表格：无行选择、点击行不打开编辑、不渲染 `EditChannelModal`。
  - 筛选：隐藏"供应商用户名/邮箱"输入（与供应商模式同）。
- **日志页**：`useLogsData({ mode })`，`mode==='viewer'` 时 `isViewerUser=true`：
  - URL `/api/viewer/log/`、统计 `/api/viewer/log/stat`；`STORAGE_KEY='logs-table-columns-viewer'`。
  - 列：渠道列显示（与 admin‖supplier 同），用户列隐藏，"应付(¥)"列隐藏，`admin_info` 内容不渲染，IP 列因后端置空自然为空。筛选器隐藏用户名/令牌名输入。
  - 观察员走 `/log` 仍是本人日志（普通用户行为），全平台日志是新页面。
- **页面与路由**：`pages/ViewerChannels/index.jsx`（`<ChannelsPage mode='viewer' />`）、`pages/ViewerLogs/index.jsx`（`<UsageLogsTable mode='viewer' />`）；`App.jsx` 加 `/console/viewer/channels`、`/console/viewer/logs`，`ViewerRoute` 守卫。
- **侧栏**：`SiderBar.jsx` 加 `viewerItems`（"资源总览"→`/console/viewer/channels`、"全平台日志"→`/console/viewer/logs`），在"控制台"区之后、`isViewer()` 时渲染"观察员"分区。控制台/个人中心（令牌、日志、充值、个人设置）对观察员照常显示。
- i18n：`zh-CN.json`/`en.json` 补"观察员"、"资源总览"、"全平台日志"、"角色"、"官方计费"等键。

---

## 需求 2 — 供应商端隐藏售价

### 2.1 后端
- `controller/supplier_logs.go` 新增 `blankSellingPrice(logs)`，在 `blankConsumerIdentity` 之后调用：
  - `Quota = 0`
  - `Other` 删除：`group_ratio`、`user_group_ratio`、`billing_mode`、`matched_tier`、`billing_preference`、`billing_source`、`wallet_quota_deducted`、全部 `subscription_*`、`admin_info`、`stream_status`。保留 `model_ratio / completion_ratio / model_price / cache_ratio / cache_tokens / frt / is_model_mapped / upstream_model_name / claude / po / reasoning_effort / request_path / request_conversion / is_system_prompt_overwritten`（均为官方价参数或请求元数据，不含平台加价）。
- `SupplierLogsStat`：`SumSupplierStat` 拆出 `SumSupplierOfficialUsd(channelIds, start, end) float64`，响应改为 `{"official_usd", "rpm", "tpm"}`，不再返回 `quota`。`SumSupplierStat` 保留给其他调用方（rpm/tpm 部分）。

### 2.2 前端
- `UsageLogsColumnDefs`：`isSupplierUser` 时不渲染"花费"列；价格明细段落因 `group_ratio` 缺失自然不再出现"分组 3.3x"。
- `UsageLogsActions`：`isSupplierUser` 时统计标签显示 `官方计费: $x.xx`（取 `stat.official_usd`），否则维持 `消耗额度`。

---

## 需求 3 — 角色改为编辑弹窗下拉框

### 3.1 后端
- 角色持久化**不走 `Edit`**（既有安全测试 `TestEdit_DoesNotEscalateRoleStatusQuota` 锁定 `Edit` 字段白名单不含 role，防注入提权）。新增 `model.UpdateUserRole(userId, role)`：单列 `Update("role")` + `invalidateUserCache`（会话下一请求回源 DB，即时生效），只接受 1/3/5/10。`UpdateUser` 在 `Edit` 成功后、角色确有变化时调用它。
- `UpdateUser` 角色校验（在现有两处 `canManageTargetRole` 之外）：
  1. `updatedUser.Role` 必须 ∈ {1, 3, 5, 10}（`IsValidateRole` 且 ≠ Guest、≠ Root）；否则 `MsgUserRoleInvalid`。
  2. `originUser.Role == Root` 时角色不可变更；否则 `MsgUserCannotChangeRootRole`。
  3. `updatedUser.Id == 当前用户 id` 且角色变化 → 拒；`MsgUserCannotChangeOwnRole`。
  4. 角色不变时跳过以上校验（兼容前端不传 role 的老调用：payload 缺 role 时 `updatedUser.Role` 为 0 → 视为"不改"）。校验顺序：可分配集合 → 自己 → 超管 → 层级。
- `ManageUser` 删除 `promote`/`demote` 两个 case 及对应 i18n 键引用（键本身保留在 yaml，避免无关 diff）。
- 新增 i18n 键（en / zh-CN / zh-TW）：`user.role_invalid`、`user.cannot_change_root_role`、`user.cannot_change_own_role`。

### 3.2 前端
- `EditUserModal`：在"分组"之下加 `Form.Select field='role' label='角色'`，仅编辑他人（`userId` 存在）时渲染。选项按当前登录者：root → 普通用户(1)/观察员(3)/供应商(5)/管理员(10)；admin → 1/3/5。目标是超管（role 100）或目标是自己时禁用并提示。`submit` 的 payload 带 `role`。
- 删除：`UsersColumnDefs` 的"提升/降级"按钮；`UsersTable` 中 Promote/Demote 的 import、state、handler、JSX；`PromoteUserModal.jsx`、`DemoteUserModal.jsx` 文件；`useUsersData.manageUser` 注释更新（保留 enable/disable/delete/unlock 用途）。

---

## 边界与不做

- 供应商改为其他角色：名下渠道 `supplier_id` 不动，结算数据不受影响（与原"降级"一致），本期不加提示。
- 不做：按客户划分可见渠道子集、`web/default`、隐藏模型广场入口、观察员看渠道禁用原因、观察员导出。
- 无数据库结构变更（`role` 列已存在），SQLite / MySQL / PostgreSQL 无影响。
- 注册仍为供应商；观察员只能由管理员/超管在编辑弹窗中设置。

---

## 测试计划

**Go（与边界同文件夹，`testify/require`）**
- `common/constants_test.go`：`IsValidateRole(3)` 为真。
- `middleware/viewer_auth_test.go`：`ViewerAuth` 对 role 3、10、100 放行，对 1、5 拒（会话注入，`model.DB == nil` 降级路径）。
- `controller/viewer_channel_test.go`：`viewerChannelView` 白名单——构造含全部敏感字段的 `Channel`，序列化后断言敏感键不存在、白名单键存在且值正确；标签模式 children 同样脱敏。
- `controller/viewer_log_test.go`：`blankViewerLog` 清身份/成本/admin_info，保留 channel_name、model、quota。
- `controller/supplier_logs_test.go`：`blankSellingPrice` 清 quota 与黑名单键，保留 model_ratio 等；nil 安全。
- `model/log_test.go`：`SumSupplierOfficialUsd`（SQLite 内存库）= SUM(official_usd) 且忽略非本人渠道。
- `controller/user_role_test.go`：`validateRoleChange` 规则表（合法值、拒 root、拒 0/100、拒改自己、拒改 root、admin 不能设 10、root 能设 10、缺省不改）。
- `model/user_role_test.go`：`UpdateUserRole` 只改 role 不碰 status/quota，拒绝 0/100/非法值。

**前端**
- `bun run build`（`web/classic`）通过。
- 本地起服务（走现有 compose PG），三种角色各点一遍：
  - 观察员：侧栏出现"观察员"分区；渠道页无操作列/成本价，余额与已用可见；全平台日志无用户名；令牌/充值可用；直接请求 `/api/channel/` 得 403、`/api/viewer/channel/` 200。
  - 供应商：日志页无"花费"列，明细无"分组 x.x"，统计显示官方计费；`/api/supplier/self/logs` 响应 `quota=0`、`other` 无 `group_ratio`。
  - 超管：编辑用户弹窗角色下拉可切 1/3/5/10，切换后目标用户刷新即生效；用户表无"提升/降级"。

---

## 交付纪律

- 每完成一部分记 `docs/superpowers/WORKLOG.md`。
- 用户已授权：自测通过后 **commit**（不 push、不发版），随后停下等发版指令。
