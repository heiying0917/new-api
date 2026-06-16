# 官方价手册 + 一键填充官方价（方案二）

日期：2026-06-15
分支：feat/tokenki-p1a-supplier-backend
状态：已批准，开发中

## 背景与问题

导入渠道带进来的模型，需要在「分组与模型定价」里手动逐个设价，非常繁琐。根因诊断（已用本地库验证）：

1. **定价是全局按模型名生效**，存在 DB option（`ModelRatio`/`CompletionRatio`/`CacheRatio`/`CreateCacheRatio`/`ModelPrice`），与渠道无关。渠道的「模型」字段只声明能转发哪些模型，跟有没有配价无关。
2. **启动时 DB option 整表覆盖代码内置默认值**（`types/rw_map.go:88` 先 `make` 清空再 `Unmarshal`；`main.go:292` 先灌默认、`main.go:308` 再被 DB 覆盖）。所以代码升级新增的默认价，对已有库无效。
3. **现有「上游倍率同步」是纯手动多步工具**，从未被大批量应用过（本地库 `ModelRatio` 245 个 ≈ 代码默认 251 个，仅多手动加的 `gpt-5.5`；`CompletionRatio` 仅 4 个）。新模型只要不在内置默认表、又没人手动同步，就一直没价 → 计费落到 `37.5` 兜底倍率或被拒。

数据源核实：**models.dev** 收录各厂商官网价，2685 个模型、USD/1M token、更新极快（`claude-fable-5` 发布 6 天内收录，`claude-opus-4-8` 有真实价 input=$5/output=$25）。作为「官网价」权威源时效性足够。

## 目标

提供一个**官方价手册**（以 models.dev 为源，第一方官方价优先）+ **一键填充**动作，把官方价**合并写入**全局定价，解决"新模型没价/逐个手填"的问题。合并写入（read-modify-write）天然规避根因 2 的"整表覆盖"坑。

## 已批准的关键决策

1. **取价策略**：第一方官方源优先（claude→`anthropic`、gpt/o1/o3/o4→`openai`、gemini→`google`/`google-vertex`、deepseek→`deepseek`、grok→`xai` 等）；无第一方时回退"最低非零价"并标注非官方。
2. **匹配策略**：分层 ① 第一方精确 → ② 全网精确 → ③ 归一化兜底（去 `provider/` 前缀、去 `@...` 后缀、`.`→`-`、去分隔符）。归一化若有歧义（多个不同价且无第一方）→ 判"未匹配"，绝不瞎配。取价恒用第一方。
3. **填充策略**：默认"只补缺"（仅当前无 `model_ratio` 的模型）；另提供"刷新到最新官方价"（已匹配且官方价≠当前价，带 diff 预览确认）。
4. **填充语义**：填 1:1 官方原价进 `model_ratio`，加价交给分组倍率层，不揉进 model_ratio。

## 架构与数据流

```
models.dev /api.json
  │ 刷新手册：抓取 + 第一方优先抽取 + 单位换算 + 归一化索引
  ▼
官方价手册 Book（内存缓存 + 直写 options 表持久化，绕过 OptionMap）
  │ 预览：按范围圈定目标模型 → 分层匹配 → 算 diff
  ▼
预览（已匹配可勾选 / 未匹配清单）
  │ 应用：服务端按模型名从 Book 重算官方价 → 合并进 4 张 ratio map
  ▼
全局定价 option（merge，不碰未选中的 key）
```

## 后端

### 纯逻辑包 `service/official_pricing/`（无 model 依赖，便于单测）

- `types.go`：
  - `BookEntry{ Model, Provider string; FirstParty bool; ModelRatio float64; CompletionRatio,CacheRatio,CreateCacheRatio *float64 }`（可选字段用指针，缺失=nil 不写）
  - `Book{ Source string; FetchedAt int64; Entries map[string]BookEntry; normIndex map[string]normHit }`
  - `CurrentRatios{ ModelRatio,CompletionRatio,CacheRatio,CreateCacheRatio map[string]float64 }`
- `modelsdev.go`：解析 models.dev 原始 JSON（含 `cost.cache_write`）→ `BuildBook`。
  - 换算：`model_ratio = input*USD/1000`（`ratio_setting.USD=500`→input/2）、`completion=output/input`、`cache=cache_read/input`、`create_cache=cache_write/input`。
  - 每个 raw modelName 选一条候选：该名下有第一方 provider 则用之，否则最低非零 input（确定性 tiebreak）。
