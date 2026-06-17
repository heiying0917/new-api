# TokenKi 第八版 · 供应商体系增强 — 设计方案

- **日期**: 2026-06-16
- **分支**: `feat/tokenki-p1a-supplier-backend`
- **定位**: 纯叠加增强,不改动现有计费/调度/结算核心口径。仅在「供应商管理 / 渠道管理 / 概览」三处增加能力。
- **前端主题**: 仅 `web/classic`(React 18 + Vite + Semi Design)。**不碰 `web/default`**。
- **关联**: [[tokenki-aggregation-project]];前序 P1–P5 已完成(见 `2026-06-13-tokenki-aggregation-design.md`)。

---

## 0. 背景与现状(file:line)

| 能力 | 现状 | 位置 |
|---|---|---|
| 供应商列表 | `GET /api/supplier/` + `/search`,**RootAuth(超管)**,返回 `SupplierListItem`(含 `pending_cny`/`settled_cny`),**固定 `id desc` 无排序** | `router/api-router.go:150`、`controller/supplier.go`、`model/supplier.go:51` |
| 待结算聚合 | `GetSupplierPendingStat(supplierId)` → `{official_usd, payable_cny, log_count}`(两步:渠道id → 日志 GROUP) | `model/settlement_query.go:13` |
| 已结算聚合 | `GetSupplierSettledStats(supplierId, now)` → `{today, last7, total}`(CNY,来自 `computed_cny`,status=已结算) | `model/supplier_stats.go:26` |
| 结算单 | status: `已申请=1` / `已结算=2` / `已取消=3`;字段 `official_usd / computed_cny / actual_amount / actual_currency / settle_method` | `model/settlement.go:8,25` |
| 结算创建(原子打包) | `model.CreateSettlement(supplierId, source, ts) (*Settlement, error)` — 把未结算日志原子打包成 `已申请` 单 | 供 `SupplierCreateSettlement` 复用,`controller/settlement.go:18` |
| 管理员结算 | `/api/admin/settlement/*`,**RootAuth(超管)**;有 list / confirm / cancel,**无"管理员主动发起结算"** | `router/api-router.go:195`、`controller/settlement.go:247`(confirm body: `actual_amount/actual_currency/settle_method/remark`) |
| 渠道列表 | `GET /api/channel/` + `/search`,**AdminAuth(管理员)**;支持 keyword/model/group/type/status/sort;**无供应商名过滤**;有 `supplier_id`(索引)+ 回填 `supplier_name` | `router/api-router.go:293`、`model/channel.go:40`、`controller/channel.go:73(backfillChannelSupplierNames)` |
| 渠道类型聚合 | `CountChannelsGroupByType()` → `map[type]count`(全量渠道);`GetGroupMarketPrices()` → `map[group]最低成本价` | `model/channel.go:1159,1183` |
| 渠道类型常量 | 58 个 `ChannelType*`;`constant.GetChannelTypeName(type)` | `constant/channel.go:180` |
| 供应商自助概览 | `GET /api/supplier/self/overview`(SupplierAuth);组件 `components/supplier-overview/index.jsx`,仅 `isSupplier()`(role===5)在 `/console` 展示,**管理员/超管看不到** | `controller/supplier_market.go`、`web/classic/src/components/dashboard/index.jsx:63` |
| 前端路由守卫 | `AdminRoute`(role≥10)、`SupplierRoute`(role===5)、`PrivateRoute` | `web/classic/src/helpers/auth.jsx:45` |
| 侧栏菜单 | adminItems(`isAdmin()` 可见)、supplierItems(`isSupplier()` 可见) | `web/classic/src/components/layout/SiderBar.jsx:209` |

**已锁定的判断点(评审可推翻):**
1. **权限分层**:概览页(只读)= **AdminAuth**(管理员+超管,满足需求4"管理员和超管都能看到");供应商管理页 + 立即结算 + summary(涉及资金/写)= **RootAuth**(超管),沿用现有 `/api/supplier`、`/api/admin/settlement` 口径,管理员不动钱。
2. **已结算的人民币口径**:用 `actual_amount` 按 `actual_currency` 拆分,¥结算为主数,$结算另附小字;**不臆造汇率**。
3. **排序**:服务端跨页排序(非仅当前页客户端排序)。

---

## 需求1 — 供应商管理页:排序 + 立即结算 + 顶部汇总

### 1.1 顶部汇总条

**新接口** `GET /api/supplier/summary`(RootAuth)。返回全局三组指标,每组「美金(实际消耗量) + 人民币(结算金额)」:

