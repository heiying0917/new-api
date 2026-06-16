# TokenKi 第九版 · 供应商渠道列表复用管理员 + 获取模型列表修复 — 设计方案

- **日期**: 2026-06-16
- **分支**: `feat/tokenki-p1a-supplier-backend`
- **定位**: 让供应商管理「自己的渠道」时,获得与管理员**完全一致**的列表体验(同样的列、同样的行操作),仅去掉「创建者」、加上「成本 / 应收款」两列;并修复新建/编辑渠道时「获取模型列表」对多类型失败的问题。
- **前端主题**: 仅 `web/classic`(React 18 + Vite + Semi Design)。**不碰 `web/default`**。
- **关联**: [[tokenki-aggregation-project]];前序第八版 `2026-06-16-tokenki-v8-supplier-enhancements-design.md`。

---

## 用户确认的决策(2026-06-16 Q&A)

1. **操作完全对等**:供应商在自己渠道上拥有管理员列表的**全部**行操作(编辑/删除/复制/测试+模型测试/启禁用/获取模型/多Key管理/检测+应用上游模型更新/刷新余额),均以「本人渠道」为边界。
2. **优先级/权重**:在供应商列表中**展示且可编辑**(用户已接受"供应商可抬高自己流量份额"的取舍;护栏为后续可选项,本期不做)。
3. **获取模型列表**:必须对 Anthropic / AWS / OpenAI / OpenRouter / 自定义 URL 均可用(需求2,已先行修复见下文 §3)。

---

## 需求1 — 供应商渠道列表完全复用管理员列表

### 现状(file:line)

| 项 | 现状 | 位置 |
|---|---|---|
| 供应商渠道页 | 自建精简表 `SupplierChannelsTable`(仅 名称/类型/分组/成本价/官方计费/应收款/状态 + 编辑/删除) | `pages/SupplierChannels/index.jsx` → `components/table/supplier-channels/{index,SupplierChannelsTable,SupplierChannelsColumnDefs}.jsx` |
| 供应商数据钩子 | `useSupplierChannelsData`(仅 list/get/create/update/delete + keyword) | `hooks/supplier-channels/useSupplierChannelsData.jsx` |
| 供应商列表接口 | `GET /api/supplier/channel/`(仅 keyword);返回 Channel + `OfficialUsd`/`Receivable` | `controller/supplier_channel.go:31`;`router/api-router.go:165` |
| 管理员渠道页 | 全功能 `ChannelsPage` + `getChannelsColumns` + `useChannelsData`(无 mode) | `components/table/channels/{index,ChannelsTable,ChannelsColumnDefs,ChannelsFilters}.jsx`、`hooks/channels/useChannelsData.jsx` |
| 管理员查询构造器 | `buildChannelListQuery(group, statusFilter, typeFilter, supplierIds)` 已支持 supplierIds 过滤 | `controller/channel.go:116` |
| 编辑弹窗 | 共享 `EditChannelModal`,已支持 `apiMode='supplier'`(`isSupplierMode`) | `components/table/channels/modals/EditChannelModal.jsx:167` |
| 供应商归属校验范式 | `c.GetInt("id")` → `GetChannelById` → `if ch.SupplierId != supplierId { 拒 }` | `controller/supplier_channel.go:67,118,153` |

### 架构:前端「mode 参数化」+ 后端「供应商作用域接口」

**方案 A(已选)**:把管理员那套组件加一个 `mode: 'admin' | 'supplier'`,供应商页变成薄壳 `<ChannelsPage mode="supplier" />`。**真正复用同一套组件**,管理员后续改动自动同步供应商端。已有先例 `EditChannelModal` 的 `apiMode`。后端**不**让供应商复用管理员路由(否决"共享 admin 路由"方案——会把 40+ 路由变成提权面),而是新增一批薄的、带归属校验的供应商作用域接口。

### 1.1 前端改动(`web/classic`)

