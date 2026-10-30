# 渠道并发上限（max_concurrency）设计

日期：2026-10-30　状态：已与用户确认

## 需求

- 创建 / 编辑渠道时可设置「并发上限」：该渠道同一时刻最多在途处理的请求数；默认 0 = 不限。
- 管理员与供应商均可设置；前端只改 `web/classic`。
- 渠道列表展示「当前并发 / 上限」，上限为 0 显示「当前 / 不限」。
- 渠道满载时溢出到同分组/模型的其他渠道（同优先级优先，再降级；auto 分组再跨组）；全满返回 429。

## 数据模型

- `channels.max_concurrency`（`*int`，`gorm:"default:0"`），AutoMigrate 加列，三库通用。用指针是因为
  `Channel.Update()` 走 `Updates(struct)`，非指针 0 会被跳过，无法从 N 改回不限。
- 取值 0..`model.MaxChannelConcurrencyLimit`(10000)，管理员 add/update 与供应商 add/update 均校验。
- 瞬态字段 `current_concurrency`（`gorm:"-"`），仅列表/搜索回填。

## 计数器（`service/channel_concurrency.go`）

- Redis 启用（生产 3 Pod 共享）：每渠道一个 ZSET `channel_concurrency:{id}`，member=单次占位 ID，score=租约到期毫秒。
  - 占位 Lua 原子：清过期 → 达上限返回 0 → ZADD + PEXPIRE。
  - 释放 `ZREM`；后台每 15s pipeline `ZADD XX` 续期本进程持有的占位，租约 60s（Pod 被 kill -9 时泄漏 ≤ 60s，长流不被误判过期；XX 防止释放与续期竞态复活幽灵占位）。
  - 优雅停机：`srv.Shutdown` 结束后 `ReleaseAllHeldChannelSlots` 一次 pipeline 归还本进程全部占位（停机超时被切断的长流 / WebSocket 不必等租约）。
  - 占位脚本回包超时：异步尽力 `ZREM` 该 member，避免放行的同时留下占名额的幽灵占位。
  - 读数 `ZCOUNT key now +inf`，一次 pipeline 取当前页。
- Redis 未启用：进程内计数。
- Redis 出错 fail-open（放行 + 告警日志），不拖垮转发。
- 不限并发的渠道也计数（列表要显示当前值），每个转发请求 +2 次 Redis 往返。

## 选路与生命周期

- `model.GetRandomSatisfiedChannelExcluding(group, model, retry, excluded)`：内存缓存与 DB 两条路径都支持排除；
  排除后再计算优先级档位，所以同档全满会自然降档。
- `service.CacheGetRandomSatisfiedChannel`：选中即占位，失败则排除重选；排除集仅本次选路有效。
  首次撞满时一次 pipeline 读出全部候选渠道在途数，把已满渠道整体排除（`excludeSaturatedChannels`），
  避免组内大量渠道满载时每个请求逐个试占、打出 N 次 Redis 往返。
  无渠道且有渠道因满载被跳过 → `ErrChannelConcurrencyLimited`。
- 档位语义：排除后按剩余档位重新套用 retry 档位规则（retry 超出档位数钳到最末档），
  因此重试时低档全满可能回到更高档（含刚失败的渠道）——与原有「重试不排除失败渠道」一致。
- 占位存于 gin context，`TryAcquireChannelSlot` 先释放已持有的占位（重试换渠道时不会自己占自己的位）。
- Distribute：入口 `defer ReleaseChannelSlot`，覆盖流式 / WebSocket 全程与 panic；
  令牌指定渠道满 → 429（仅在需要选路转发时占位，任务查询不占位）；
  亲和首选渠道满 → 走正常选路，不因满载清除亲和记录（改走渠道成功后按原逻辑更新亲和记录）。
- Relay 重试：重新选路时换占位；重试时全满则返回上一次上游真实错误。
- Task：remix 锁定原渠道时把占位切到锁定渠道，满则 429；重试全满保留上一次错误。
- 满载不是渠道错误（错误码 `channel_concurrency_limited`，不带 `channel:` 前缀），不触发自动禁用。
- 后台「测试渠道」不经 Distribute，不计数。多 Key 渠道按整渠道计数。

## 前端（classic）

- 编辑弹窗：优先级 / 权重同区新增「并发上限」InputNumber（min 0, max 10000，提示 0 表示不限）。
- 列表新增「并发」列（管理员/供应商可见，观察员隐藏）：`当前 / 上限|不限`；
  标签聚合行：当前求和；上限只算已启用子渠道，其中任一不限则「不限」，否则求和。

## 测试

- 内存计数器：占位、上限、释放、重复释放、不限。
- 选路排除：同档换渠道、整档满降级、全满 nil（内存缓存与 DB 路径）。
- 校验：负数 / 超上限拒绝。
- 满载预排除、auto 分组跨组溢出、Distribute 429 与请求结束后归还名额（`middleware/distributor_concurrency_test.go`）。
- Redis Lua / 续期 / 读数 / 停机归还 / 争抢原子性：真 Redis 集成测试（`CHANNEL_CONCURRENCY_TEST_REDIS=redis://host:port/db`，未设置则跳过）。
