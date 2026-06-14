# TokenKi 核心安全整改实施计划 — 官key管理 / 计费 / 结算（零差错版）

> **For agentic workers:** 用 superpowers:subagent-driven-development 或 executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。
> **配套**：威胁与方案见 `docs/superpowers/specs/2026-06-14-tokenki-security-architecture.md`（含运营者确认的两处「设计精化」）。

**目标**：在**不弄坏任何核心业务**的前提下，修复三块「碰钱/碰密钥、绝不能错」的命门——① 官key/token 静态加密；② 计费成交价逐条快照；③ 结算累加 + 原子幂等 + 资金账本。

**范围（只做核心钱/密钥，其余 P1/P2 暂不在本计划）**：计费快照、结算完整性、密钥加密与 secret 治理。
**非目标**：限流/会话/CAPTCHA/可观测等（在总方案路线图，另行计划）。

**运营者已确认决策（2026-06-14）**：
- **B 存量日志不回填**：当前均为测试日志，忽略存量；只保证**新日志冻结当时单价**。删除 B4 回填任务与「旧日志兜底」逻辑（旧测试日志 snapshot=0，结算自然计为 0，不影响）。
- **C5 无退款链路**：跳过退款冲正。
- **C2b 日志库与主库同库**：`CreateSettlement` 打包用 `DB.Transaction` 事务，无需跨库降级。权限不变：超管见全部日志，供应商仅见自己相关日志。

**技术栈约束（每条都必须遵守）**：Go/Gin/GORM；JSON 走 `common.Marshal/Unmarshal`；DB 三库兼容（SQLite/MySQL/PG），迁移只用 `ALTER TABLE ADD COLUMN`；日志库走 `LOG_DB`；保留 new-api / QuantumNous 品牌（Rule 5）。

---

## 总则：保障核心零差错的 9 条铁律（每个任务都要回答）

1. **只增不改不删**：迁移只 `ADD COLUMN`（带默认值），**绝不** `ALTER COLUMN`/`DROP`/改类型/改语义。旧列在新逻辑**完全验证 + 灰度满**之前一直保留；清理留到独立的「收尾发布」。
2. **特性开关 + 默认兼容现状**：每个行为变更包一个 env 开关（仿 `common/init.go` 既有 `GetEnvOrDefaultBool`），默认值 = **现状行为**；分环境逐步打开；可一键回滚。
3. **影子计算先于切换**：碰钱的改动（结算金额、计费快照）先「新旧两种算法并行计算 + 落差异日志」跑一个观察期，**确认新旧一致**再让新算法成为权威。
4. **幂等 + 批量 + 不长锁**：所有回填/迁移可重复执行、分批（如每批 500 行）、单批短事务，**绝不**全表阻塞。
5. **加密双读**：密文带版本前缀（如 `enc:v1:`）；解密侧无前缀即视为遗留明文原样返回——实现读兼容、可灰度、可回滚。
6. **money 不可篡改留痕**：结算确认/撤销/打款写 append-only 账本 + 审计；金额最终走整数最小单位（micro-CNY/micro-USD）消除浮点漂移。
7. **三库 + 并发 + 钱算式 三类测试缺一不可**：每个碰钱/碰锁的任务必须有：跨库一致性测试、并发竞态测试、金额不变量（含套现回归）测试。
8. **先建安全网，再动核心**：Phase A（特性开关 + 现状特征测试 + 金额/并发测试脚手架）必须**先完成**，作为后续一切改动的回归基线。
9. **绝不自动 push/部署**：每阶段 实现→测试→对抗评审→提交（功能分支）；push/发布/部署等用户明确指令。

---

## 阶段总览与依赖

```
A 安全网（特性开关 + 现状特征测试 + 测试脚手架）   ← 一切的前置
        │
        ├─ B 计费逐条单价快照 + 结算改累加          ← 直接拆掉「改价套现」Critical（不依赖密钥）
        │        │
        │        └─ C 结算原子幂等 + 资金账本        ← 双重付款 / 不可对账
        │
        └─ D 密钥 hygiene（secret 外置/固定/分离）   ← E 的前置（cheap，零迁移）
                 │
                 └─ E 官key/token 静态加密（信封 + 双读 + 盲索引 + 灰度回填）
```

