# v6 首页重做 + 登录注册 Tab 焦点 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 classic 默认首页重做成"面向供应商的官方 Key 托管平台"招募落地页(浅色 SaaS 基调、深宝蓝主色、明暗跟随系统、可配数字),并修复登录/注册 Tab 焦点落在密码小眼睛的问题。

**Architecture:** 纯前端为主 + 一个只读透出的 `/api/status` 字段 + 一个个性化选项。落地页只渲染中间内容板块(复用全局 HeaderBar/FooterBar,品牌红线自动保留),全用 Semi 令牌取色(明暗自动适配)。全局主色通过覆盖 Semi `--semi-color-primary` 令牌族实现(首页+控制台一起换)。

**Tech Stack:** React 18 + Semi Design(`@douyinfe/semi-ui`/`semi-icons`)+ Tailwind(`semi-color-*` 映射)+ i18next(单 `zh-CN`,中文做 key)+ Go/Gin 后端 status。

---

## 关键约束(执行前必读)

- **提交纪律(全局规则)**:本计划中的 `git commit` 仅为占位说明。**实际 commit / 构建镜像 / 部署必须等用户明确指令**。执行时每个任务完成 → 构建 + 自测 + 汇报,**不自行 commit**。每个任务给出"建议提交信息"供用户说"提交"时使用。
- **品牌红线(Rule 5)**:不改动 new-api / QuantumNous 任何标识。落地页复用全局 FooterBar(品牌在此),不自建 footer。
- **核心业务零改动**:官 key 管理 / 计费 / 结算的后端逻辑、模型、relay 一律不动。本期后端仅新增 1 个选项默认值 + status 透出 1 个只读字段。
- **JSON 规则(Rule 1)**:后端如需 JSON,用 `common.Marshal/Unmarshal`(本计划后端不涉及 JSON 序列化,前端用浏览器 `JSON.parse`)。
- **构建命令**:前端 `cd web/classic && bun run build`;后端 `go build ./...`、`go test ./...`。

## 文件结构(改动地图)

新增:
- `web/classic/src/pages/Home/landing/SupplierLanding.jsx` — 组装各板块 + 顶部留白
- `web/classic/src/pages/Home/landing/Hero.jsx` — 主视觉 + 双 CTA + 收益示意面板 + 信任数据条(读 HomeSupplierStats)
- `web/classic/src/pages/Home/landing/Advantages.jsx` — 4 卡核心优势(数据驱动)
- `web/classic/src/pages/Home/landing/Process.jsx` — 4 步流程(数据驱动)
- `web/classic/src/pages/Home/landing/Security.jsx` — 安全保障 4 点(数据驱动)
- `web/classic/src/pages/Home/landing/Channels.jsx` — 渠道徽章(数据驱动)
- `web/classic/src/pages/Home/landing/CtaBand.jsx` — 底部 CTA
- `web/classic/src/pages/Home/landing/landing.css` — 本页渐变/光晕/动效(全部基于 Semi 令牌 var)

修改:
- `web/classic/src/index.css` — 覆盖 Semi 主色令牌(浅/深 Sapphire)
- `web/classic/src/pages/Home/index.jsx` — 空内容分支改渲染 `<SupplierLanding />`
- `web/classic/src/components/settings/OtherSetting.jsx` — 新增"首页供应商数据"卡片(`HomeSupplierStats`)
- `web/classic/src/components/auth/LoginForm.jsx`、`RegisterForm.jsx` — 密码眼睛移出 Tab 顺序
- `web/default/src/components/password-input.tsx` — 眼睛 `<Button>` 加 `tabIndex={-1}`
- `controller/misc.go` — GetStatus 透出 `HomeSupplierStats`
- `model/option.go` — 注册 `HomeSupplierStats` 默认值
- `web/classic/src/i18n/locales/zh-CN.json` — 新增首页文案 identity 键

---

## Task 1: 全局主色 → 深宝蓝 Sapphire(Semi 令牌覆盖)

