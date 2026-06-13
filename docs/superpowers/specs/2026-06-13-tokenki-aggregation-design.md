# TokenKi 官key聚合平台 — 需求与技术方案

| 项 | 内容 |
|---|---|
| 文档版本 | v1.0 |
| 日期 | 2026-06-13 |
| 代码基线 | new-api fork `heiying0917/new-api` @ `6f415428` |
| 技术栈 | Go 1.22+ / Gin / GORM v2；React 19 / Rsbuild / Base UI / Tailwind；PostgreSQL + Redis |
| 部署 | Docker（PostgreSQL + Redis），本地 5001→容器 3000 |
| 域名 | tokenki.com |
| 定位 | **叠加式**二次开发：保留 new-api 原有用户/钱包/订阅体系，在其上叠加供应商聚合与结算能力 |

---

## 1. 项目概述

### 1.1 背景与目标
在 new-api 基础上二次开发一个 **Claude / OpenAI / AWS 官 key 供应商聚合平台**。供应商注册入驻后上传自己的官 key 资源（渠道），平台聚合大量官 key 统一**管理、调度、计费、结算**，对外提供给企业使用。平台按**官方计费方式与价格**计量每个供应商的官 key 消耗，供应商可申请结算，平台按其消耗量与报价生成结算单并完成结算。

### 1.2 核心业务闭环
```
供应商注册入驻 → 上传官key渠道(选分组+报价 X元=1刀) → 平台调度其渠道对外服务
   → 按官方价累计该供应商消耗($) → 供应商发起结算 → 超管确认实付金额(¥/$) → 结算完成
```

### 1.3 范围边界（本期）
- **仅做上游结算账本**：聚焦"平台应付供应商多少钱"。
- **下游消费方**（用 token 调用 API 的企业）沿用 new-api 现有的 token / quota / 钱包计费，本期**不改造**。
- 下游消费方的开通/充值/计费 UI 不在本期范围。

---

## 2. 名词定义

| 名词 | 含义 |
|---|---|
| **供应商 (Supplier)** | 注册入驻、上传官 key 资源的角色。新增角色，权限介于普通用户与管理员之间 |
| **下游消费方** | 使用平台 token 调用 API 的企业。沿用现有用户体系 |
| **官 key / 渠道 (Channel)** | 供应商上传的上游 API 凭据，复用 new-api 的 Channel 实体 |
| **分组 (Group)** | **产品线**。如 `claude官key`、`claude官key速刷`、`AWS platform`、`AWS bedrock`、`GPT官key`。供应商上传渠道时选择分组并报价；调度/市场价以分组为单位 |
| **成本价 (CostPrice)** | 渠道级，单位 **¥/$**。如 `2.5` 表示供应商用 2.5 元人民币卖 1 美元的官方价消耗。仅参与结算 |
| **官方价 (Official Price)** | Claude/OpenAI 等的官方模型价格（美元）。计量供应商消耗的基准，**不含给下游的折扣（group ratio）** |
| **结算 (Settlement)** | 把某周期内某供应商的官方价消耗打包成账单，按成本价折算应付金额，经超管确认实付 |

---

## 3. 已确认的关键决策

| # | 决策点 | 结论 |
|---|---|---|
| D1 | 计费范围 | **仅上游**：累计供应商官方价消耗→结算。下游不动 |
| D2 | 组织单位 | **分组=产品线**。供应商上传渠道选分组+报价；市场价/竞价以分组内最低价为准 |
| D3 | 价格模型 | 仅**渠道级**成本价（¥/$）。供应商管理页"价格"列仅做展示（区间/均价），不参与计算 |
| D4 | 调度策略 | **全局开关**二选一：竞价（分组内价低优先）/ 优先级（供应商优先级→渠道优先级） |
| D5 | 渠道编辑权 | 供应商使用**完整渠道编辑器**，按归属过滤只能见/改自己的渠道 |
| D6 | 健康检测 | 定时检测，默认 **30 分钟**，可配置 |
| D7 | 市场价展示 | 按**分组**展示当前最低成本价（匿名，仅露价格）；仅供应商+管理员可见 |
| D8 | 自动结算 | 到点只**自动生成待结算账单**，仍需超管确认实付金额 |