建议落地顺序：**A → B → C →（D → E）**。B/C 与 D/E 互不依赖，可并行评审、串行合并。最高风险收益比：B（拆套现）与 D（堵 DB 凭据/备份现实泄露）。

---

## Phase A — 安全网先行（无行为变更，纯加固测试能力）

**Files:**
- Create: `common/feature_flags.go`（或直接在 `common/init.go` 增 env，下同）
- Create/Test: `model/settlement_money_test.go`、`service/text_quota_money_test.go`
- 参考：`model/settlement.go`、`service/text_quota.go:460-490`、`common/init.go`

- [ ] **A1 现状特征测试（characterization）**：为「当前」结算金额算法写测试，**锁定现状行为**（含已知缺陷），后续改动若意外改变非目标行为即报警。
  - 用例：构造若干消费日志（不同 channel、official_usd），跑 `CreateSettlement`，断言 `ComputedCNY == Σ(渠道 official_usd × 当前 cost_price)`（现状公式）。
- [ ] **A2 跨库测试矩阵**：测试在 SQLite（默认）跑通；并提供 MySQL/PG 的 DSN 环境变量开关跑同一套（CI 三库）。确认金额/SUM 在三库一致。
- [ ] **A3 并发测试脚手架**：可对同一供应商并发触发 `CreateSettlement` / `ConfirmSettlement` 的测试工具（goroutine + WaitGroup），用于 C 阶段验证。
- [ ] **A4 特性开关骨架**：在 `common/init.go` 增 `BillingSnapshotEnable`、`SettlementAtomicEnable`、`SecretEncryptionEnable`、`SecretEncryptionWriteEnable` 等 env（默认全 false=现状），供后续阶段挂载。
- [ ] **A5 运行 + 提交**：`go test ./... && go build ./...` 通过 → 提交 `test(security): 核心钱路径特征/并发/跨库测试基线 + 特性开关骨架`。

**验收**：测试基线绿；开关默认关时行为与今日**逐字节一致**。

---

## Phase B — 计费逐条单价快照 + 结算改累加（拆掉「改价套现」Critical）

**核心**：成交价在**消费写日志那一刻**冻结到该条日志；结算**按条累加**，不再活取现价。

**Files:**
- Modify: `model/log.go`（Log 结构 +列；RecordConsumeLog 参数+赋值，:56-57,:222,:266）
- Modify: `service/text_quota.go`（:460-490 注入快照）
- Modify: `model/settlement.go`（:78-96 改累加；含影子计算）
- Modify: 前端使用日志/结算明细展示（`web/classic` 对应组件，逐条 ¥）
- Test: `model/settlement_money_test.go`、`service/text_quota_money_test.go`

- [ ] **B1 加列（additive）**：`logs` 新增 `cost_price_snapshot`（GORM tag `gorm:"default:0"`，`LOG_DB.AutoMigrate(&Log{})` 自动加列，三库兼容）。**默认 0** 表示「无快照（旧日志）」。
- [ ] **B2 消费时冻结单价**：`service/text_quota.go` 在已算出 `officialUsd`（:466）处，取**当前请求所用渠道的** `cost_price`（从 relay 上下文/渠道对象），写入 log 参数 `CostPriceSnapshot`。
  - ⚠️ 必须用「此刻渠道生效的 cost_price」，不是结算时的。多 key/级联渠道用实际承载本次请求的渠道。
  - 渠道无 cost_price（非供应商渠道）→ 快照存 0，结算时按现状(0=不计供应商应收)处理。
- [ ] **B3 结算影子计算（先不切换，铁律 3）**：`CreateSettlement` 在现状公式旁，**额外**算 `SELECT COALESCE(SUM(official_usd*cost_price_snapshot),0) WHERE settlement_id=?`，把「新算法值 vs 旧算法值」差异 `SysLog`（或落临时列）。开关 `BillingSnapshotEnable=false` 时仍用旧值。观察期确认：**新装机后产生的账期**新旧应一致（旧日志快照=0 的账期会不同，属预期）。
- [ ] **B4 切换权威 + 旧日志兜底**：确认无误后 `BillingSnapshotEnable=true`：
  - 结算金额改用累加式（删除 :91-96 的「按渠道活取现价」循环）。
  - **旧日志（snapshot=0 且 created_at < 切换时间戳 `BILLING_SNAPSHOT_CUTOVER`）**：一次性回填脚本按「该渠道在切换时刻或账期内的单价」冻结（运营确认口径）；**绝不**用「当前价」。回填走 Phase A 的幂等批量模式。
  - 附带修复：单价随日志走，渠道被删不再导致该段结算成 0。
