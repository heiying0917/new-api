# AWS 区域测试模型可在弹窗中选择 — 设计

> 2026-11-11 · 状态：已确认 · 范围：仅 web/classic

## 背景

「测试可用区域」从 v2026.11.11.1 起固定用 `claude-opus-4-6` 探测（前端常量 `AWS_REGION_TEST_MODEL`，后端未传 `model` 时默认值也是它）。有些账号只开通了部分 Claude 模型，管理员需要在测试时临时换模型。后端 `POST /api/channel/aws/test_regions`（及供应商路由）本来就接受 `model` 参数，所以只改前端。

## 交互

在区域多选框下方加一个整行宽的下拉框（前缀「测试模型」），替换原来那行静态的「测试模型：claude-opus-4-6」提示；按钮行保持原样：

```
[区域多选 ▾]
[测试模型 | claude-opus-4-6                ▾ ⓧ]
[测试可用区域] [批量生成可用区域密钥]
```

（最初设计是下拉框与按钮同一行；侧边栏宽度不够，实测「批量生成可用区域密钥」会被挤到第二行，改为整行。）

- 可搜索，也可手动输入任意模型名（Semi Select `filter` + `allowCreate`，可清空）。
- 选择保存在浏览器 localStorage（键 `aws_region_test_model`），下次打开弹窗沿用；读写都包 try/catch，读不到、为空或被清空时回退到 `AWS_REGION_TEST_MODEL`。
- 一次测多组凭证时，所有凭证都用同一个模型。
- 测试时记下本次用的模型，结果区第一行显示「测试模型：X」（复用已有词条 `测试模型：{{model}}`）；测试后再改下拉框，不会让旧结果看起来像新模型测出来的。关闭弹窗时与其他测试结果一起清空。

## 候选列表

纯函数 `buildAwsTestModelOptions(defaultModel, channelModels, adaptorModels, currentModel)`（`awsRegionCredentials.js`）：

1. `defaultModel`（`claude-opus-4-6`）永远第一；
2. `currentModel`（当前选中值）第二，不受 Claude 过滤——否则手动输入的值刷新后在下拉里看不到；
3. 当前渠道已选的模型（`inputs.models`）中名字含 `claude` 的，保持用户原顺序；
4. AWS 适配器支持的模型（`getChannelModels(33)`，来源后端 `awsModelIDMap`，`/api/models` 下发并缓存在 localStorage，供应商也能拿到）中含 `claude` 的，**按名称倒序**——后端 map 遍历顺序随机，倒序让新模型靠前、Claude 3.x 落到最后；
5. 去掉首尾空白、去重（保留首次出现）、过滤空值；渠道 / 适配器列表不是数组时忽略，避免弹窗崩溃。

只保留 Claude 是因为探测请求固定用 Anthropic 格式；手动输入不受此限制，输入非 Claude 模型时 AWS 返回的错误原因会照常显示在结果里。

候选在每次渲染时重算，**不用 useMemo**：`channel_models` 由渠道页挂载后异步加载，memo 会停在加载前只有默认值的列表（实测踩到）；`getChannelModels` 自带缓存，重算开销可忽略。

同一会话内手动输入的值由 Semi `allowCreate` 追加到其内部列表末尾；刷新后由本函数放在第 2 位。

纯函数 `resolveAwsTestModel(value, defaultModel)`：去掉首尾空白，为空则返回 `defaultModel`。

## 不做

- 系统级全站默认（options 表 / 设置页）。
- 按渠道持久化测试模型。
- 后端改动。

## 测试

- bun 单测：`buildAwsTestModelOptions`（默认第一、渠道已选优先于适配器列表、只保留 Claude、去重与空白处理、输入为空/undefined）；`resolveAwsTestModel`（空白、空串、undefined 回退默认，正常值去空白）。
- classic `bun test`、ESLint、Prettier（改动文件）、`bun run build`。
- 后端不改，跑发版要求的 `go build ./...` 与 model/service/controller/logger 测试作回归。
- 本地起后端 + classic 构建产物，用浏览器真实操作：下拉候选、手动输入、刷新后保留选择、请求体里的 `model` 正确、结果区显示本次测试模型。