### 假设（随文档一并确认，可纠正）
- **A1** 渠道优先级：供应商可设，管理员可覆盖。
- **A2** 竞价比较量：同分组内按渠道 `cost_price`（¥/$）升序，越低越优先；同价用 weight。
- **A3** 手机号：注册必填，不做短信验证。
- **A4** 供应商优先级仅在「优先级」调度模式下生效；「竞价」模式纯看价格。
- **A5** 下游消费方已存在（用现有 token 调用），本期不为其做新计费 UI。
- **A6** 邮箱验证依赖平台已配置 SMTP（运维前置项）。

---

## 4. 需求规格

### 4.1 供应商申请与账号（FR-S）
- **FR-S1** 关闭普通用户的公开注册产出；公开注册/登录仅面向供应商。
- **FR-S2** 注册需提交 **邮箱（邮件验证）+ 手机号（不验证）**，注册成功**自动成为供应商**。
- **FR-S3** 后台新增「供应商管理」页，**仅超级管理员可见**。
- **FR-S4** 供应商列表（仿用户列表），字段含：供应商优先级（可编辑）、是否启用、价格（展示）、备注、结算方式等。

### 4.2 权限与渠道归属（FR-P）
- **FR-P1** 新增供应商角色，权限 = 普通用户 + 上传/管理自己的 key、编辑自己上传的渠道、设定渠道优先级。
- **FR-P2** 每个渠道与供应商关联。
- **FR-P3** 供应商仅对自己上传的渠道可见、可编辑；对他人渠道不可见。
- **FR-P4** 供应商添加渠道时填写成本价（¥/$）；该价参与结算、展示在渠道列表；管理员可据价手动调整渠道优先级。

### 4.3 计费记账（FR-B）
- **FR-B1** 按 Claude/OpenAI 官方模型价格计量供应商渠道的消耗（美元口径，不含下游折扣）。
- **FR-B2** 产生费用后供应商可发起结算。

### 4.4 调度与可用性（FR-D）
- **FR-D1** 两级调度：先按供应商优先级调度供应商，再按渠道优先级选渠道（优先级模式）。
- **FR-D2** 全局开关切换竞价/优先级两种调度策略。
- **FR-D3** 某供应商渠道全部不可用时自动重试其他供应商。
- **FR-D4** 自动禁用不可用渠道；某供应商全部渠道不可用→自动禁用该供应商；恢复后自动启用。
- **FR-D5** 支持定时自动检测渠道可用性（默认 30min，可配）。

### 4.5 市场价展示（FR-M）
- **FR-M1** 控制台首页展示当前在跑供应商的价格，按分组显示分组内最低价（匿名）。
- **FR-M2** 仅供应商和管理员可见。

### 4.6 结算系统（FR-T）
- **FR-T1** 「账单结算」页：显示当前待结算金额、过往账单记录（日期 / 消耗金额 / 实际结算金额 $或¥）。
- **FR-T2** 点击结算 → 弹窗确认"结算 $XX" → 确认后把当前待结算金额与消耗记录**打包**生成一张**待结算账单**；供应商与管理员均可**撤销**。
- **FR-T3** 账单申请后超管可见结算申请；超管手动处理：确认结算、输入**实际结算金额（¥或$二选一）**、填备注、点「确认结算」。
- **FR-T4** 完整账单包含：账单周期、消耗金额、实际结算金额、结算时间、结算方式、**精确到每一条使用日志的明细**。
- **FR-T5** 账单支持查看详情、**导出 Excel**。
- **FR-T6** 自动结算：供应商可在结算页选择手动/自动；自动可选按天/周/月。按天=次日 0 点生成前一天账单；按周=周一 0 点生成上周；按月=每月 1 号 0 点生成上月。手动/自动用户与管理员均可编辑；管理员在供应商管理编辑供应商时也可配置。
- **FR-T7** 超管可见供应商结算申请列表（含供应商信息 + 待结算账单信息），可进详情查看全部内容。

---

## 5. new-api 现状能力深入分析

> 结论：现有底子对本需求**支撑度高**。角色体系、渠道调度（优先级+权重+分组）、官方价表、按渠道聚合日志、定时任务框架、自动禁用都已具备，主要工作是**叠加新角色 + 渠道归属 + 供应商层调度 + 全新结算账本**。