- [ ] **B5 逐条 ¥ 展示**：使用日志 / 结算明细每行展示 `¥ = official_usd × cost_price_snapshot`；账单总额 = 明细之和（所加即所见）。
- [ ] **B6（推荐，可与 B 合并或紧随）整数金额**：把快照与累加改为**整数最小单位**（micro-CNY=¥×1e6），逐条整数求和精确、免漂移。若本阶段先用 float，记 TODO 到 C 的账本阶段统一。
- [ ] **B7 测试（必须含套现回归）**：
  - **套现回归**：消费 N 条（snapshot 已冻结）→ 改高渠道 cost_price → `CreateSettlement` → 断言金额 = Σ(逐条 official_usd×快照价)，**不受改价影响**。
  - 跨库 SUM 一致；渠道删除后该段仍正确结算；snapshot=0 旧日志兜底口径正确。
- [ ] **B8 提交**：`feat(billing): 成交价逐条快照 + 结算累加（拆改价套现）+ 跨库/套现回归测试`。

**回滚**：`BillingSnapshotEnable=false` 即回旧算法；新列保留无害。

**验收**：套现回归测试通过；影子观察期新装机账期新旧一致；前端逐条 ¥ 与总额相符。

---

## Phase C — 结算原子性 + 幂等 + 资金账本（堵双重付款 / 可对账）

**Files:**
- Modify: `model/settlement.go`（CreateSettlement 加锁；ConfirmSettlement/CancelSettlement 条件原子化，:108-146）
- Create: `model/settlement_ledger.go`（append-only 账本 + 迁移）
- Create: `service/audit.go`（财务动作审计埋点，最小骨架）
- Test: 并发用例（用 A3 脚手架）

- [ ] **C1 确认/撤销条件原子化**：把 `ConfirmSettlement`（:127）/`CancelSettlement`（:108）改为**条件 UPDATE + 检查 RowsAffected**：
  ```go
  res := DB.Model(&Settlement{}).
      Where("id = ? AND status = ?", id, SettlementStatusApplied).
      Updates(map[string]interface{}{"status": SettlementStatusSettled, ...})
  if res.RowsAffected != 1 { return errors.New("结算状态已变更，请刷新重试") }
  ```
  消除「先 First 再 Save」的 TOCTOU（两个超管并发 confirm → 只有一个成功打款）。
- [ ] **C2 发起防双重打包**：`CreateSettlement` 对「同一供应商建单」加互斥——
  - 首选：把第 66-69 的「占位建单 + UPDATE logs settlement_id」包进**同库事务**（确认 `DB` 与 `LOG_DB` 同库时用 `DB.Transaction`；不同库时见 C2b）；
  - 并发闸：每供应商一把锁（Redis `SETNX supplier:settle:<id>` 双模，或 PG advisory lock），防两个并发 CreateSettlement 各自打包重叠日志。
  - [ ] **C2b 跨库（LOG_SQL_DSN）告警**：若日志库与主库不同库，事务不可跨库——记录该限制，退化为「供应商级 Redis 锁 + 打包后校验 settlement_id 无重叠」。
- [ ] **C3 append-only 资金账本**：新增 `settlement_ledger`（id, settlement_id, action(create/confirm/cancel), supplier_id, official_usd, computed_cny, actual_amount, currency, operator_id, snapshot_hash, created_at），**只插不改不删**；confirm/cancel/create 各写一条。`snapshot_hash` = 账单关键字段哈希，事后可验账单未被篡改。
- [ ] **C4 财务审计埋点**：`service/audit.go` 统一记录 confirm/cancel/改实付/解锁 等敏感动作（operator、前后值、时间），写独立审计表（与业务表分离）。
- [ ] **C5 退款冲正（如适用）**：评估退款是否冲销 `official_usd`/快照收益；若有退款链路，结算净额应扣除——记录设计，按需实现。
- [ ] **C6 测试**：并发 confirm 只成功一次（无重复打款）；并发 create 无重叠打包；账本每动作一条且 hash 可校验；幂等重试安全。
- [ ] **C7 提交**：`feat(settlement): 状态机原子幂等 + 供应商级建单锁 + append-only 资金账本 + 财务审计`。

