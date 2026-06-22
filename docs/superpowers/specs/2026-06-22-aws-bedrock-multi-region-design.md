# AWS Bedrock 多区域并发 + Global 跨区域推理 设计文档

- 日期: 2026-06-22
- 项目: newapi-juhe (`/Users/xuyang/workspace/newapi-juhe`)
- 渠道类型: AWS (`ChannelTypeAws = 33`)
- 参考实现: sub2api 的"添加账号 / AWS Bedrock"(Region 下拉 + 强制 Global 跨区域推理)

## 1. 背景与目标

### 现状
newapi-juhe 创建 AWS 渠道时,Region 必须手写进密钥串:
- AK/SK 模式: `AccessKey|SecretAccessKey|Region`
- API Key 模式: `APIKey|Region`

后端 `relay/channel/aws/relay-aws.go` 按 `|` 分段数判断模式,用 Region 建 Bedrock 客户端,并按 region 前缀(`us`/`eu`/`ap`)+ `awsModelCanCrossRegionMap` 白名单自动给模型 ID 加 `us.`/`eu.`/`apac.` 跨区前缀。**完全没有 `global.` 概念。**

### 痛点 / 目标
用户的真实诉求:**用同一对 AK/SK 同时在多个区域并发跑,叠加各区域配额,把额度用满用快。**
机制上 newapi-juhe 已经具备:
- **批量创建**:密钥框一行一个,批量建渠道。
- **密钥聚合模式 (`multi_to_single`)**:多行密钥聚合进一个渠道,轮询使用。
- 后端已能把每行 `AK|SK|region` 建成对应区域的客户端并自动跨区。

唯一缺口:**Region 要逐行手写,区域一多没法手输。**

因此本设计新增两块能力:
- **Feature A — 区域多选 + 并发测试 + 一键写回**:填一对凭证 → 多选区域 → 并发测试可用性 → 可用区域默认勾选 → 按钮把 `AK|SK|region` 多行写回密钥框 → 照常走批量+聚合。
- **Feature B — 强制使用 Global 跨区域推理**:可勾选项,勾选后模型 ID 用 `global.` 前缀。

### 不在本次范围(YAGNI)
- 不引入 sub2api 的 `jp`/`au`/`us-gov` 区域前缀细分;region→前缀映射保持现有 `us`/`eu`/`apac` 不变。
- 不改现有 `AK|SK|Region` 密钥格式与后端分段解析逻辑;完全向后兼容。
- 不支持 AWS 临时凭证 session_token(现有 newapi 也不支持,保持一致)。

## 2. 决策记录(已与用户确认)

| 决策 | 结论 |
|---|---|
| 区域来源 | 多选下拉框 + 测试按钮,**不**无脑补全全部 |
| 测试探测方式 | **真实调用模型**(极小 InvokeModel,200/429 视为可用) |
| 测试用模型 | 渠道已选模型中**最便宜的一个**;未选则回退内置小模型 |
| 写回密钥框 | **按钮**显式写回(可见、可再编辑、复用批量/去重/提交流程) |
| Global 行为 | 勾选后**无条件**加 `global.` 前缀,绕过 per-model 白名单 |
| region→前缀映射 | **保持不变**(us/eu/apac) |

## 3. 总体架构

```
前端 EditChannelModal (type === 33)
 ├─ 密钥框: 填一对 AK|SK (或 APIKey)
 ├─ [新] 区域多选下拉 (预填全量区域常量)
 ├─ [新] "测试可用区域" 按钮 ──POST──> 后端 /api/channel/aws/test_regions
 │        └─ 回填: 可用默认选中 / 不可用默认不选, 显示延迟与错误
 ├─ [新] "生成密钥(写回密钥框)" 按钮 → 写入 AK|SK|region 多行
 ├─ [新] "强制使用 Global 跨区域推理" 勾选框 → settings.aws_force_global
 └─ 照常: 批量创建 + 密钥聚合模式 + 密钥去重 + 提交

后端
 ├─ [新] controller.TestAwsRegions: 并发探测每个区域可用性
 ├─ [改] relay/channel/aws/relay-aws.go: AwsForceGlobal=true 时用 global. 前缀
 └─ [改] dto/channel_settings.go: ChannelOtherSettings 新增 AwsForceGlobal
```

## 4. Feature A — 区域多选 + 并发测试 + 写回

### 4.1 前端:区域常量(单点维护)
文件:`web/classic/src/constants/channel.constants.js`

新增导出常量,默认覆盖 newapi 跨区前缀能识别的 us/eu/ap 三大地理下的 Bedrock 区域(~19 个):