### 5.1 角色与认证 ✅ 高复用
- 角色常量 `common/constants.go:182-196`：`RoleGuestUser=0 / RoleCommonUser=1 / RoleAdminUser=10 / RoleRootUser=100`，**层级数值制**（值越大权限越大）。
- 认证中间件 `middleware/auth.go`：`UserAuth()`(170)、`AdminAuth()`(176)、`RootAuth()`(182)。
- 层级管理判定 `controller/user.go:279` `canManageTargetRole(myRole, targetRole)`。
- 注册 `controller/user.go:138-234`：有邮箱字段与邮箱验证开关（`common.EmailVerificationEnabled`），新用户**强制 `RoleCommonUser`**(184)。**User 模型无 phone 字段**（`model/user.go:24-56`）。
- 前端角色 `web/default/src/lib/roles.ts`（USER=1/ADMIN=10/SUPER_ADMIN=100）；菜单按角色过滤 `hooks/use-sidebar-view.ts`、`hooks/use-sidebar-data.ts`。
- **差距**：需新增供应商角色（值=5）+ `SupplierAuth` 中间件；注册赋供应商角色；User 加 phone；前端加角色与菜单。

### 5.2 渠道模型与调度 ✅ 高复用
- Channel 模型 `model/channel.go:23-60`：含 `Priority(*int64)`、`Weight(*uint)`、`Group(逗号分隔)`、`Status(1启用/2手动禁/3自动禁)`、`AutoBan`、多 Key（`ChannelInfo`）。**无 owner/supplier 字段**。
- 调度两层：`middleware/distributor.go:32-169` → `service/channel_select.go:83-162` → `model/ability.go`（Ability 表三元组 group+model+channel；`getPriority()`:61 按优先级分层，重试降级；`GetChannel()`:106 同优先级内按 weight 加权随机）。分组支持 `auto` 跨组重试。
- 重试 `controller/relay.go:190-237`：`common.RetryTimes` 次，`shouldRetry()`:324 决策，每次重试取下一渠道。
- 自动禁用 `service/channel.go:19-65` `ShouldDisableChannel/DisableChannel`；多 Key 部分禁用 `model/channel.go:641-691`。
- **差距**：调度需叠加"供应商层"排序键；需"竞价"模式（分组内 cost_price 升序）；供应商级联禁用/恢复。

### 5.3 计费与配额 ✅ 高复用（有一个关键坑）
- 计量单位 `common/constants.go:62` `QuotaPerUnit = 500000`，即 **50万 quota = $1**。
- 官方模型价表 `setting/ratio_setting/model_ratio.go`：模型倍率(26-283)、固定价(285-317)、补全倍率(341-346)；接口 `GetModelRatio/GetModelPrice/GetCompletionRatio`。
- 计费公式（按倍率）：`quota = (prompt + completion×completionRatio) × modelRatio × groupRatio`，结算入口 `service/billing.go:34` / `service/billing_session.go`。
- **⚠️ 关键坑**：现有 `Log.Quota` 已乘 **groupRatio**（对下游的折扣）。供应商结算要用**官方原价**（groupRatio 视为 1），不能直接复用现有 quota → 需在供应商渠道请求上**单独记录官方价**。
- **差距**：在供应商渠道 post-consume 处记录"官方价美元"；按供应商/分组聚合。

### 5.4 用量日志 ✅ 高复用
- `model/log.go:34-56`：`Log` 含 `UserId/ChannelId(索引)/ModelName/Quota/PromptTokens/CompletionTokens/CreatedAt/Group` 等；`LogTypeConsume=2`。
- 记录 `RecordConsumeLog`:222；聚合 `SumUsedQuota(...channel...)`:457 已支持按渠道筛选。
- **回溯能力**：可经 `channel_id` → 渠道 → 供应商，按时间区间聚合 quota。✅ 满足结算明细需求。
- **差距**：Log 加 `OfficialUsd`（或入 `Other` JSON）+ `SettlementId`（已结算标记）。

### 5.5 定时任务 / 健康检测 ⚠️ 部分
- 已有定时任务框架（日志可见 `codex credential auto-refresh tick=10m`、`subscription quota reset tick=1m`、`upstream model update interval=30m` 等周期任务）。
- 渠道测试 `controller/channel-test.go:77-157` 仅**手动**；**无定时自动检测**、无自动恢复。
- **差距**：新增定时健康检测任务（默认 30min）+ 自动启用恢复。