**回滚**：账本/审计为新增表（旁路），可独立关闭；原子 UPDATE 是纯增强，无需回滚。

**验收**：并发付款不重复；账本与审计完整；跨库限制有明确降级与告警。

---

## Phase D — 密钥管理 hygiene（E 的前置，零迁移、半天级）

**Files:** `docker-compose.yml`、`common/init.go`、`common/constants.go:75-76`、`.env.example`(新建)

- [ ] **D1 compose 弱口令外置**：`docker-compose.yml` 把 `POSTGRES_PASSWORD`/`SQL_DSN`/`REDIS` 密码改为 `${PG_PASSWORD:?set me}` 等 env 引用；新增 `.env.example`（不含真值）；停用字面 `123456`。**不删任何服务/网络/卷**。
- [ ] **D2 secret 固定化 + 分离**：
  - `SESSION_SECRET`/`CRYPTO_SECRET`：`common/init.go` 已读 env；把 `constants.go:75-76` 的 `uuid.New()` 默认改为「未设则启动**显著告警**；生产模式(`DEPLOY_MODE=production` 或 `REQUIRE_STRONG_SECRETS=true`)缺失则 `log.Fatal`」（仿现有对 `random_string` 的拦截，init.go:51-54）。本地/dev 不阻断。
  - 新增独立 `SECRET_ENCRYPTION_KEY`（E 阶段加密用），**与 SESSION/CRYPTO 分离**（打破 init.go:62 的共用）；缺失时 `SecretEncryptionEnable` 不得开启。
- [ ] **D3 暴露面顺手收敛（可选并入）**：GORM Debug SQL 二级开关（防 key 进日志）、错误信息不带 key。
- [ ] **D4 提交**：`chore(security): compose 密钥外置 + secret 固定化与加密密钥分离 + 生产 fail-closed`。

**回滚**：env 缺省回告警模式（dev 不受影响）。

**验收**：dev `docker compose up` 仍正常；生产模式缺强 secret 时拒启动并明确报错。

---

## Phase E — 官key / token 静态加密（信封 + 双读 + 盲索引 + 灰度回填）

> 最高风险阶段，**每步可回滚、读永远兼容明文**。依赖 D2 的 `SECRET_ENCRYPTION_KEY`。

**Files:**
- Create: `common/encryption.go`（AES-256-GCM + 版本前缀 + 双读）
- Modify: `model/channel.go`（Key 读写路径）、`model/token.go`（Key + 新 key_lookup、GetTokenByKey:275、Insert）、`model/token_cache.go:53`、`model/channel_cache.go`
- Create: `model/migrate_encrypt_keys.go`（幂等批量回填）
- Test: `common/encryption_test.go`、`model/*_encrypt_test.go`

- [ ] **E1 加密原语**：`common/encryption.go`：
  ```go
  // EncryptSecret: 明文 -> "enc:v1:" + base64(nonce|ciphertext)，AES-256-GCM，key=SECRET_ENCRYPTION_KEY 派生(32B)
  // DecryptSecret: 有 "enc:v1:" 前缀 -> 解密; 无前缀 -> 原样返回(遗留明文，双读)
  ```
  全面单测：加解密往返、错误密钥失败、空串、明文直通、前缀识别。
- [ ] **E2 全量读写点盘点（关键，决定零差错）**：列出 `channel.Key` 与 `token.Key` 的**每一处读与写**（含 `Updates(map{...})`、原始 SQL、多 key 更新 `handlerMultiKeyUpdate`、cache 写入）。重点：**map/raw 写法会绕过 GORM 钩子与自定义类型**——必须逐一改为经加密封装写入。盘点产出一张「读/写点清单 + 改造方式」表，评审通过再动手。
- [ ] **E3 channel.Key 透明加密（先双读、再双写）**：
  - 读：所有取 `channel.Key` 的路径经 `DecryptSecret`（`GetNextEnabledKey` `model/channel.go:211`、`:188-209` 多key split、`:567-572`）。因双读，明文行不受影响。
  - 写：`SecretEncryptionWriteEnable=true` 后，新增/更新渠道 key 经 `EncryptSecret`；**map/raw 写法**（多 key 更新等）显式加密。
  - 缓存：`channel_cache` 存解密后明文（内存明文风险记 P3：缓存存密文用时瞬解，本阶段不做）。
