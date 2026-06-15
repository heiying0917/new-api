# v6 首页重做 + 登录注册 Tab 焦点优化 — 设计方案 (spec)

> 日期:2026-06-15 · 范围:仅 `web/classic`(线上默认主题)· 语言:中文为主,接 i18next
> 关联:`docs/superpowers/WORKLOG.md`;实现计划见 `docs/superpowers/plans/2026-06-15-v6-home-redesign.md`(待写)。
> 约定:未经用户明确指令,不 commit / push / 发布 / 部署。

## 1. 目标

把 classic 的**默认首页**(未登录访客看到的公开页)从"通用 API 网关"落地页,重做成体现本平台定位的**供应商招募落地页**:面向供应商的官方 Key 托管平台。要求:大气、官方大平台质感、赢得供应商信任;配色与控制台一致、明暗跟随系统;并修复登录/注册 Tab 焦点错落到密码"小眼睛"的问题。

## 2. 已锁定的设计决策(用户确认)

1. **范围**:只改 classic(线上默认 `theme.frontend=classic`);default 主题暂不动。
2. **基调**:浅色现代 SaaS(精修 B),已出 mockup 确认。
3. **全局主色**:深宝蓝 **Sapphire**;浅色 `#2052CC`、深色 `#7AA2FF`。**全局生效**(首页 + 控制台一起换),通过覆盖 Semi 主色令牌实现。
4. **明暗**:跟随系统 + 手动切换——复用现有 `context/Theme`(默认 `auto`,`prefers-color-scheme`,置 `body[theme-mode=dark]`)。落地页只用 Semi 令牌即自动适配,无需自建明暗逻辑。
5. **数字后台可配**:首页统计数字由后台配置;**未配置时不编造具体数字**,回退为定性表述(避免不实宣传)。
6. **文案接 i18next**:与 classic 现有一致(`useTranslation`),默认中文,日后可加英文。
7. **Tab 修复**:与首页一起做、一起提交。
8. **品牌红线(Rule 5)**:页脚/版权中的 new-api / QuantumNous 不改不删——落地页复用全局 FooterBar,天然保留。

## 3. 现状关键事实(调研结论)

- **布局壳** `components/layout/PageLayout.jsx`:固定全局 `HeaderBar`(所有路由,含 `/`)+ 仅 `/console*` 的 `SiderBar` + 全局 `FooterBar`(`/` 上显示)。
- **首页** `pages/Home/index.jsx`:`homePageContentLoaded && homePageContent === ''` 分支渲染硬编码网关落地页(161–335 行)= **本次要替换的对象**;否则渲染后台 `HomePageContent`(markdown / `https://` iframe)= **保留不动**。
- 首页已用 `StatusContext` + `useTranslation` + `useActualTheme`。
- **状态通道**:`PageLayout.loadStatus()` → `GET /api/status` → `statusDispatch` → `StatusContext`。前端按 `statusState.status.*` 读取(如 `HeaderNavModules`、`docs_link`)。
- **配色**:全站用 Semi 令牌(`--semi-color-*`);无自定义 Semi 主题包 → 当前是 Semi 默认蓝 `#0064FA`(浅)/`#54A9FF`(深)。全局样式入口 `web/classic/src/index.css`(1051 行)。
- **管理员设置**:`components/settings/OtherSetting.jsx` 已管理 `SystemName / Logo / Footer / HomePageContent` 等个性化选项。

## 4. 架构与组件设计

### 4.1 落地页落点(不引入重复 nav/footer)

落地页**只渲染中间内容板块**,顶栏用全局 `HeaderBar`、页脚用全局 `FooterBar`(品牌红线在此,无需我方处理)。在 `Home/index.jsx` 的空内容分支,把硬编码网关落地页替换为新组件 `<SupplierLanding />`。自定义 `HomePageContent`/iframe 分支保持不变。

新组件目录(职责单一、便于维护):
```
web/classic/src/pages/Home/landing/
  SupplierLanding.jsx     // 组装各板块 + 顶部留白(避开 fixed header)
  Hero.jsx                // 主标题/副标题/双 CTA/收益示意面板/信任数据条
  Advantages.jsx          // 4 卡核心优势
  Process.jsx             // 4 步托管流程
  Security.jsx            // 安全保障(呼应 v5 真实能力)
  Channels.jsx            // 支持的官方渠道徽章
  CtaBand.jsx             // 底部行动号召
  landing.css             // 仅本页用到的渐变/光晕/动效(全部基于 Semi 令牌变量)
```
- 全部颜色用 Semi 令牌(`var(--semi-color-*)` 或 tailwind 映射 `text-semi-color-*` / `bg-semi-color-bg-1` 等),**不写死颜色**;主色 = `--semi-color-primary`(已被全局覆盖为 Sapphire),明暗自动适配。
- 高级感:仅在 hero 光晕 / 眉标 chip / 收益面板头部 / CTA / 步骤序号用**克制的 Sapphire→靛蓝渐变**点缀;其余靠字阶、间距、发丝边框(`--semi-color-border`)、悬停微动效。
- 响应式:桌面多列、移动端单列;图标全部 inline SVG(或复用 `@lobehub/icons` / `@douyinfe/semi-icons`)。