- **`hooks/channels/useChannelsData.jsx`**:`export const useChannelsData = (mode = 'admin')`。新增 `const apiBase = mode === 'supplier' ? '/api/supplier/channel' : '/api/channel'`,把全部 `/api/channel/...` 端点串统一改为基于 `apiBase` 构造(列表/搜索/更新/删除/批量删/全测/单测/全余额/单余额/fix/copy/tag/multi_key/upstream_updates/fetch_models)。供应商模式:
  - 列表/搜索:`GET {apiBase}/` 与 `{apiBase}/search`,不传 `supplier_name`(供应商端无此筛选);后端强制 `supplier_id=本人`。
  - 上游模型更新子钩子 `useChannelUpstreamUpdates`:同样接受 `apiBase`(detect/detect_all/apply/apply_all)。
  - 多Key弹窗 `MultiKeyManageModal`:`multi_key/manage` 端点按 `apiBase`。
- **`components/table/channels/ChannelsColumnDefs.jsx`**:`getChannelsColumns(... , mode)`。供应商模式:
  - **移除**「创建者」列。
  - **新增**「成本」(`cost_price`,¥ 前缀)与「应收款」(`receivable`,¥、2 位小数)两列(数据来自供应商列表接口已回填的 `cost_price`/`receivable`)。
  - 其余列(ID/名称/分组/类型/状态/响应时间/已用·剩余/优先级/权重)保持一致;优先级/权重保留内联可编辑。
- **`components/table/channels/ChannelsFilters.jsx`**:接受 `mode`,供应商模式隐藏「供应商名」筛选,保留 关键词/分组/模型/类型/状态。
- **`components/table/channels/index.jsx`(`ChannelsPage`)**:接受 `mode='admin'` 默认;把 `mode` 透传给 `useChannelsData`、`getChannelscolumns`、`ChannelsFilters`、`ChannelsTable`、`EditChannelModal(apiMode)`。
- **`pages/SupplierChannels/index.jsx`**:改为渲染 `<ChannelsPage mode="supplier" />`。
- **退役**:`components/table/supplier-channels/*` 与 `hooks/supplier-channels/useSupplierChannelsData.jsx`(确认无其它引用后删除)。
- **侧栏/路由**:供应商端菜单项与路由不变(仍指向 `pages/SupplierChannels`),仅其内部实现切换。

### 1.2 后端改动

**列表(扩展筛选,复用管理员查询构造器):**
- `SupplierListChannels` / 新增 `SupplierSearchChannels`:读取与管理员一致的 query(`group`/`model`/`type`/`status`/`keyword`/分页/排序),用 `buildChannelListQuery(group, status, type, nil)` 之上**强制** `.Where("supplier_id = ?", c.GetInt("id"))` 构造,供应商无法越权改范围。沿用第八版的分页/排序/`Omit("key")` 模式,并继续回填 `OfficialUsd`/`Receivable`。
- 复用现有 `model` 查询能力;必要时新增 `GetChannelsBySupplierFiltered(...)`,而非散落重复筛选逻辑。

**全部行操作的供应商作用域接口(每个都是薄包装:`GetChannelById` → 校验 `SupplierId==me` → 调用既有 ownership-agnostic 核心):**

挂在 `/api/supplier/channel`(`SupplierAuth`)下新增:
| 操作 | 新供应商路由 | 复用的核心 |
|---|---|---|
| 测试单渠道 | `GET /test/:id` | `testChannel(channel,...)`(channel-test.go:77) |
| 复制渠道 | `POST /copy/:id` | `GetChannelById` + `clone.Insert()`(channel.go:1268 逻辑),强制 clone.SupplierId=me、CreatedBy=me |
| 刷新余额 | `GET /update_balance/:id` | `updateChannelBalance(channel)`(channel-billing.go:359) |
| 多Key管理 | `POST /multi_key/manage` | `ManageMultiKeys` 内联逻辑(channel.go:1346),body `channel_id` 先校验归属 |
| 检测上游模型更新 | `POST /upstream_updates/detect` | `checkAndPersistChannelUpstreamModelUpdates`(body `id` 校验归属) |
| 应用上游模型更新 | `POST /upstream_updates/apply` | `applyChannelUpstreamModelUpdates`(body `id` 校验归属) |
| 批量检测/应用 | `POST /upstream_updates/detect_all`、`/apply_all` | 复用核心,但渠道集合限定 `supplier_id=me`(新增 `findEnabledSupplierChannelsAfterID` 或在现有基础加 supplier 过滤) |