- [ ] **E4 token.Key 加密 + 盲索引**：AES-GCM 随机 nonce ⇒ 同明文密文不同 ⇒ **不能** `WHERE key=?`。方案：
  - `tokens` 新增 `key_lookup`（`HMAC_SHA256(CryptoSecret, 明文key)`，确定性，唯一索引）。
  - Insert：`key=EncryptSecret(明文)`、`key_lookup=HMAC(明文)`。
  - `GetTokenByKey`（:275）改 `WHERE key_lookup = HMAC(incoming)`；Redis 缓存键（`cacheGetTokenByKey` token_cache.go:53）改用 `HMAC(key)` 作键。
  - 注意 `Token.Update`(:297) 的 Select 不含 key，无需动；显示用 `GetMaskedKey` 经解密。
- [ ] **E5 幂等灰度回填**：`model/migrate_encrypt_keys.go`：分批（每批 N 行）扫描**无 `enc:` 前缀**的 channel/token，加密回写 + 回填 `key_lookup`；可重复执行、单批短事务、可中断续跑；提供 `--dry-run` 统计。**先开 E3/E4 双写跑一段，再回填存量**，确保新写已是密文。
- [ ] **E6 热路径性能验证**：relay 每请求经 `GetNextEnabledKey` 解密一次——压测确认解密开销可忽略（GCM 很快）；必要时按渠道缓存解密结果（注意内存明文权衡）。
- [ ] **E7 测试（零差错核心）**：
  - 「经**每一条**写入路径（含 map/多key/raw）写 key → 读回 → 断言为原始明文」；
  - 旧明文行双读正常（未回填也能用）；
  - token 按 key 鉴权仍通过（盲索引）；
  - 回填幂等（跑两次结果一致、不重复加密）；
  - 关 `SecretEncryptionEnable` 回滚后系统仍能读（双读保证）。
- [ ] **E8 提交**：`feat(security): 官key/token 静态加密（信封+双读+盲索引+幂等回填）`。

**回滚**：`SecretEncryptionWriteEnable=false` 停止新写密文（读双兼容）；`SecretEncryptionEnable=false` 整体退回明文路径（已回填的密文仍可解密读出，因为解密始终开）。**收尾发布**（另行）才考虑去明文兜底与旧列清理。

**验收**：每条写路径读回明文一致；鉴权不破；旧行兼容；回填幂等；热路径性能达标。

---

## 验收与回滚总表

| 阶段 | 权威切换开关 | 回滚动作 | 必过测试 |
|---|---|---|---|
| A | — | — | 特征/跨库/并发基线绿 |
| B | `BillingSnapshotEnable` | 关开关回旧算法 | 套现回归、跨库 SUM、渠道删除兜底 |
| C | `SettlementAtomicEnable` | 原子化为纯增强；账本旁路可关 | 并发 confirm 不重复、无重叠打包、账本可校验 |
| D | `REQUIRE_STRONG_SECRETS` | 回告警模式 | dev 正常、prod 缺 secret 拒启 |
| E | `SecretEncryptionWriteEnable` / `SecretEncryptionEnable` | 停写密文 / 退明文路径（解密常开） | 全写路径读回明文、鉴权通过、双读兼容、回填幂等 |

---

## 执行方式

- 每个任务：**spec→实现→测试(go test+三库+并发)→对抗评审→提交（功能分支）**；碰钱/碰密钥的任务强制对抗评审（专门找「这步会不会算错钱/泄密/破鉴权」）。
- 顺序：A 先行；随后 B→C 与 D→E 两条线并进；最高优先 B（套现）与 D（DB/备份现实泄露）。
- **绝不** push/发布/部署，等用户明确指令。
- 真歧义（如旧日志快照回填口径 B4、退款冲正 C5、跨库事务 C2b）记录待用户确认，不自行拍板影响钱的口径。