```jsonc
{
  "pending": {                 // 未结算日志(未打包进任何账单)
    "official_usd": 0,         // Σ official_usd
    "payable_cny": 0,          // Σ (official_usd × cost_price_snapshot)
    "supplier_count": 0,       // 有未结算日志的供应商数
    "log_count": 0
  },
  "applied": {                 // 结算单 status=已申请(1)
    "official_usd": 0,         // Σ official_usd
    "computed_cny": 0,         // Σ computed_cny
    "count": 0
  },
  "settled": {                 // 结算单 status=已结算(2)
    "official_usd": 0,         // Σ official_usd(=实际消耗量)
    "actual_cny": 0,           // Σ actual_amount where currency=CNY(实际结算金额)
    "actual_usd": 0,           // Σ actual_amount where currency=USD(以美元结算的,附小字)
    "computed_cny": 0,         // Σ computed_cny(应付,参考)
    "count": 0
  }
}
```

**实现(cross-DB,无 JOIN):**
- `pending`:① 取 `channels` 中 `supplier_id>0` 的 `(id, supplier_id)` 映射(一次查询);② `LOG_DB` 对 `type=consume AND settlement_id=0` 按 `channel_id` GROUP 聚合 `SUM(official_usd)`、`SUM(official_usd*cost_price_snapshot)`、`COUNT`;③ Go 内折叠到供应商维度(顺带得到 1.2 排序所需的 per-supplier pending)。`supplier_count` = 折叠后有量的供应商数。
- `applied` / `settled`:对 `settlements` 按 `status` GROUP 聚合;`settled` 的 `actual_*` 再按 `actual_currency` 分别 SUM(用 `common.UsingPostgreSQL` 等无关,纯 GORM `Group`/`Select`)。

**新增 model 函数**(放 `model/supplier_stats.go`):
- `GetAllSuppliersPendingStat() (map[int]SupplierPendingStat, SupplierPendingStat, error)` — 返回 per-supplier map + 全局合计(一次聚合两用)。
- `GetSettlementTotalsByStatus() (applied, settled SettlementTotals, error)`。

### 1.2 排序

`GET /api/supplier/`(及 `/search`)新增 query:
- `sort_by` ∈ `{priority, pending_cny, pending_usd, settled_cny}`;缺省维持 `id desc`(传 `priority` 等才排序)。
- `sort_order` ∈ `{asc, desc}`,缺省 `desc`。

**实现**:`GetAllSuppliers` 现为「DB 分页 → 逐个补 stats」。改为:当带 `sort_by` 且为计算列(pending/settled)时,先取全量 role=5 用户 id → 用 1.1 的 `GetAllSuppliersPendingStat` map + per-supplier settled map 组装可排序值 → Go 排序 → 内存分页。`sort_by=priority` 可直接 DB `ORDER BY priority`。供应商规模(几十~数百)下全量计算成本可接受;`pending` 聚合是单次 GROUP 查询。

**前端**:`SuppliersColumnDefs.jsx` 给「优先级 / 待结算 / 已结算」列加 `sorter: true`(服务端排序),`SuppliersTable`/`useSuppliersData` 接 `onChange` → 把 `sort_by/sort_order` 传给 `loadSuppliers/searchSuppliers`。

### 1.3 立即结算(生成待结算账单 + 打开确认弹窗)

**新接口** `POST /api/admin/settlement/initiate`(RootAuth),body `{ "supplier_id": 123 }`:
1. 校验该供应商存在且为 role=5。
2. 调 `model.CreateSettlement(supplierId, "manual", common.GetTimestamp())`(复用原子打包:`UPDATE logs SET settlement_id=billId WHERE 供应商渠道 AND settlement_id=0 AND type=consume` → 算 `official_usd/computed_cny/log_count` → 建 status=`已申请` 单)。
3. 写资金台账 `RecordSettlementLedger{Action:"create", OperatorIsAdmin:true, OperatorId:操作管理员}`。
4. 无未结算日志 → `CreateSettlement` 应返回明确错误"无待结算消费",透传给前端提示。
5. 返回新建 `Settlement{id, official_usd, computed_cny, log_count, ...}`。

**前端流程**:
- 抽取共享组件 `components/table/settlement-review/modals/ConfirmSettlementModal.jsx`(从结算审核页现有"确认结算"弹窗逻辑提炼;结算审核页改为复用它,行为不变)。入参 `{visible, settlement, onClose, onConfirmed}`,内部调 `POST /api/admin/settlement/:id/confirm`。
- 供应商管理页:每行「立即结算」按钮 → 调 `initiate` → 成功后用返回的 settlement 内联打开 `ConfirmSettlementModal`(管理员留在本页,可连续结算);确认成功 → 刷新列表 + 汇总条。取消弹窗 → 账单保留 `已申请`,可在结算审核页继续处理。
- 列定义在 `operate` 列追加按钮;`useSuppliersData` 增加 `initiateSettlement(supplierId)` + 弹窗 state。

### 1.4 顶部汇总条 UI