### 5.6 前端结构 ✅ 高复用（缺 Excel 库）
- TanStack 文件路由 `web/default/src/routes/_authenticated/*`；新增页仿 `users/index.tsx`，`beforeLoad` 做角色守卫。
- 侧边栏 `hooks/use-sidebar-data.ts`（admin 组）；过滤 `use-sidebar-view.ts`。
- 模板：用户管理 `features/users/*`（表格 TanStack Table、编辑抽屉 RHF+Zod）；渠道管理 `features/channels/*`（含复杂表单 `use-channel-mutate-form.ts`）。
- i18n `src/i18n/locales/{en,zh,...}.json`，`useTranslation()`。
- **⚠️ 未安装 Excel 导出库**（无 xlsx/exceljs）。
- **差距**：新增供应商管理、账单结算等页；Excel 建议**服务端 excelize 生成**（明细可能很大）。

### 5.7 能力复用度总览
| 能力域 | 现状 | 复用度 | 主要差距 |
|---|---|---|---|
| 角色/认证 | 4 级角色 + 中间件 | 高 | +供应商角色、+phone |
| 渠道/调度 | 优先级+权重+分组+重试 | 高 | +归属、+供应商层、+竞价模式 |
| 计费 | 官方价表 + quota | 高 | +官方价记账(去折扣) |
| 用量日志 | 字段齐全可聚合 | 高 | +OfficialUsd、+SettlementId |
| 定时任务 | 框架在 | 中 | +定时健康检测 |
| 自动禁用 | 渠道级有 | 中 | +供应商级联+恢复 |
| 前端框架 | 路由/菜单/表格/i18n | 高 | +新页面、+Excel(服务端) |
| 结算 | **无** | 无 | 全新构建 |

---

## 6. 需求—能力差距矩阵

| 需求 | 现有可用 | 差距/改造点 | 阶段 |
|---|---|---|---|
| FR-S1/S2 注册即供应商+手机号 | 注册+邮箱验证 | +供应商角色赋值、+phone 列、前端表单 | P1 |
| FR-S3/S4 供应商管理页 | 用户管理页模板 | 新页+RootAuth+Supplier 表字段 | P1 |
| FR-P1~P4 渠道归属+自助 | 渠道 CRUD(admin) | +supplier_id/cost_price、供应商作用域接口、归属过滤 | P2 |
| FR-D1/D2 两级调度+开关 | 单层优先级+权重 | +供应商层排序、+竞价模式、+全局开关 | P3 |
| FR-D3 跨供应商重试 | 渠道重试 | 复用+按供应商分组重试 | P3 |
| FR-D4 自动禁用+级联+恢复 | 渠道自动禁用 | +级联禁用供应商、+自动恢复 | P3 |
| FR-D5 定时健康检测 | 手动测试 | +定时任务 | P3 |
| FR-B1/B2 官方价记账 | 官方价表+日志 | +官方价记账(去折扣)、+按供应商聚合 | P4 |
| FR-M1/M2 市场价 | 首页有 dashboard | +分组最低价卡片+可见性 | P4 |
| FR-T1~T7 结算系统 | 无 | 全新：Settlement 模型+流程+自动结算+Excel | P5 |

---

## 7. 技术方案

### 7.1 总体架构与数据流
```
[供应商] 注册入驻 ─► User(role=supplier) + Supplier(profile)
            │
            └─ 上传渠道 ─► Channel(supplier_id, group, cost_price, priority, key...)
                                    │
[下游 token 请求] ─► distributor ─► 候选渠道(group+model via Ability)
                                    │
                          ┌─ priority 模式: 供应商优先级 → 渠道优先级 → weight
                          └─ bidding  模式: 分组内 cost_price 升序 → weight
                                    │
                          relay 转发 ─ 失败 ─► 自动禁用 + 跨供应商重试
                                    │ 成功
                          post-consume ─► Log(+OfficialUsd, channel_id, supplier 可溯)
                                    │
[结算] 聚合 Log(SettlementId=0) ─► Settlement(账单) ─► 超管确认实付 ─► 锁定明细
[市场价] 按 group 取 min(cost_price of 启用渠道) ─► 首页卡片
[健康检测] 定时任务 ─► 探测渠道 ─► 更新 status ─► 级联禁用/恢复供应商
```