- `normalize.go`：`Norm(s)` + 构建 `normIndex`（每个 normKey 选 best：有第一方用第一方，否则价格一致则用之，价格冲突且无第一方→标 ambiguous）。`Match(book, model)` 分 4 层、**恒优先第一方**：① 第一方精确名 → ② 第一方归一化（让 `anthropic/claude-opus-4.8` 这类中转命名也落到 anthropic 官方价）→ ③ 任意精确名（中转）→ ④ 非歧义归一化（中转）→ 未匹配。
- `fetch.go`：`FetchModelsDev(ctx, timeout)` 拉取 `https://models.dev/api.json`（带重试），返回原始 JSON。
- `fill.go`（纯函数）：
  - `BuildPreview(book, targets []string, cur CurrentRatios, mode string) PreviewResult`（rows + unmatched）
  - `ApplyToMaps(book, models []string, cur CurrentRatios) (next CurrentRatios, changed map[string]bool, applied []string)`

### Controller `controller/official_pricing.go`（接 model / ratio_setting）

- 内存缓存 Book（package var + mutex），缺失时从 raw option 懒加载。
- `POST /api/pricing/official_book/refresh`：抓取→BuildBook→缓存+持久化(`OfficialPriceBookCache` 直写 options 表)→返回 meta。
- `GET  /api/pricing/official_book`：返回 meta（source、fetched_at、model_count、first_party_count）。
- `POST /api/pricing/official_book/preview`：body `{scope:{kind,channel_id,models}, mode}` → 解析目标模型（all_missing=`model.GetEnabledModels()`；channel=拆 `channel.Models`；models=显式）→ 读当前 4 map copy → `BuildPreview` → 返回 rows+unmatched+counts。
- `POST /api/pricing/official_book/apply`：body `{models:[]}` → 读当前 map → `ApplyToMaps` → 对 changed 的 map `model.UpdateOption(key,json)`（同时持久化 DB + 更新内存 ratio map）→ 返回 applied 数。

### model 新增

- `GetRawOptionValue(key) (string,error)` / `SaveRawOption(key,value) error`：直读写 options 表，不进 OptionMap（避免 GetOptions 下发大 blob）。

### 路由

`apiRouter.Group("/pricing")` + `RootAuth()`，挂上述 4 个端点。

## 前端 `web/default/src/features/system-settings/models/`

- 新增 `official-price-book.tsx` 卡片，接入 `ratio-settings-card.tsx`，与"上游倍率同步"并列：
  - 状态行：来源 models.dev · 收录 N · 最后刷新 时间 · `[刷新手册]`
  - 填充区：范围（所有缺价/选渠道/手动选）+ 模式（只补缺/刷新到最新）+ `[预览]`
  - 预览表：已匹配（可勾选，列 来源provider/官方价/当前价/动作）+ 未匹配清单
  - `[应用所选]` → apply → toast + 刷新定价表
- `api.ts` 增 4 个调用；i18n 文案走 `t()`。

## 边界与错误处理

- models.dev 抓不到 → 用缓存手册，提示"缓存于 X"；无缓存则报错引导先刷新。
- `input=0`（免费）→ ratio 0（合法）。
- 按次/图像/视频（无 per-token 价）→ 列"未匹配/暂不支持"，v1 只覆盖 token 计费；`model_price` 自动填留后续。
- 无第一方源 → 回退最低价并标注非官方（前端可见）。
- 归一化歧义 → 未匹配，不猜。

## 测试

`service/official_pricing/*_test.go`（fixture 用 models.dev 子集，纯函数无网络/DB）：
- `Norm()` 表驱动：`anthropic/claude-opus-4.8`、`claude-opus4-8`、`...@default`、region 前缀。
- `BuildBook`：第一方选取、换算（含 cache_write→create_cache_ratio）、无第一方回退最低价。
- `Match`：exact / normalized / 第一方优先 / 歧义→未匹配。
- `BuildPreview`：只补缺 vs 刷新到最新、未匹配归集。
- `ApplyToMaps`：**合并保留既有 key**（正是本次踩的坑）、只写存在的可选字段。
- `go build ./...` + 前端 typecheck/build 通过。

## 不做（YAGNI）

- 方案三（导入渠道即自动补价）：本期不做，但 fill 服务设计成可被钩子复用。
- 逐厂商爬虫、models.dev per-call 价自动填、改"DB 覆盖默认"的加载语义（merge 写库已绕开）。