供应商管理页(`components/table/suppliers/index.jsx`)在 `CardPro` 的 `descriptionArea`/`statsArea` 上方加一行 3 张统计卡(复用概览页的紧凑 StatCard 风格):每卡主数 = 人民币,小字 = 美金 + 笔数。挂载 `useSuppliersData` 新增的 `summary` state(并行 `GET /api/supplier/summary`,与列表同刷新)。

---

## 需求2 — 渠道管理页:按供应商名模糊搜索

**后端** `GET /api/channel/` 与 `/api/channel/search` 新增 query `supplier_name`(模糊):
- 若非空:先 `users` 查 role=5 且 `username LIKE %kw% OR email LIKE %kw% OR phone LIKE %kw%` 得 `user_ids`(用 `commonGroupCol` 等无关,普通列);再对渠道追加 `supplier_id IN (user_ids)`;`user_ids` 为空 → 直接返回空集(短路)。
- 与现有 keyword/group/model/type/status 过滤是 AND 关系,可叠加。

**前端**:
- `components/table/channels/ChannelsFilters.jsx` 增加"供应商"`Form.Input`(placeholder「按供应商用户名/邮箱搜索」)。
- `hooks/channels/useChannelsData.jsx`:`formInitValues` 加 `searchSupplier`;`getFormValues` 提取;`searchChannels` 拼接 `&supplier_name=${encodeURIComponent(searchSupplier)}`。结果沿用已存在的 `supplier_name` 列展示。

---

## 需求3+4+5 — 新建管理员「供应商概览」页

### 3.1 后端

**新接口** `GET /api/admin/supplier-overview`(**AdminAuth**,满足需求4):

```jsonc
{
  "summary": {
    "supplier_total": 0,        // role=5 总数
    "supplier_enabled": 0,      // suppliers.enabled=true
    "channel_total": 0,         // supplier_id>0 的渠道总数
    "channel_available": 0,     // status=enabled
    "channel_unavailable": 0
  },
  "by_type": [
    {
      "type": 14,
      "type_name": "Anthropic",
      "supplier_count": 3,      // 提供该类型渠道的去重供应商数
      "channel_count": 8,
      "available": 7,
      "unavailable": 1,
      "lowest_price": 2.40,     // 该类型 enabled 且 cost_price>0 的最低成本价(¥/$)
      "groups": [              // 下钻用:该类型下各分组竞价梯队
        { "group": "claude-official", "lowest_price": 2.40,
          "bids": [ { "price": 2.40, "supplier_id": 12, "supplier_name": "a***" } ] }
      ]
    }
  ]
}
```

**实现**:
- 取 `channels` 中 `supplier_id>0` 的精简列 `(id, type, group, status, supplier_id, cost_price)`(一次查询),Go 内按 `type` 聚合:`channel_count`、`available/unavailable`(status)、`supplier_count`(去重 supplier_id 集合大小)、`lowest_price`(enabled 且 cost_price>0 的 min)。
- `summary` 由同一批数据 + `users` role=5 计数(`GetSupplierCount`/现成统计)得出。
- `by_type` 仅含「有供应商渠道」的类型;`type_name` 用 `constant.GetChannelTypeName`。
- `groups`/`bids` 复用 `GetGroupMarketPrices` 思路按 type+group 聚合;供应商名按现有匿名化策略脱敏(`a***`)。下钻数据可在同接口返回(数据量小)或后置懒加载;**MVP 一次返回**。

**新增 model 函数**(`model/supplier_stats.go` 或 `model/supplier_aggregation.go`):`GetSupplierOverview() (SupplierOverview, error)`。

### 3.2 前端

- 新页面 `web/classic/src/pages/SupplierOverviewAdmin/index.jsx`(+ 数据 hook `hooks/supplier-overview-admin/useSupplierOverviewData.jsx`)。
- **路由**:`App.jsx` 新增 `lazy(SupplierOverviewAdmin)`,路径 `/console/supplier-overview`,包 `<AdminRoute>`。
- **侧栏**:`SiderBar.jsx` adminItems 增「供应商概览」→ `/console/supplier-overview`,`isAdmin()` 可见;`routerMap` 加映射。
- **布局**(需求5「紧凑卡片 + 响应式多列网格」):
  - 顶部:汇总统计卡(供应商总数/启用、渠道可用/不可用)。
  - 主体:`Row gutter` + `Col xs=12 sm=8 md=6 lg=4`(大屏每行约 6 张,中屏 3~4 张,移动 2 张)的**紧凑卡片网格**,一卡一种官key类型:类型名(可带渠道 logo)+ 供应商数 + 渠道数 + 最低价 + 可用状态圆点。十几~几十种类型整齐换行,无横向溢出。
  - 交互:点卡片 → `SideSheet`/`Modal` 展示该类型 `groups` 竞价梯队 + 供应商明细(复用 `supplier-overview` 现有梯队条样式)。
  - 空态:无供应商渠道时 `Empty`。