### 7.2 数据模型变更（字段级）

**(1) 角色** `common/constants.go`
```go
RoleSupplierUser = 5   // 介于 Common(1) 与 Admin(10)
```
新增中间件 `SupplierAuth()`（role>=5）。审计现有 `role == RoleCommonUser` 的等值判断（应多为 `>=`，需逐一确认不误伤供应商）。

**(2) User** `model/user.go`
```go
Phone string `json:"phone" gorm:"index"`   // 不验证
```

**(3) Supplier（新表，1:1 user_id）** —— 隔离供应商专属属性，`priority` 需建索引供调度排序
```go
type Supplier struct {
  UserId          int    `gorm:"primaryKey"`
  Priority        int    `gorm:"index;default:0"`  // 管理员设，优先级模式用
  Enabled         bool   `gorm:"default:true"`
  SettlementMode  string `gorm:"default:'manual'"` // manual|auto
  SettlementCycle string `gorm:"default:'month'"`  // day|week|month
  Remark          string
  // 缓存字段(可选): PendingQuota / LastSettledAt
}
```

**(4) Channel** `model/channel.go`
```go
SupplierId int     `gorm:"index"`   // = owner user id
CostPrice  float64 `gorm:"default:0"` // ¥/$
```
复用现有 `Group / Priority / Weight / Status / AutoBan / ChannelInfo`。

**(5) Log** `model/log.go`（官方价 + 已结算标记）
```go
OfficialUsd  float64 `gorm:"index"` // 本条消耗的官方价美元(groupRatio=1)
SettlementId int     `gorm:"index;default:0"` // 0=未结算
```

**(6) Settlement（新表，结算账单）**
```go
type Settlement struct {
  Id              int
  SupplierId      int    `gorm:"index"`
  Status          int    `gorm:"index"` // 1待审核 2已结算 3已撤销
  PeriodStart     int64
  PeriodEnd       int64
  ConsumedUsd     float64 // 周期内官方价消耗($)
  ComputedAmount  float64 // 按成本价折算应付(¥)
  ActualAmount    float64 // 超管确认实付
  ActualCurrency  string  // CNY|USD
  SettleMethod    string  // 转账/支付宝/...
  Remark          string
  Source          string  // manual|auto
  CreatedAt       int64
  SettledAt       int64
  // 明细 = Log where settlement_id = this.Id
}
```

**(7) 系统设置** `setting/`（OptionMap）
```
DispatchStrategy             = "priority"  // priority|bidding 全局开关
SupplierHealthCheckInterval  = 1800        // 秒
SupplierHealthCheckEnabled   = true
```

### 7.3 调度方案（双模式，全局开关 D4）
扩展 `service/channel_select.go` 候选渠道排序：
```
候选 = Ability(group, model) → 关联 channel(supplier_id, cost_price, priority, weight)
                              → 关联 supplier(priority, enabled) 且 enabled=true

if DispatchStrategy == "priority":      // A4：供应商优先级生效
    排序键 = (supplier.priority DESC, channel.priority DESC)
    同键内 weight 加权随机
else (bidding):                          // A2：纯价格
    排序键 = (channel.cost_price ASC)
    同价内 weight 加权随机

重试：按"供应商"分桶，本供应商所有渠道耗尽再降到下一供应商（FR-D3），
      复用现有 RetryTimes / shouldRetry。
```
> 性能：调度在热路径且走缓存。将 `supplier_priority / cost_price / supplier_enabled` 随渠道缓存一并加载（扩展 channel/ability 缓存），避免每次 join DB。

### 7.4 计费记账（官方价 FR-B1，解决 5.3 的坑）
在供应商渠道（`channel.supplier_id>0`）的 post-consume（`service/quota.go` 系列）处：
```
officialQuota = promptTokens × modelRatio
              + completionTokens × modelRatio × completionRatio   // 不乘 groupRatio
officialUsd   = officialQuota / QuotaPerUnit
→ 写入 Log.OfficialUsd
```
聚合：`SUM(official_usd) WHERE channel.supplier_id=? AND settlement_id=0 AND type=2 [AND 时间区间]`。