先做主色,后续落地页 `var(--semi-color-primary)` 即自动是 Sapphire。

**Files:**
- Modify: `web/classic/src/index.css`(文件末尾追加覆盖块)

- [ ] **Step 1: 在 `web/classic/src/index.css` 末尾追加 Semi 主色覆盖**

```css
/* === v6: 全局主色 → 深宝蓝 Sapphire（覆盖 Semi 默认蓝；首页与控制台统一） === */
/* 浅色：body:not([theme-mode='dark']) 特异性(0,1,1) 胜过 Semi 的 :root，且位于 semi.css 之后 */
body:not([theme-mode='dark']) {
  --semi-color-primary: #2052CC;
  --semi-color-primary-hover: #1B47B3;
  --semi-color-primary-active: #143A91;
  --semi-color-primary-disabled: #AEC2EC;
  --semi-color-primary-light-default: #EAF0FB;
  --semi-color-primary-light-hover: #DCE6F8;
  --semi-color-primary-light-active: #CEDCF4;
  --semi-color-link: #2052CC;
  --semi-color-link-hover: #1B47B3;
  --semi-color-link-active: #143A91;
  --semi-color-link-visited: #2052CC;
  --semi-color-focus-border: #2052CC;
}
body[theme-mode='dark'] {
  --semi-color-primary: #7AA2FF;
  --semi-color-primary-hover: #9BBAFF;
  --semi-color-primary-active: #BFD3FF;
  --semi-color-primary-disabled: rgba(122, 162, 255, 0.4);
  --semi-color-primary-light-default: rgba(122, 162, 255, 0.2);
  --semi-color-primary-light-hover: rgba(122, 162, 255, 0.3);
  --semi-color-primary-light-active: rgba(122, 162, 255, 0.4);
  --semi-color-link: #7AA2FF;
  --semi-color-link-hover: #9BBAFF;
  --semi-color-link-active: #BFD3FF;
  --semi-color-link-visited: #7AA2FF;
  --semi-color-focus-border: #7AA2FF;
}
```

- [ ] **Step 2: 构建 classic**

Run: `cd web/classic && bun run build`
Expected: 构建成功无报错。

- [ ] **Step 3: 用 Playwright 核验主色全局生效 + 对比度**