- 紧凑卡片样式从现有 `components/supplier-overview/index.jsx` 的 `StatCard`/`BidCard` 提炼复用,避免重复。

> 注:现有供应商自助概览(`supplier-overview` 的 `BidCard`,`lg=8` 3 列偏宽)同样存在"卡片偏宽"问题。本期**优先交付管理员概览页的紧凑网格**;自助概览的同款收窄列为可选优化(stretch,不阻塞)。

---

## 数据流总览

```
[需求1 汇总条]  前端 useSuppliersData ──GET /api/supplier/summary──▶ GetAllSuppliersPendingStat + GetSettlementTotalsByStatus
[需求1 排序]    列头 sorter ──sort_by/sort_order──▶ GetAllSuppliers(计算列→全量算→内存分页)
[需求1 立即结算] 行按钮 ──POST /admin/settlement/initiate──▶ CreateSettlement(已申请) ──▶ 前端 ConfirmSettlementModal ──POST /admin/settlement/:id/confirm──▶ 已结算
[需求2]         ChannelsFilters 供应商输入 ──supplier_name──▶ users(role=5,LIKE)→supplier_id IN ──▶ 渠道过滤
[需求3/4/5]     新页面 ──GET /api/admin/supplier-overview(AdminAuth)──▶ GetSupplierOverview ──▶ 紧凑卡片网格 + 下钻
```

---

## 错误处理与边界

- `initiate`:无待结算日志 → 明确错误文案;并发重复点击 → `CreateSettlement` 内事务保证不重复打包(沿用 P5 原子幂等)。
- `supplier_name` 搜索:空匹配短路返回空集,不报错。
- 汇总/排序的全量计算:供应商规模上千时关注性能;`pending` 为单次 GROUP 查询,settled 为单次 GROUP 查询,可接受;若未来超大规模再加缓存(本期不做)。
- 概览页 `by_type` 仅含有供应商渠道的类型,避免 58 类型空卡铺屏。
- 已结算美元口径:`actual_usd>0` 时卡片附「另含 $X 以美元结算」小字,不并入 ¥ 主数。

## 数据库兼容(SQLite / MySQL≥5.7.8 / PostgreSQL≥9.6)

- 全部用 GORM `Select/Where/Group/Order/Pluck`,无方言函数;`LIKE` 三库通用。
- 不新增表、不改列;仅新增只读聚合查询 + 一个复用既有写逻辑的 initiate 接口。
- 金额用 `COALESCE(SUM(...),0)`(已是现有模式)。

## 测试计划

**后端(Go,`go test ./...`):**
- `GetAllSuppliersPendingStat`:多供应商/多渠道/含已结算与未结算日志,验证 per-supplier 与全局合计一致;无渠道供应商为 0。
- `GetSettlementTotalsByStatus`:applied/settled 分桶、actual_currency 拆分 CNY/USD 正确。
- `GetSupplierOverview`:by_type 计数(去重供应商、可用/不可用、最低价)与构造数据吻合;无供应商渠道返回空 `by_type`。
- admin `initiate`:有未结算→建已申请单 + 台账 create(OperatorIsAdmin=true);无未结算→错误;非供应商 id→错误。
- 渠道 `supplier_name` 过滤:命中/不命中/与其他过滤叠加。
- 排序:`sort_by=pending_cny&sort_order=asc/desc` 顺序正确。
- 复用 PG+Redis 本地栈跑(见 [[local-deploy-db-connection]]),并保证 SQLite 单测通过。

**前端:**
- `bun run build` + typecheck 通过(`web/classic`)。
- Playwright 实测 5001:汇总条三指标显示、列排序、立即结算→确认弹窗→已结算;渠道页供应商搜索过滤;新概览页紧凑网格在窄/宽屏换行正常、点卡下钻。

## 交付纪律

- 实现→测试/typecheck/build 通过→评审→**停下汇报,等用户明确指令再 commit**(全局铁律:未授权不 commit/push/部署)。
- 每完成一部分追加 `docs/superpowers/WORKLOG.md`(何时/做了什么/改了哪些文件/如何验证),见 [[worklog-discipline]]。
- 受保护标识(new-api / QuantumNous)一律不动。

## 不做(YAGNI / 本期排除)

- 自动结算 cron(沿用 P5 stretch 状态)。
- 自助概览卡片收窄(可选优化,不阻塞)。
- 概览页 by_type 的后置懒加载(MVP 一次返回即可)。
- 汇总/排序的缓存层(规模到瓶颈再加)。
- USD↔CNY 汇率换算(按币种分列展示,不臆造汇率)。