### 7.5 可用性（FR-D4/D5）
- **渠道自动禁用**：复用 `ShouldDisableChannel`。
- **级联**：渠道状态变更后检查该供应商是否"全部渠道不可用"→ 置 `Supplier.Enabled=false` 并移出调度；任一渠道恢复→ 自动 `Enabled=true`。
- **定时健康检测**：新增任务（默认 1800s），对启用渠道发最小探测请求（仿 `channel-test.go`），更新 status，触发级联与恢复。探测消耗计入供应商正常消耗（D6）。

### 7.6 结算状态机（FR-T）
```
                         供应商发起结算(FR-T2)
   [累计中(SettlementId=0)] ───────────────► [待审核 status=1]
            ▲                                      │  撤销(供应商/管理员, FR-T2)
            └──────────────────────────────────────┘  → 释放日志(SettlementId 归 0)
                                                   │  超管确认(填实付¥/$+方式+备注, FR-T3)
                                                   ▼
                                            [已结算 status=2] (明细快照锁定)
```
- 发起：把区间内 `SettlementId=0` 的消耗日志批量置为新账单 Id；`ConsumedUsd / ComputedAmount(=Σ officialUsd × cost_price)` 入账单。
- 撤销：账单置 3，关联日志 `SettlementId` 归 0。
- 自动结算（D8/FR-T6）：日/周/月 定时（Asia/Shanghai，次日/周一/1号 0 点）只生成 `status=1, source=auto` 账单。
- 导出：服务端 `excelize` 生成（账单头 + 每条日志明细）。

### 7.7 市场价（FR-M）
```
每个分组: minCostPrice = MIN(channel.cost_price) WHERE group=? AND status=enabled
首页卡片（供应商+管理员可见）展示各分组当前最低价（匿名）。
```

### 7.8 权限矩阵
| 能力 | 供应商(5) | 管理员(10) | 超管(100) |
|---|---|---|---|
| 注册/登录、钱包、令牌、Chat（继承普通用户） | ✅ | ✅ | ✅ |
| 管理自己的渠道（增删改、设优先级、报价） | ✅(仅自己) | ✅(全部) | ✅ |
| 看他人渠道 | ❌ | ✅ | ✅ |
| 发起/撤销自己的结算 | ✅ | ✅ | ✅ |
| 市场价卡片 | ✅ | ✅ | ✅ |
| 供应商管理页（设供应商优先级/启用/结算配置） | ❌ | ❌ | ✅ |
| 结算申请审核、确认实付 | ❌ | （待定*） | ✅ |
> *FR-T3 明确"超管"确认结算；供应商管理页 FR-S3 也是超管。审核确认默认 **RootAuth**，若希望管理员也能审核可再调整。

---

## 8. 可落地实施计划（分阶段）

> 顺序：**P1 → P2 → (P3 ∥ P4) → P5**。每阶段独立可交付、可测试，单独走 spec→计划→实现→验收。

### P1 — 供应商身份 & 权限基础（地基）
- **数据模型**：`RoleSupplierUser=5`；User `+Phone`；新建 `Supplier` 表 + 迁移。
- **后端**：`SupplierAuth` 中间件；注册逻辑改为赋供应商角色 + 建 Supplier 记录 + 收 phone；供应商管理接口（list/搜索/编辑供应商优先级·启用·结算配置·备注，RootAuth）；`canManageTargetRole` 纳入新角色。
- **前端**：注册表单加手机号；`/_authenticated/suppliers` 路由 + 守卫；侧边栏菜单（admin 组，超管可见）；供应商管理页（列表+编辑抽屉，仿 users）；i18n 文案。
- **验收**：邮箱验证注册→自动成为供应商并能登录；超管在供应商管理页看到该供应商、可改其优先级/启用/结算方式；非超管访问该页被拒。
- **风险**：角色等值判断误伤（需审计 `role==Common`）；邮箱 SMTP 须先配好。

### P2 — 渠道归属 & 供应商自助
- **数据模型**：Channel `+SupplierId +CostPrice` + 迁移。
- **后端**：渠道接口加供应商作用域（list/get/add/update/delete 按 `supplier_id` 过滤，供应商创建时自动归属本人）；成本价校验（>0）；管理员可改任意渠道优先级。
- **前端**：供应商渠道页（复用完整渠道编辑器 + 成本价字段 + 仅显示自己的）；渠道列表展示成本价列。
- **验收**：供应商只见/改自己的渠道；新建渠道带成本价并归属本人；管理员可见全部并可调优先级。
- **风险**：完整编辑器暴露给供应商的字段安全（上游地址/模型由其自填，属其自有 key，风险可控；需确认不暴露跨供应商数据）。