```js
export const AWS_BEDROCK_REGIONS = [
  // US
  { region: 'us-east-1',      label: 'us-east-1 (N. Virginia)',   geo: 'us' },
  { region: 'us-east-2',      label: 'us-east-2 (Ohio)',          geo: 'us' },
  { region: 'us-west-1',      label: 'us-west-1 (N. California)',  geo: 'us' },
  { region: 'us-west-2',      label: 'us-west-2 (Oregon)',        geo: 'us' },
  // EU
  { region: 'eu-west-1',      label: 'eu-west-1 (Ireland)',       geo: 'eu' },
  { region: 'eu-west-2',      label: 'eu-west-2 (London)',        geo: 'eu' },
  { region: 'eu-west-3',      label: 'eu-west-3 (Paris)',         geo: 'eu' },
  { region: 'eu-central-1',   label: 'eu-central-1 (Frankfurt)',  geo: 'eu' },
  { region: 'eu-central-2',   label: 'eu-central-2 (Zurich)',     geo: 'eu' },
  { region: 'eu-north-1',     label: 'eu-north-1 (Stockholm)',    geo: 'eu' },
  { region: 'eu-south-1',     label: 'eu-south-1 (Milan)',        geo: 'eu' },
  { region: 'eu-south-2',     label: 'eu-south-2 (Spain)',        geo: 'eu' },
  // APAC
  { region: 'ap-northeast-1', label: 'ap-northeast-1 (Tokyo)',    geo: 'ap' },
  { region: 'ap-northeast-2', label: 'ap-northeast-2 (Seoul)',    geo: 'ap' },
  { region: 'ap-northeast-3', label: 'ap-northeast-3 (Osaka)',    geo: 'ap' },
  { region: 'ap-south-1',     label: 'ap-south-1 (Mumbai)',       geo: 'ap' },
  { region: 'ap-south-2',     label: 'ap-south-2 (Hyderabad)',    geo: 'ap' },
  { region: 'ap-southeast-1', label: 'ap-southeast-1 (Singapore)',geo: 'ap' },
  { region: 'ap-southeast-2', label: 'ap-southeast-2 (Sydney)',   geo: 'ap' },
];
```

说明:列表偏"广覆盖",个别区域可能没有 Claude;测试不可用的默认不选中,用户也可手动剔除。后续 AWS 增减区域时改这一个常量即可。

### 4.2 前端:UI(仅 `inputs.type === 33` 显示)
文件:`web/classic/src/components/table/channels/modals/EditChannelModal.jsx`,放在 AWS 的"密钥格式"选择器附近 / 密钥框下方。

组件:
1. **区域多选下拉**:Semi UI `Form.Select multiple filter`,`optionList` 来自 `AWS_BEDROCK_REGIONS`。本地 state `awsRegions`(已选区域数组)、`awsRegionTestResult`(`{ [region]: { ok, latencyMs, message } }`)。测试后每个 option 的 label 追加状态后缀(`✓ 320ms` / `✗ AccessDenied`)。
2. **"测试可用区域" 按钮**(loading 态):
   - 从密钥框第一行非空行解析出凭证主体:`aws_key_type==='api_key'` 取第 1 段 `APIKey`;否则取前 2 段 `AK|SK`(忽略行内已写的 region)。
   - 校验:无凭证则 `showInfo('请先在密钥框输入一对 AK|SK')`。
   - 取测试模型(见 4.3),POST 到测试接口,带全量 `AWS_BEDROCK_REGIONS` 的 region 列表。
   - 返回后:`awsRegionTestResult` 落库渲染;`awsRegions` 设为所有 `ok===true` 的区域(默认选中可用项);用户可手动增减。
3. **"生成密钥(写回密钥框)" 按钮**:
   - 以当前凭证主体 + `awsRegions` 勾选项,生成多行 `AK|SK|region`(api_key 模式为 `APIKey|region`)。
   - 去重后 `formApiRef.current.setValue('key', text)` + `handleInputChange('key', text)`(与现有 `deduplicateKeys` 同一套写法)。
   - 若 `awsRegions` 为空则提示先选区域。
   - 友好提示:写回后建议勾选"批量创建 + 密钥聚合模式"。

> 交互说明:测试与写回解耦为两个按钮,保证"测试 → 人工微调勾选 → 写回"的可见、可控流程;写回结果进入密钥框后完全复用现有 批量创建 / 密钥聚合 / 密钥去重 / 提交 逻辑,提交路径零改动。

### 4.3 测试用模型选择规则(前端)
从 `inputs.models`(已选模型)里挑一个用于测试,规则:
1. 过滤出 AWS 支持的模型(出现在后端 `awsModelIDMap` 的 key 中;前端维持一份等价的"支持集"或直接传给后端校验)。
2. 在支持集中按"便宜优先"排序挑第一个:含 `haiku` > 含 `sonnet` > 含 `opus` > 其他;同级按字符串排序保证确定性。
3. 若没有任何 AWS 支持模型被选,回退内置默认 `claude-3-5-haiku-20241022`。
4. 把选中的模型名(OpenAI 侧名,如 `claude-3-5-haiku-20241022`)作为 `model` 传给测试接口;后端用 `getAwsModelID` 解析为 Bedrock ID。