启动本地预览(或用运行容器 http://localhost:5001),登录后逐页(登录页、Dashboard `/console`、渠道、结算、供应商页)在浅色/深色下截图。
验证点:① 主按钮/链接/选中态均为 Sapphire(浅 `#2052CC`、深 `#7AA2FF`);② 浅色深蓝按钮上白字、深色浅蓝按钮上深字均清晰可读;③ 无明显突兀。
程序化确认:`getComputedStyle(document.body).getPropertyValue('--semi-color-primary')` 浅色返回 `#2052CC`、深色返回 `#7AA2FF`。
若发现个别组件仍是旧蓝,排查其是否直接引用 `--semi-blue-*` 色阶;如是,在同两个选择器内补 `--semi-blue-5` 等覆盖,再次验证。

- [ ] **Step 4: 提交(等用户指令)**

建议提交信息:`feat(classic): 全局主色切换为深宝蓝 Sapphire（Semi 令牌覆盖，明暗两套）`

---

## Task 2: 后端 — 注册可配选项 + status 透出

**Files:**
- Modify: `model/option.go`(约 69 行,`HomePageContent` 注册附近)
- Modify: `controller/misc.go`(GetStatus,约 106 行 `HeaderNavModules` 附近)

- [ ] **Step 1: 在 `model/option.go` 注册默认值**

在 `common.OptionMap["HomePageContent"] = ""`(约 69 行)下一行新增:

```go
	common.OptionMap["HomeSupplierStats"] = ""
```

- [ ] **Step 2: 在 `controller/misc.go` 的 GetStatus 透出该字段**

在 `"HeaderNavModules": common.OptionMap["HeaderNavModules"],`(约 106 行)下一行新增:

```go
		"HomeSupplierStats":   common.OptionMap["HomeSupplierStats"],
```

- [ ] **Step 3: 构建后端**

Run: `go build ./...`
Expected: 成功无报错。

- [ ] **Step 4: 验证 status 透出**

启动后端(或重建容器),`curl -s http://localhost:5001/api/status | grep HomeSupplierStats`
Expected: 返回里含 `"HomeSupplierStats":""`(未配置为空串)。
回归:`go test ./...` 不新增失败(与改动前基线对比)。

- [ ] **Step 5: 提交(等用户指令)**

建议提交信息:`feat: 新增 HomeSupplierStats 选项并经 /api/status 只读透出(首页可配数字)`

---

## Task 3: 供应商落地页组件 + 接入 Home

落地页 = 中间内容板块;复用全局 HeaderBar/FooterBar。视觉以 `docs/superpowers/mockups/2026-06-15-home-v6/version-b2-refined.html` 为准,移植为 JSX,**颜色一律换成 Semi 令牌**。

**取色映射表(移植 landing.css / className 时严格遵守):**

| 用途 | 用 |
|---|---|
| 页面/板块底色 | `var(--semi-color-bg-0)` |
| 交替板块浅底 | `var(--semi-color-fill-0)` |
| 卡片面 | `var(--semi-color-bg-1)` + `1px solid var(--semi-color-border)` |
| 强文本/次文本/弱文本 | `var(--semi-color-text-0)` / `-text-1` / `-text-2` |
| 发丝边框 | `var(--semi-color-border)` |
| 主色 | `var(--semi-color-primary)` |
| 高级渐变(蓝→靛蓝) | `linear-gradient(135deg, var(--semi-color-primary), rgba(var(--semi-indigo-5), 1))` |

Tailwind 等价类(已在 `tailwind.config.js` 映射):`bg-semi-color-bg-0/1`、`text-semi-color-text-0/1/2`、`border-semi-color-border`、`text-semi-color-primary` 等。

**Files:**
- Create: `web/classic/src/pages/Home/landing/{SupplierLanding,Hero,Advantages,Process,Security,Channels,CtaBand}.jsx`、`landing.css`
- Modify: `web/classic/src/pages/Home/index.jsx`

- [ ] **Step 1: 创建 `SupplierLanding.jsx`(组装 + 顶部留白避开固定顶栏)**

```jsx
import React from 'react';
import './landing.css';
import Hero from './Hero';
import Advantages from './Advantages';
import Process from './Process';
import Security from './Security';
import Channels from './Channels';
import CtaBand from './CtaBand';

const SupplierLanding = () => (
  <div className='supplier-landing w-full overflow-x-hidden bg-semi-color-bg-0'>
    <Hero />
    <Advantages />
    <Process />
    <Security />
    <Channels />
    <CtaBand />
  </div>
);

export default SupplierLanding;
```

- [ ] **Step 2: 创建 `Hero.jsx`(主视觉 + 双 CTA + 收益示意面板 + 信任条,读 HomeSupplierStats)**

收益面板为**静态示意**(明确标"示意"),金额非承诺。信任条优先读 `statusState.status.HomeSupplierStats`,未配置/无 value → 回退定性条目。

```jsx
import React, { useContext } from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../context/Status';

const Hero = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);

  const fallbackStats = [
    { value: '', label: t('数百家'), caption: t('企业客户') },
    { value: '', label: t('持续'), caption: t('充足订单') },
    { value: '', label: t('多币种'), caption: t('快速结算') },
    { value: '', label: t('端到端'), caption: t('加密隔离') },
  ];
  let stats = fallbackStats;
  const raw = statusState?.status?.HomeSupplierStats;
  if (raw) {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) {
        stats = parsed.slice(0, 4).map((s) => ({
          value: s.value || '',
          label: s.label || '',
          caption: s.caption || '',
        }));
      }
    } catch (e) {
      /* 解析失败保留定性默认 */
    }
  }

  const sampleRows = [
    { name: 'Claude (Anthropic)', masked: 'sk-ant-••••', amount: '¥ 12,480' },
    { name: 'AWS Bedrock', masked: 'AKIA••••', amount: '¥ 8,920' },
    { name: 'OpenAI', masked: 'sk-••••', amount: '¥ 6,310' },
  ];

  return (
    <section className='landing-hero'>
      <div className='landing-hero__bg' aria-hidden='true' />
      <div className='landing-container landing-hero__grid'>
        <div className='landing-hero__copy'>
          <span className='landing-eyebrow'>{t('面向供应商 · 官方额度变现')}</span>
          <h1 className='landing-hero__title'>
            {t('托管你的官方 Key,接入全球订单')}
          </h1>
          <p className='landing-hero__sub'>
            {t(
              '我们为全球数百家企业稳定供应 AI 大模型 token,订单量充足。把你闲置的 Claude / AWS / OpenAI 官方额度托管给我们——安全隔离、实时计量、多币种快速结算。',
            )}
          </p>
          <div className='landing-hero__cta'>
            <Link to='/register'>
              <Button theme='solid' type='primary' size='large' className='!rounded-xl px-7'>
                {t('成为供应商')}
              </Button>
            </Link>
            <Link to='/login'>
              <Button size='large' className='!rounded-xl px-7'>
                {t('登录控制台')}
              </Button>
            </Link>
          </div>
          <div className='landing-stats'>
            {stats.map((s, i) => (
              <div className='landing-stats__item' key={i}>
                <div className='landing-stats__value'>{s.value || s.label}</div>
                <div className='landing-stats__label'>{s.value ? s.label : s.caption}</div>
              </div>
            ))}
          </div>
        </div>
        <div className='landing-panel' role='img' aria-label={t('收益示意')}>
          <div className='landing-panel__head'>
            <span>{t('我的托管渠道')}</span>
            <span className='landing-panel__badge'>{t('示意')}</span>
          </div>
          {sampleRows.map((r) => (
            <div className='landing-panel__row' key={r.name}>
              <div className='landing-panel__name'>
                <div>{r.name}</div>
                <div className='landing-panel__mask'>{r.masked} · {t('已加密')}</div>
              </div>
              <div className='landing-panel__amt'>{r.amount}</div>
            </div>
          ))}
          <div className='landing-panel__total'>
            <span>{t('本周期累计收益(示意)')}</span>
            <span className='landing-panel__total-amt'>¥ 27,710</span>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Hero;
```

- [ ] **Step 3: 创建 `Advantages.jsx`(数据驱动 4 卡)**

```jsx
import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconPulse, IconLayers, IconShield, IconCreditCard } from '@douyinfe/semi-icons';

const Advantages = () => {
  const { t } = useTranslation();
  const items = [
    { icon: <IconPulse />, title: t('订单量充足,Key 不闲置'),
      desc: t('平台聚合全球数百家企业的真实需求,你的官方额度持续接单,稳定产生收益。') },
    { icon: <IconLayers />, title: t('多种官方 Key 托管'),
      desc: t('支持 Claude(Anthropic)、AWS Bedrock、OpenAI 等主流官方渠道,一处托管、统一接单。') },
    { icon: <IconShield />, title: t('关键数据隔离加密'),
      desc: t('官方 Key 加密存储,供应商之间严格隔离、互不可见;遵循最小可见原则。') },
    { icon: <IconCreditCard />, title: t('多币种快速结算'),
      desc: t('按实际消耗实时计量,成交价逐笔冻结(不受事后改价影响),支持多币种、快速回款。') },
  ];
  return (
    <section className='landing-section'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('核心优势')}</span>
        <h2 className='landing-h2'>{t('为什么把官方 Key 托管给我们')}</h2>
        <div className='landing-grid-4'>
          {items.map((it, i) => (
            <div className='landing-card' key={i}>
              <div className='landing-card__icon'>{it.icon}</div>
              <h3 className='landing-card__title'>{it.title}</h3>
              <p className='landing-card__desc'>{it.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Advantages;
```

- [ ] **Step 4: 创建 `Process.jsx`(数据驱动 4 步)**

```jsx
import React from 'react';
import { useTranslation } from 'react-i18next';

const Process = () => {
  const { t } = useTranslation();
  const steps = [
    { n: '1', title: t('接入官方 Key'), desc: t('提交你的 Claude / AWS / OpenAI 官方凭证,加密入库。') },
    { n: '2', title: t('平台智能分发'), desc: t('全球企业订单按规则分发到你的渠道,真实流量、稳定调用。') },
    { n: '3', title: t('实时计量计费'), desc: t('每笔调用按官方口径计量,成交价即时快照,账目清晰可对账。') },
    { n: '4', title: t('多币种结算'), desc: t('按周期生成结算单,核对无误后快速打款,支持多币种。') },
  ];
  return (
    <section className='landing-section landing-section--alt'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('托管流程')}</span>
        <h2 className='landing-h2'>{t('四步开始,简单透明')}</h2>
        <div className='landing-grid-4'>
          {steps.map((s) => (
            <div className='landing-step' key={s.n}>
              <div className='landing-step__num'>{s.n}</div>
              <h3 className='landing-card__title'>{s.title}</h3>
              <p className='landing-card__desc'>{s.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Process;
```

- [ ] **Step 5: 创建 `Security.jsx`(数据驱动 4 点)**

```jsx
import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconTick } from '@douyinfe/semi-icons';

const Security = () => {
  const { t } = useTranslation();
  const points = [
    t('端到端加密存储,供应商之间数据严格隔离、互不可见'),
    t('防套现结算:成交价逐笔快照,事后改价不影响已结算金额'),
    t('原子幂等结算 + 资金账本,杜绝重复打款、全程可审计对账'),
    t('账号级登录防暴破、可吊销会话、SSRF 校验等纵深防护'),
  ];
  return (
    <section className='landing-section'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('安全保障')}</span>
        <h2 className='landing-h2'>{t('把安全做到供应商敢托付的程度')}</h2>
        <ul className='landing-seclist'>
          {points.map((p, i) => (
            <li className='landing-seclist__item' key={i}>
              <IconTick className='landing-seclist__tick' />
              <span>{p}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
};

export default Security;
```

- [ ] **Step 6: 创建 `Channels.jsx`(渠道徽章)**

```jsx
import React from 'react';
import { useTranslation } from 'react-i18next';

const Channels = () => {
  const { t } = useTranslation();
  const list = ['Claude (Anthropic)', 'AWS Bedrock', 'OpenAI', t('更多持续接入')];
  return (
    <section className='landing-section landing-section--alt'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('支持的官方渠道')}</span>
        <h2 className='landing-h2'>{t('主流官方渠道,统一接单')}</h2>
        <div className='landing-badges'>
          {list.map((c) => (
            <div className='landing-badge' key={c}>{c}</div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Channels;
```

- [ ] **Step 7: 创建 `CtaBand.jsx`(底部行动号召)**

```jsx
import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const CtaBand = () => {
  const { t } = useTranslation();
  return (
    <section className='landing-ctaband'>
      <div className='landing-container landing-ctaband__inner'>
        <h2 className='landing-ctaband__title'>
          {t('现在就开始托管,让你的官方额度产生稳定收益')}
        </h2>
        <Link to='/register'>
          <Button theme='solid' type='primary' size='large' className='!rounded-xl px-8'>
            {t('成为供应商')}
          </Button>
        </Link>
      </div>
    </section>
  );
};

export default CtaBand;
```

- [ ] **Step 8: 创建 `landing.css`(移植 mockup 视觉,颜色换 Semi 令牌)**

从 `docs/superpowers/mockups/2026-06-15-home-v6/version-b2-refined.html` 的 `<style>` 移植以下选择器对应的视觉规则(布局、间距、圆角、阴影、悬停、渐变、光晕、响应式),**把每个硬编码颜色替换为上文映射表的 Semi 令牌 var**:`.landing-container .landing-hero(.__bg/__grid/__copy/__title/__sub/__cta) .landing-eyebrow .landing-stats(.__item/__value/__label) .landing-panel(.__head/__badge/__row/__name/__mask/__amt/__total/__total-amt) .landing-section(--alt) .landing-h2 .landing-grid-4 .landing-card(.__icon/__title/__desc) .landing-step(.__num) .landing-seclist(.__item/__tick) .landing-badges .landing-badge .landing-ctaband(.__inner/__title)`。
要点:① 顶部 `.landing-hero` 加 `padding-top` 避开固定顶栏(参照现有 `pt-24`≈6rem);② 渐变统一用 `linear-gradient(135deg, var(--semi-color-primary), rgba(var(--semi-indigo-5),1))`;③ 不写死任何明暗色——明暗由 Semi 令牌随 `body[theme-mode]` 自动切换;④ `max-width` 容器 + 移动端单列。

- [ ] **Step 9: 接入 `Home/index.jsx`(替换空内容分支)**

在 `web/classic/src/pages/Home/index.jsx`:顶部加 `import SupplierLanding from './landing/SupplierLanding';`;把空内容分支(当前 159–336 行 `<div className='classic-home-default'>…</div>` 整块,即"统一的大模型接口网关"硬编码落地页)替换为:

```jsx
        <SupplierLanding />
```

保持外层 `homePageContentLoaded && homePageContent === '' ? ( … ) : ( …自定义内容/iframe… )` 三元结构与 else 分支不变。可删除仅供旧落地页使用、替换后未再引用的 import(`@lobehub/icons`、`ScrollList/ScrollItem`、`API_ENDPOINTS`、`IconPlay/IconFile/IconGithubLogo/IconCopy`、`handleCopyBaseURL`、`endpointItems`/`endpointIndex` 相关 effect 等);保留 `NoticeModal`、`displayHomePageContent`、`StatusContext`、`useActualTheme` 等仍被使用的部分。删 import 后必须构建通过。

- [ ] **Step 10: 构建并视觉验证**

Run: `cd web/classic && bun run build`
Expected: 构建成功。
预览(本地或容器)未登录访问 `/`,Playwright 浅色+深色、桌面+移动各截一张:逐板块比对 mockup;信任条未配置时显示定性默认(数百家/持续/多币种/端到端),无编造数字;主色为 Sapphire;FooterBar 的 new-api/QuantumNous 仍在。已登录访问 `/` 仍被 `AuthRedirect` 跳控制台(未回归)。

- [ ] **Step 11: 提交(等用户指令)**

建议提交信息:`feat(classic): 首页重做为供应商招募落地页(Semi 令牌取色/明暗自适应/i18n/可配数字)`

---

## Task 4: 管理员设置 — 首页供应商数据表单

在 `OtherSetting.jsx` 增加一张卡片,4 行(数值/标签/说明)序列化为 `HomeSupplierStats` JSON,经现有 `updateOption` 保存。

**Files:**
- Modify: `web/classic/src/components/settings/OtherSetting.jsx`

- [ ] **Step 1: inputs 默认值加 HomeSupplierStats**

在 `useState({ … HomePageContent: '' })`(约 42–51 行)对象里加一项:

```jsx
    HomeSupplierStats: '',
```

- [ ] **Step 2: 新增提交函数(复用 updateOption)**

在 `submitFooter` 之后新增:

```jsx
  // 个性化设置 - 首页供应商数据
  const submitHomeSupplierStats = async () => {
    try {
      setLoadingInput((l) => ({ ...l, HomeSupplierStats: true }));
      // 校验 JSON 合法(空串表示用定性默认)
      if (inputs.HomeSupplierStats) {
        JSON.parse(inputs.HomeSupplierStats);
      }
      await updateOption('HomeSupplierStats', inputs.HomeSupplierStats);
      showSuccess(t('首页数据已更新'));
    } catch (error) {
      showError(t('首页数据格式有误(应为 JSON 数组)'));
    } finally {
      setLoadingInput((l) => ({ ...l, HomeSupplierStats: false }));
    }
  };
```

- [ ] **Step 3: 在个性化设置区(`HomePageContent` 卡片附近)渲染输入卡片**

按本文件既有卡片写法,新增一个区块:一个多行 `TextArea`(绑定 `inputs.HomeSupplierStats`,`id='HomeSupplierStats'`,`onChange={handleInputChange}`)+ 一个保存 `Button`(`loading={loadingInput.HomeSupplierStats}` `onClick={submitHomeSupplierStats}`),并加说明文案 `t('首页信任数据(JSON 数组,最多 4 项,每项 {value,label,caption};留空则用定性默认,不显示具体数字)')` 与占位示例:

```json
[{"value":"500+","label":"企业客户","caption":""},{"value":"","label":"持续","caption":"充足订单"}]
```

(若本文件个性化区用 Semi `Form`,则用 `Form.TextArea field='HomeSupplierStats'` 并在表单初值里带上;与相邻 `HomePageContent`/`About` 卡片保持同样的组件与栅格写法。)

- [ ] **Step 4: 构建并验证可配 → 显示联动**

Run: `cd web/classic && bun run build`
Expected: 成功。
手测:管理员设置里填入示例 JSON 保存 → 刷新首页,信任条显示 `500+ 企业客户` 等;清空保存 → 首页回退定性默认(数百家/持续/…),无编造数字;填非法 JSON → 报错且不保存。

- [ ] **Step 5: 提交(等用户指令)**

建议提交信息:`feat(classic): 管理员设置新增"首页供应商数据"(HomeSupplierStats)`

---

## Task 5: 登录/注册 Tab 焦点修复

目标:密码框按 Tab 跳到下一个输入框,不停在小眼睛;鼠标点击眼睛仍可切换明文。

**Files:**
- Modify: `web/classic/src/components/auth/LoginForm.jsx`(密码框,约 757–765 行)
- Modify: `web/classic/src/components/auth/RegisterForm.jsx`(密码 + 确认密码,约 606–614 行及其上方密码框)
- Modify: `web/default/src/components/password-input.tsx`(约 50 行眼睛 Button)

- [ ] **Step 1: LoginForm.jsx — 用受控 type + tabIndex=-1 自定义眼睛替换 mode='password'**

确保文件已 `import { useState } from 'react'`、`import { Button } from '@douyinfe/semi-ui'`、`import { IconLock, IconEyeOpened, IconEyeClosed } from '@douyinfe/semi-icons'`(缺则补)。在组件内加 `const [showPwd, setShowPwd] = useState(false);`。把密码 `Form.Input` 改为:

```jsx
                <Form.Input
                  field='password'
                  label={t('密码')}
                  placeholder={t('请输入您的密码')}
                  name='password'
                  type={showPwd ? 'text' : 'password'}
                  onChange={(value) => handleChange('password', value)}
                  prefix={<IconLock />}
                  suffix={
                    <Button
                      theme='borderless'
                      type='tertiary'
                      size='small'
                      tabIndex={-1}
                      icon={showPwd ? <IconEyeOpened /> : <IconEyeClosed />}
                      onClick={() => setShowPwd((v) => !v)}
                    />
                  }
                />
```

- [ ] **Step 2: RegisterForm.jsx — 密码与确认密码同样处理**

同上确保 import。加两个状态 `const [showPwd, setShowPwd] = useState(false);` 与 `const [showPwd2, setShowPwd2] = useState(false);`。把 `field='password'` 与 `field='password2'` 两个 `Form.Input` 的 `mode='password'` 各自替换为 `type={showPwd?'text':'password'}`(/`showPwd2`)+ 对应的 `suffix` 眼睛按钮(`tabIndex={-1}`,`onClick` 切自身状态),写法同 Step 1。

- [ ] **Step 3: default password-input.tsx — 眼睛 Button 加 tabIndex={-1}**

在 `web/default/src/components/password-input.tsx` 的眼睛 `<Button …>`(约 50 行)属性里加:

```tsx
        tabIndex={-1}
```

- [ ] **Step 4: 构建 + 键盘走查**

Run: `cd web/classic && bun run build`
Expected: 成功。
手测(classic 登录页与注册页):从用户名起按 Tab → 密码 →(确认密码)→ 下一控件/提交,**不**停在眼睛;鼠标点眼睛可切换明文。default 主题同样核对(如已切到 default 或单测 password-input)。

- [ ] **Step 5: 提交(等用户指令)**

建议提交信息:`fix(auth): 密码可见性切换按钮移出 Tab 顺序(classic + default)`

---

## Task 6: 整体联调 + 视觉 QA + i18n 收尾

**Files:**
- Modify: `web/classic/src/i18n/locales/zh-CN.json`

- [ ] **Step 1: 补 i18n identity 键(可选但符合约定)**

把本计划新增的所有中文 `t('…')` 源串作为 identity 键加入 `zh-CN.json` 的 `"translation"` 对象(`"中文":"中文"`)。注意:i18next 缺键时回退返回 key(即中文),故页面即使不补也能显示中文;补齐是为对齐项目约定与未来英文版。键里**不得含 ASCII `.`**(默认 keySeparator 为 `.`);本计划文案均用中文标点,安全。

- [ ] **Step 2: 全量构建**

Run: `cd web/classic && bun run build && go build ./...`
Expected: 均成功。

- [ ] **Step 3: 端到端视觉 QA(Playwright)**

未登录 `/`:浅色+深色、桌面(1440)+移动(390)各截图,逐板块核对;切换系统/手动明暗均正常;主色 Sapphire 一致;FooterBar 品牌红线在。控制台若干页主色核对无回归。Tab 走查通过。可配数字:配置/清空两态正确。

- [ ] **Step 4: 回归**

Run: `go test ./...`
Expected: 不新增失败(与本任务开始前基线对比;已知预先存在的失败不计)。

- [ ] **Step 5: 记 WORKLOG**

在 `docs/superpowers/WORKLOG.md` 追加本次实现条目(何时/做了什么/改了哪些文件/如何验证)。

- [ ] **Step 6: 汇报,等用户"提交"指令**

汇总改动 + 自测结果,等用户明确说"提交"再 `git commit`(全局规则)。

---

## 自检(写计划后的 spec 对照)

- **Spec §4.1 落地点(空内容分支 + 不自建 nav/footer)** → Task 3 Step 9 / SupplierLanding。✓
- **Spec §4.3 全局主色 Sapphire** → Task 1。✓
- **Spec §4.4 可配数字 + 诚实默认** → Task 2(后端)+ Task 3 Step 2(前端读+回退)+ Task 4(管理员表单)。✓
- **Spec §4.2 i18n** → 全程 `t()` + Task 6 Step 1。✓
- **Spec §5 Tab 修复** → Task 5。✓
- **Spec §8 验证** → 各 Task 的构建/Playwright/回归步骤 + Task 6。✓
- **品牌红线** → 复用全局 FooterBar,未自建 footer;Task 3/6 验证品牌仍在。✓
- **类型/命名一致**:`HomeSupplierStats`(后端 OptionMap/status 字段、前端 status 读取、管理员表单 key)三处同名;stat 结构 `{value,label,caption}` 前后端一致。✓
- **占位扫描**:无 TBD;landing.css 以 mockup 为明确来源 + 映射表,非占位。✓