### P3 — 供应商层级调度 + 可用性
- **后端**：扩展候选渠道排序（双模式）；全局 `DispatchStrategy` 开关；跨供应商重试；供应商级联禁用/恢复；定时健康检测任务（默认 30min，可配）；缓存扩展（携带 supplier 优先级/价格/启用）。
- **前端**：系统设置加调度策略开关 + 健康检测间隔/开关。
- **验收**：priority 模式按供应商→渠道优先级命中；bidding 模式命中分组最低价；某供应商全挂自动跳其他供应商;渠道恢复后自动启用;定时检测生效。
- **风险**：调度热路径性能（务必走缓存）；健康检测真实花费（频率可配）。

### P4 — 计费记账 & 市场价
- **后端**：供应商渠道 post-consume 记录 `OfficialUsd`（去 groupRatio）;按供应商/分组聚合查询;市场价查询接口。
- **前端**：首页"市场价"卡片（分组最低价，供应商+管理员可见）;供应商可见自己的待结算金额。
- **验收**：消耗后 `OfficialUsd` 正确（与官方价一致、不受 group 折扣影响）;首页按分组显示最低价且仅授权角色可见。
- **风险**：官方价计算口径（多模态/音频/补全倍率）需逐类型覆盖测试。

### P5 — 结算系统
- **数据模型**：`Settlement` 表;Log `+SettlementId`。
- **后端**：发起结算（打包日志+生成待审核账单）;撤销;超管审核列表 + 确认实付（¥/$、方式、备注）;自动结算定时（日/周/月 0 点，仅生成待审核）;账单详情;Excel 导出（excelize）。
- **前端**：「账单结算」页（待结算/历史/发起/撤销/手动·自动配置）;超管结算申请列表 + 详情;导出按钮。
- **验收**：发起→待审核→可撤销→超管确认→锁定明细;账单含周期/金额/实付/时间/方式/逐条日志;自动结算按周期到点生成;Excel 导出正确。
- **风险**：并发结算与日志归集的一致性（事务/水位）;撤销后日志正确释放。

---

## 9. 横切关注点
- **数据库兼容**（CLAUDE.md Rule 2）：所有迁移/查询须同时兼容 SQLite/MySQL/PostgreSQL;用 GORM 抽象,避免 DB 专有语法;新增列用 `ALTER TABLE ADD COLUMN`。
- **JSON**（Rule 1）：统一走 `common.Marshal/Unmarshal`。
- **品牌保护**（Rule 5）：不得移除/替换 new-api、QuantumNous 相关标识。
- **i18n**：后端 en/zh;前端 en/zh/fr/ru/ja/vi,新文案需补齐。
- **金额精度**：成本价与金额用 decimal 思路处理,避免浮点误差累积（结算金额建议定点/分计）。
- **测试**：遵循 TDD;调度、官方价记账、结算归集为重点测试对象。
- **安全**：供应商作用域强校验（IDOR 防护,任何渠道/账单操作校验归属）。

## 10. 风险与开放问题
| 风险/问题 | 说明 | 处置 |
|---|---|---|
| 官方价口径 | groupRatio 折扣、补全倍率、多模态计费差异 | P4 单独记账 + 分类型测试 |
| 调度性能 | 叠加供应商层 join | 缓存携带供应商字段 |
| 健康检测成本 | 真实消耗 key | 频率可配,默认 30min |
| 结算一致性 | 并发发起/撤销 | 事务 + SettlementId 标记 |
| 角色等值判断 | 误伤供应商 | P1 审计 `role==Common` |
| 审核权限归属 | 结算审核给超管还是管理员 | 默认超管(见 7.8 注) |

## 11. 待确认（开放）
- O1：结算审核确认权限——仅超管，还是管理员也可？（默认仅超管）
- O2：供应商管理页"价格"展示列——显示该供应商渠道的最低价 / 均价 / 区间？（默认区间）
- O3：自动结算周期变更后,是否影响已生成账单?（默认不影响,仅对未来周期生效）

---

*本文档为 v1 方案基线,经确认后逐阶段细化为实现计划。*