### 4.4 后端:并发测试接口
- 路由(`router/api-router.go`,`channelRoute` 组内,已带 `middleware.AdminAuth()`):
  ```go
  channelRoute.POST("/aws/test_regions", controller.TestAwsRegions)
  ```
- 控制器:新建 `controller/channel-aws.go`,函数 `TestAwsRegions(c *gin.Context)`。

**请求体**:
```json
{
  "aws_key_type": "ak_sk",        // 或 "api_key"
  "access_key": "AKIA...",        // ak_sk 模式
  "secret_key": "....",           // ak_sk 模式
  "api_key": "....",              // api_key 模式
  "regions": ["us-east-1", "eu-west-1", "..."],
  "model": "claude-3-5-haiku-20241022"  // 可空,空则用内置默认
}
```

**响应体**:
```json
{
  "success": true,
  "data": [
    { "region": "us-east-1", "ok": true,  "status_code": 200, "latency_ms": 321, "message": "" },
    { "region": "eu-west-3", "ok": false, "status_code": 400, "latency_ms": 180, "message": "model not supported in region" }
  ]
}
```

**探测逻辑**(每个区域一次,goroutine 并发,带 bounded 并发与单区域超时):
1. 复用/抽取 AWS 客户端构造:在 `relay/channel/aws` 包内导出 `BuildBedrockRuntimeClient(keyType, ak, sk, apiKey, region string, httpClient *http.Client) (*bedrockruntime.Client, error)`,供 relay 与本接口共用(消除重复)。
2. 解析模型:`base := getAwsModelID(model)`;按现有逻辑 `prefix := getAwsRegionPrefix(region)`;若 `awsModelCanCrossRegion(base, prefix)` 则 `modelId := awsModelCrossRegion(base, prefix)`,否则 `modelId := base`。(测试路径与真实流量一致。)
3. 发一个极小 InvokeModel(非流式),body 为最小 Anthropic 负载:
   ```json
   {"anthropic_version":"bedrock-2023-05-31","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}
   ```
4. 结果分类:
   - HTTP 200 → `ok=true`
   - 429 Throttling → `ok=true`(凭证有效,仅限流)
   - 400 ValidationException(模型不支持该区域)→ `ok=false`
   - 403 AccessDenied(模型未授予访问)/ 401 / 签名错误 → `ok=false`
   - 超时 / 网络错误 → `ok=false`,`message` 记原因
   - `status_code` 用 `getAwsErrorStatusCode(err)` 提取;`message` 取精简错误串。
5. 并发控制:`errgroup` 或带缓冲 channel 的 worker pool,并发上限例如 8;单区域 `context.WithTimeout`(例如 10s);整体可设总超时。
6. 安全:仅 `AdminAuth`;**绝不记录** access_key/secret_key/api_key 到日志;响应只回 region 维度状态。

### 4.5 与批量/聚合的衔接(无需改动)
写回后密钥框形如:
```
AKIA...|secret...|us-east-1
AKIA...|secret...|us-east-2
AKIA...|secret...|eu-west-1
...
```
用户勾选 **批量创建 + 密钥聚合模式** 提交,走现有 `mode='multi_to_single'` 路径(`EditChannelModal.jsx` L1899-1923),后端为每行建对应区域客户端并自动跨区,聚合渠道轮询使用 → 多区域并发。

## 5. Feature B — 强制使用 Global 跨区域推理

### 5.1 DTO
文件:`dto/channel_settings.go`,`ChannelOtherSettings` 新增:
```go
AwsForceGlobal bool `json:"aws_force_global,omitempty"` // 强制使用 global. 跨区域推理
```
存储沿用现有 `settings` JSON 列机制(与 `AwsKeyType` 完全一致),适配器侧通过 `info.ChannelOtherSettings.AwsForceGlobal` 读取(`relay/common/relay_info.go` 已注入)。

### 5.2 前端
文件:`EditChannelModal.jsx`,AWS 渠道表单新增勾选框"强制使用 Global 跨区域推理":
- 绑定 `inputs.aws_force_global`,onChange 走 `handleChannelOtherSettingsChange('aws_force_global', value)`(与 `aws_key_type` 同一机制)。
- 保存:在组装 `settings` 处加入 `settings.aws_force_global = !!localInputs.aws_force_global;`(对照现有 L1800 一带 `aws_key_type` 的写法)。
- 回填:解析 `data.settings` 时 `data.aws_force_global = parsedSettings.aws_force_global || false;`(对照现有 L901-963)。
- 文案提示:"启用后模型 ID 使用 global. 前缀,请求可路由到全球任意支持区域。需该模型已开通 Global 跨区域推理。"