### 4.2 文案与 i18n

所有可见文案用 `t('中文源串')`,并把新增中文键补入 `web/classic/src/i18n/locales/zh.json`(及 en.json 占位,值可暂等于中文/英文翻译)。源串清单见附录 A。

### 4.3 全局主色 → Sapphire(Semi 令牌覆盖)

在 `index.css` 覆盖 Semi 主色族,分浅色/深色两套(深色用 `body[theme-mode="dark"]` 选择器):

浅色:
```
--semi-color-primary:        #2052CC;
--semi-color-primary-hover:  #1B47B3;
--semi-color-primary-active: #143A91;
--semi-color-primary-light-default: #EAF0FB;
--semi-color-link:           #2052CC;
--semi-color-link-hover:     #1B47B3;
--semi-color-link-active:    #143A91;
--semi-color-focus-border:   #2052CC;
```
深色(`body[theme-mode="dark"]`):
```
--semi-color-primary:        #7AA2FF;
--semi-color-primary-hover:  #9BBAFF;
--semi-color-primary-active: #BFD3FF;
--semi-color-primary-light-default: rgba(122,162,255,0.20);
--semi-color-link:           #7AA2FF;
--semi-color-link-hover:     #9BBAFF;
--semi-color-link-active:    #BFD3FF;
--semi-color-focus-border:   #7AA2FF;
```
说明 / 风险:
- 这是**全局变更**,会同步影响整个控制台的主色。实现计划须包含:枚举 Semi 实际用到的 primary/link 令牌全集(含 `-light-hover` / `-light-active` / `-disabled` 等)并补齐;落地后用 Playwright 在真实控制台页面(登录、Dashboard、渠道、结算、供应商页)逐一核查无突兀、对比度达标(浅色深蓝按钮上白字、深色浅蓝按钮上深字均需可读)。
- 少数 Semi 组件可能直接引用 `--semi-blue-*` 色阶;若核查发现偏差,再决定是否一并覆盖蓝色阶。**低风险、可回退**(集中在 index.css)。

### 4.4 可配数字(后台可配 + 诚实默认)

- **后端**:新增系统选项 `HomeSupplierStats`(TEXT,存 JSON 字符串),并在 `GET /api/status` 负载里原样透出(与 `HeaderNavModules` 同模式)。
- **数据结构**(数组,最多 4 项):
  ```json
  [
    {"value":"500+","label":"企业客户","caption":""},
    {"value":"","label":"持续充足","caption":"订单供给"}
  ]
  ```
- **前端**:`Hero` 解析 `statusState.status.HomeSupplierStats`。
  - 某项有 `value` → 显示"大字 value + label/caption";
  - 某项 `value` 为空,或整个选项未配置 → **回退为定性条目,不编造数字**:默认四项 = `数百家·企业客户` / `持续·充足订单` / `多币种·快速结算` / `端到端·加密隔离`。
- **管理员编辑**:在 `OtherSetting.jsx` 增加"首页供应商数据"卡片——4 行结构化表单(数值 / 标签 / 说明),保存为 `HomeSupplierStats` JSON。留空即用定性默认。
- **收益示意面板**:作为**产品示意图**保留(静态、明确标注"示意 / sample"),其金额为 UI 演示数据而非收益承诺;本期不做成可配(YAGNI),如需后续再扩展。

### 4.5 CTA 跳转

`成为供应商 → /register`(注册即走供应商注册流);`登录控制台 → /login`。沿用 `react-router-dom` 的 `<Link>`。

## 5. 登录/注册 Tab 焦点修复

目标:在登录页与注册页,从密码框按 Tab 应跳到**下一个输入框**,而不是停在密码可见性"小眼睛"上。

- **classic(线上)**:`components/auth/LoginForm.jsx`、`RegisterForm.jsx` 的密码框为 Semi `Form.Input mode="password"`,其内置眼睛图标进入了 Tab 顺序。实现计划须先用 Playwright 审出该眼睛节点的真实 DOM/`tabindex`,然后采用其一:
  - 首选:用自定义后缀按钮(`tabIndex={-1}`)+ 受控 `type` 切换替换 `mode="password"` 的内置眼睛(行为可控、可访问);
  - 或:挂载后通过 ref 把内置眼睛节点 `tabIndex` 置 `-1`。
  确认 `password`、`password2`(确认密码)两处都修。
- **default(非线上,顺手)**:`web/default/src/components/password-input.tsx` 的眼睛 `<Button>` 加 `tabIndex={-1}`。
- 验收:键盘 Tab 依次经过 用户名 → 密码 →(确认密码)→ 提交,**不**经过眼睛;鼠标点击眼睛仍可切换明文。

