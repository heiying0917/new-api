# 事故复盘 — 2026-09-17 v2026.09.17.1 渠道创建 SQLSTATE 22P02（已回滚）

## 时间线（UTC+8）
| 时间 | 事件 |
|------|------|
| 13:17:51 | v2026.09.17.1 滚动部署完成（master 1/1 + slave 2/2），健康 200，P4 冒烟 0 panic，P6 六轮真实用户 5xx=0 |
| 13:35:37 – 13:36:57 | 用户（供应商角色）新建 Anthropic Claude 渠道连续失败，前端提示 `ERROR: invalid input syntax for type json (SQLSTATE 22P02)`；CLS 记录 8 条 error 级 SQL 日志，全部 `INSERT INTO "channels"` |
| 13:38 | 用户反馈截图 |
| 13:39 – 13:40:26 | 按红线先回滚：slave → master `kubectl rollout undo`，两 deployment 回到 v2026.06.24.1，健康 200 |
| 13:40 – 13:55 | 事故分析：CLS 拉 22P02 明细、代码审计 json 列 Valuer、定位上游配套修复 `6eb6f35ed` |
| 13:55 – 14:15 | 修复：cherry-pick `6eb6f35ed`（回归测试先 FAIL 后 PASS）；真 PG 端到端复现（修复前 22P02 / 修复后 建改读全过）；新增 PG 写路径冒烟脚本并写入 SOP |

**生产受影响时长**：13:17:51 → 13:40:26，约 22.5 分钟；受影响功能仅渠道新建/编辑（管理员 + 供应商），relay 转发、计费、登录不受影响，无脏数据。

## 根因
- 本次同步 cherry-pick 了上游 `66031a09d`（PostgreSQL 关闭 GORM `PrepareStmt`，兼容 PgBouncer 等事务池代理）。关闭后 pgx 走 **simple protocol**，所有 `[]byte` 类型参数按 bytea 十六进制字面量 `'\x…'` 发送。
- `model/channel.go` 的 `ChannelInfo.Value()`（写 `channels.channel_info`，`gorm:"type:json"`）返回 `common.Marshal` 的 `[]byte` → PG 对 json 列拒收 → 22P02。同类还有 `tasks.properties / private_data`、`prefill_groups.items` 三个 Valuer。
- 上游在同一天（2026-08-30）用 `6eb6f35ed` 修了这个问题（四个 Valuer 改返回 `string`，Scan 兼容 `[]byte`/`string`，附回归测试），我挑提交时**漏拿了配套修复**。

## 误判记录（SOP / 测试 / Skill 哪里失守）
1. **单测全部跑 SQLite**：SQLite 对 `[]byte` 写 json 列不报错，`go test ./...` 全绿不代表 PG 路径正确。
2. **本地 PG 真库验证只做了"启动 + /api/status + 无 token 的 relay 请求"**，没有任何写入 json 列的操作——启动迁移成功给了"PG 没问题"的错觉。
3. **生产 P4 冒烟**：冒烟 token 的分组无渠道且无管理员会话，所有请求在 distributor 层返回，覆盖不到渠道写入；SOP 只要求 relay 基线。
4. **cherry-pick 依赖识别**只看了"编译期依赖"（缺符号会报错），没有查"同日后续修复"（`git log --since --until -- <touched files>`），行为级依赖被漏掉。

## 改进项（已落地）
| 项 | 落地 |
|---|---|
| 修复 | main `8d0fb08f0` cherry-pick 上游 `6eb6f35ed`；`model/json_column_test.go` 回归测试：Valuer 必须返回 string / nil，Scanner 同时接受 []byte 与 string（修复前 FAIL、修复后 PASS 已反向验证） |
| 真 PG 端到端 | `.agents/skills/tke-release/scripts/pg-write-smoke.sh`：一次性 postgres:15 容器 → 跑待发版二进制 → setup/login → 建渠道(INSERT json) → 改渠道(UPDATE json) → 读回(Scan) → 断言 22P02/panic=0。修复前二进制稳定复现 22P02，修复后 PASS |
| SOP | `tke-release` Phase 1 新增 **Step 1.1b PG 写路径冒烟（必做）**，allowed-tools 加 `docker`/`bash`；Phase 2 关键 SOP 补记本事故 |
| 挑提交纪律 | 以后 cherry-pick 某提交前，必须 `git log <c>..upstream/main -- <该提交触及的文件>` 看后续是否有"fix"跟进，尤其是同日提交 |

## 重发
修复后按完整 6 Phase 重发 v2026.09.17.2（见 `docs/deploy/report/2026-09-17-tokenki-prod-v2026.09.17.2.md`）。