### 5.3 后端
文件:`relay/channel/aws/relay-aws.go`,`doAwsClientRequest`(现 L98-105):
```go
awsModelId := getAwsModelID(info.UpstreamModelName)

if info.ChannelOtherSettings.AwsForceGlobal {
    awsModelId = "global." + awsModelId          // 无条件 global. 前缀,绕过白名单
} else {
    awsRegionPrefix := getAwsRegionPrefix(awsCli.Options().Region)
    if awsModelCanCrossRegion(awsModelId, awsRegionPrefix) {
        awsModelId = awsModelCrossRegion(awsModelId, awsRegionPrefix)
    }
}
```
说明:base 模型 ID 形如 `anthropic.claude-opus-4-6-v1` → `global.anthropic.claude-opus-4-6-v1`,与 sub2api 行为一致。不支持 global 的模型由 AWS 返回错误(用户自负)。

### 5.4 Feature A 与 B 的关系
两者独立可组合:
- 仅 A:多区域 key,每区域按 `us./eu./apac.` 跨区。
- 仅 B:单/少区域 key,所有模型走 `global.` 全球路由。
- A+B:多区域 key 且都用 `global.` 前缀 → 多个"源区域"各自的 global 推理配额叠加(冗余但无害)。
测试按钮(Feature A)固定按区域前缀探测,不受 Global 勾选影响。

## 6. 兼容性

- 现有 AWS 渠道(密钥含 region、未设 `aws_force_global`)行为完全不变。
- 不改密钥分段解析、批量/聚合/去重/提交主流程。
- 新增 DTO 字段带 `omitempty`,旧 settings JSON 反序列化为零值(false),无迁移。

## 7. 涉及文件清单

前端:
- `web/classic/src/constants/channel.constants.js` — 新增 `AWS_BEDROCK_REGIONS` 常量。
- `web/classic/src/components/table/channels/modals/EditChannelModal.jsx` — 区域多选 + 测试按钮 + 写回按钮 + Global 勾选框 + 占位符/提示更新;新增对测试接口的 API 调用与 i18n 文案。
- 可能涉及 i18n 资源(若项目集中管理 zh/en 文案)。

后端:
- `dto/channel_settings.go` — `ChannelOtherSettings` 新增 `AwsForceGlobal`。
- `relay/channel/aws/relay-aws.go` — Global 前缀分支;抽取 `BuildBedrockRuntimeClient` 供复用。
- `relay/channel/aws/constants.go` — 如需,辅助暴露 helper(`getAwsModelID`/`getAwsRegionPrefix`/`awsModelCanCrossRegion`/`awsModelCrossRegion` 已在包内,可直接被 controller 通过包内函数或新导出函数调用)。
- `controller/channel-aws.go` — 新增 `TestAwsRegions`。
- `router/api-router.go` — 新增路由 `POST /channel/aws/test_regions`。

## 8. 测试计划

后端单测(Go):
- `BuildBedrockRuntimeClient`:ak_sk / api_key 两种模式构造客户端不报错。
- Global 前缀:`AwsForceGlobal=true` 时 `claude-opus-4-6` → `global.anthropic.claude-opus-4-6-v1`;`false` 时维持 `us./eu./apac.` 现状(table 驱动)。
- `TestAwsRegions`:结果分类映射(200/429→ok;400/403→not ok)的纯函数部分可单测(把"HTTP 状态/错误 → ok 判定"抽成可测函数)。

手动验证(`/run` 或本地起服务):
- 填一对真实/无效 AK|SK,点测试 → 观察可用区域勾选与错误展示。
- 写回密钥框 → 批量 + 聚合创建 → 聚合渠道含多区域 key。
- 勾选 Global,实际请求确认上游模型 ID 带 `global.` 前缀(日志/抓包)。

## 9. 边界与风险

- 测试会对每个区域产生一次极小 InvokeModel 调用(约 1 token 成本);区域多时为并发 N 次,需 bounded 并发避免打满。
- "可用区域"以测试模型为准;不同大小模型在区域的可用性可能不同(已通过"用最便宜的已选模型"贴近真实)。
- 安全:测试接口接收明文凭证(仅管理员、管理员会话内),务必不落日志、不持久化。
- 广覆盖区域列表可能含无 Claude 区域,测试不可用→默认不选,可手动剔除。

## 10. 未来可选(本次不做)
- 引入 `jp`/`au`/`us-gov` 区域前缀细分(对齐 sub2api,但会改变现有 ap-* 行为)。
- 测试支持 session_token / 临时凭证。
- 测试结果缓存、按 geo 分组一键全选/反选。