- 启禁用、优先级、权重:走**已有**的 `SupplierUpdateChannel`(`PUT /api/supplier/channel/`),前端在供应商模式把"启用/禁用/改优先级/改权重"统一走该接口(与管理员用 `PUT /api/channel/` 改 status/priority/weight 同构)。需确认 `SupplierUpdateChannel` 接受 `status`/`priority`/`weight`/`channel_info` 字段且只校验归属、不被滥用(如不允许改 supplier_id、created_by——现已锁定)。
- 全量类操作(全测 `/test`、全余额 `/update_balance`、fix、批量删 `/batch`、disabled 清理)在供应商模式:**限定到本人渠道**;`detect_all`/`apply_all` 同理。其余如 tag(分组打标)按管理员同款提供(供应商对自己渠道打 tag 合理)。

**安全铁律**:每个供应商接口入口都必须 `SupplierId == c.GetInt("id")` 校验;批量/全量接口的查询必须带 `supplier_id=me`;禁止任何"按 id 直接操作而不校验归属"的路径。

### 1.3 数据库兼容

- 仅新增只读筛选(复用 `buildChannelListQuery`,全 GORM,无方言)与复用既有写逻辑;不新增表、不改列。
- 三库(SQLite / MySQL≥5.7.8 / PostgreSQL≥9.6)通用;保留字 `group` 用既有 `ApplyChannelGroupFilter` / `commonGroupCol`。

---

## 需求2 / §3 — 获取模型列表修复(已先行实现并部署验证)

> 本项在本设计成文前已实现 + 部署 `juhe-v11fetchfix` + 端到端实测通过,此处仅作记录。

- **根因**:新建路径 `fetchModelsByParams` 对除 Ollama/Gemini 外一律 `Authorization: Bearer + {base}/v1/models`,与编辑路径 `fetchChannelUpstreamModelIDs`(按 provider 定制头/路径)分叉退化。Anthropic 需 `x-api-key`+`anthropic-version`(实测 Bearer→401);AWS 无 `/v1/models`、默认 base 空。
- **改法**:新增 `fetchModelIDsForDisplay(channel)`:先调按-provider 定制的核心,失败/空时回退 `relay.GetAdaptor(common.ChannelType2APIType(type)).GetModelList()` 静态列表;`fetchModelsByParams` 构造临时渠道委托之;编辑两 handler 同改。刻意与"上游模型更新检测"分离(后者保严格错误语义)。
- **验证**:`controller/fetch_models_test.go` 3 条确定性测试(Anthropic 头/AWS 兜底/OpenAI Bearer);真机 API 实测空 base→真实 Anthropic 返回 8 模型;部署后线上 HTTP 实测 Anthropic+AWS 均 `success:true`。

---

## 测试计划

**后端(`go test ./...`,SQLite + PG 双跑):**
- 每个供应商作用域接口:本人渠道成功 / 他人渠道返回 forbidden(归属校验)/ 不存在渠道报错。
- 批量/全量(detect_all/apply_all/test/update_balance):仅作用于本人渠道,绝不触碰他人渠道(构造跨供应商数据验证)。
- 供应商列表筛选:group/model/type/status/keyword 与 `supplier_id=me` 叠加正确;供应商无法通过参数越权看到他人渠道。
- 复制:clone 强制归属本人、created_by 本人。

**前端:**
- `cd web/classic && bun run build` 通过。
- Playwright 实测 5001(经授权):供应商登录 → 渠道页列与管理员一致(无创建者、有成本/应收款)、行操作齐全且仅作用于本人渠道;管理员渠道页**无回归**(mode 默认 admin)。

---

## 不做(YAGNI / 本期排除)

- 优先级/权重的滥用护栏(范围钳制 / 管理员覆盖)——用户已选"可编辑",护栏后续按需。
- `web/default` 任何改动。
- 供应商跨账号的任何可见性。

## 交付纪律

- 实现 → 测试/typecheck/build 通过 → 评审 → **停下汇报,等用户明确指令再 commit / push / 部署**(全局铁律)。
- 每完成一部分追加 `docs/superpowers/WORKLOG.md`(何时/做了什么/改了哪些文件/如何验证),见 [[worklog-discipline]]。
- 受保护标识(new-api / QuantumNous)一律不动。