## 6. 非目标(本期不做)

- 不改 default 主题首页(除上面的 Tab 一行修复)。
- 不动 `HomePageContent` 自定义内容 / iframe 渲染逻辑。
- 不改全局 HeaderBar / SiderBar / FooterBar 结构(仅受第 4.3 节主色影响)。
- 收益面板不做成后台可配(静态示意)。
- 不触碰核心业务:官 key 管理、计费、结算后端逻辑一律不动(本期纯前端 + 一个只读透出的 status 字段 + 一个个性化选项)。

## 7. 文件改动地图

新增:
- `web/classic/src/pages/Home/landing/{SupplierLanding,Hero,Advantages,Process,Security,Channels,CtaBand}.jsx` + `landing.css`

修改:
- `web/classic/src/pages/Home/index.jsx` — 空内容分支改渲染 `<SupplierLanding />`(其余不动)。
- `web/classic/src/index.css` — 覆盖 Semi 主色令牌(浅/深 Sapphire)。
- `web/classic/src/i18n/locales/zh.json`、`en.json` — 新增首页文案键。
- `web/classic/src/components/settings/OtherSetting.jsx` — 新增"首页供应商数据"卡片(`HomeSupplierStats`)。
- `web/classic/src/components/auth/LoginForm.jsx`、`RegisterForm.jsx` — 密码眼睛移出 Tab 顺序。
- `web/default/src/components/password-input.tsx` — 眼睛 `<Button>` 加 `tabIndex={-1}`。
- 后端:`/api/status` 负载补 `HomeSupplierStats`(读 OptionMap),并在选项默认值表登记该 key(参照现有选项注册方式)。

## 8. 验证计划

- 构建:`web/classic` 通过 `bun run build`(或 `bunx rsbuild/vite build`)无报错;`go build ./...` 通过(若动到后端 status)。
- 视觉:本地预览,浅色 + 深色、桌面 + 移动,逐板块核对;Playwright 截图。
- 主色全局核查:登录/Dashboard/渠道/结算/供应商页在浅深两态对比度与观感正常。
- 可配数字:配置 `HomeSupplierStats` 后正确显示;清空后回退定性默认、无编造数字。
- Tab:键盘走查登录/注册,不落在眼睛上;鼠标点击仍可切换明文。
- 回归:`go test ./...` 不新增失败(若改后端)。
- 品牌红线:确认 FooterBar 的 new-api/QuantumNous 仍在、未被改动。

## 附录 A · 首页中文文案(i18n 源串)

- 眉标:`面向供应商 · 官方额度变现`
- H1:`托管你的官方 Key,接入全球订单`
- 副标题:`我们为全球数百家企业稳定供应 AI 大模型 token,订单量充足。把你闲置的 Claude / AWS / OpenAI 官方额度托管给我们——安全隔离、实时计量、多币种快速结算。`
- CTA:`成为供应商` / `登录控制台`
- 信任条(定性默认):`数百家 企业客户` / `持续 充足订单` / `多币种 快速结算` / `端到端 加密隔离`
- 核心优势(标题 `为什么把官方 Key 托管给我们`):
  1. `订单量充足,Key 不闲置` — `平台聚合全球数百家企业的真实需求,你的官方额度持续接单,稳定产生收益。`
  2. `多种官方 Key 托管` — `支持 Claude(Anthropic)、AWS Bedrock、OpenAI 等主流官方渠道,一处托管、统一接单。`
  3. `关键数据隔离加密` — `官方 Key 加密存储,供应商之间严格隔离、互不可见;遵循最小可见原则。`
  4. `多币种快速结算` — `按实际消耗实时计量,成交价逐笔冻结(不受事后改价影响),支持多币种、快速回款。`
- 托管流程(标题 `四步开始,简单透明`):`接入官方 Key` / `平台智能分发` / `实时计量计费` / `多币种结算`(各含一句说明,见 mockup)。
- 安全保障(标题 `把安全做到供应商敢托付的程度`):`端到端加密存储,供应商之间数据严格隔离、互不可见` / `防套现结算:成交价逐笔快照,事后改价不影响已结算金额` / `原子幂等结算 + 资金账本,杜绝重复打款、全程可审计对账` / `账号级登录防暴破、可吊销会话、SSRF 校验等纵深防护`
- 渠道(标题 `主流官方渠道,统一接单`):`Claude (Anthropic)` / `AWS Bedrock` / `OpenAI` / `更多持续接入`
- 底部 CTA:`现在就开始托管,让你的官方额度产生稳定收益` + `成为供应商`

## 附录 B · 设计稿

`docs/superpowers/mockups/2026-06-15-home-v6/version-b2-refined.html`(基调)、`version-b3-primary-compare.html`(主色对比,Sapphire 选定)。
