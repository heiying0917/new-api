# P1-B 供应商身份（前端）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan unit-by-unit. Steps use checkbox (`- [ ]`) syntax.

> **⚠️ 提交策略（项目铁律）：** 禁止自动 git commit/push。每单元跑通校验后**停下汇报**，等用户指令再提交。

> **⚠️ 前端无单测框架（仅 1 个 .test.tsx）。** 本计划不写 TDD 单测；验证方式 = `cd web/default && bun run lint` + 类型检查/构建 `bun run build`（在 Docker 外本地 `bun install` 后可跑）+ 手动 UI 冒烟。若本地无 bun，则以 `bun run build` 在容器内构建为准。

**Goal:** 为 P1-A 的供应商后端补齐前端：前端供应商角色、注册表单手机号、超管专属「供应商管理」菜单/路由/页面（列表+编辑），打通注册→管理闭环。

**Architecture:** 复用 new-api 前端既有模式（TanStack 文件路由 + Zustand auth-store + React Query + RHF/Zod + Base UI + i18next）。供应商管理页镜像 `features/users/*` 结构，数据接 P1-A 的 `/api/supplier/`。菜单项做 item 级 `minRole` 门控，确保仅超管(role≥100)可见。

**Tech Stack:** React 19 / TypeScript / TanStack Router & Query / react-hook-form / zod / Base UI / Tailwind / lucide-react / i18next。

**基线接口（P1-A 已实现）：**
- `GET /api/supplier/?p=&page_size=` → `{success, data:{items:SupplierListItem[], total}}`
- `GET /api/supplier/search?keyword=&p=&page_size=`
- `PUT /api/supplier/` body `{user_id, priority?, enabled?, settlement_mode?, settlement_cycle?, remark?}`
- `SupplierListItem` 字段：`user_id, username, email, phone, user_status, priority, enabled, settlement_mode("manual"|"auto"), settlement_cycle("day"|"week"|"month"), remark`
- 注册接口 `POST /api/user/register` 现要求 `phone`（必填，≤20）。

---

## Unit B1 — 前端角色常量 + 注册表单手机号

### Files
- Modify `web/default/src/lib/roles.ts`
- Modify `web/default/src/features/users/constants.ts`
- Modify `web/default/src/stores/auth-store.ts`
- Modify `web/default/src/features/auth/constants.ts`（registerFormSchema）
- Modify `web/default/src/features/auth/types.ts`（RegisterPayload）
- Modify `web/default/src/features/auth/sign-up/components/sign-up-form.tsx`（phone 字段 + 提交传参）
- Modify `web/default/src/features/auth/api.ts`（若 register payload 在此组装则补 phone）
- i18n: `web/default/src/i18n/locales/zh.json` 与 `en.json`（新增 `"Phone"`, `"Enter your phone number"`, `"Phone is required"`）

- [ ] **Step 1 — roles.ts 加 SUPPLIER**

在 `ROLE` 对象加 `SUPPLIER: 5`（置于 USER 与 ADMIN 之间）；在 `ROLE_LABEL_KEYS` 加 `[ROLE.SUPPLIER]: 'Supplier'`：
```typescript
export const ROLE = {
  GUEST: 0,
  USER: 1,
  SUPPLIER: 5,
  ADMIN: 10,
  SUPER_ADMIN: 100,
} as const
```
```typescript
const ROLE_LABEL_KEYS: Record<RoleValue, string> = {
  [ROLE.SUPER_ADMIN]: 'Super Admin',
  [ROLE.ADMIN]: 'Admin',
  [ROLE.SUPPLIER]: 'Supplier',
  [ROLE.USER]: 'User',
  [ROLE.GUEST]: 'Guest',
}
```

- [ ] **Step 2 — users/constants.ts 角色选项加 Supplier**

在 `USER_ROLE` 加 `SUPPLIER: 5`；在 `USER_ROLES` 映射加 Supplier 项（图标复用一个已 import 的 lucide 图标，如 `User`，避免新增 import 失败——优先用已存在的）；在 `getUserRoleOptions` 返回数组加：
```typescript
{ label: t('Supplier'), value: String(USER_ROLE.SUPPLIER), icon: User },
```
（确认文件顶部已 import 所用图标；若用 `User` 已 import 则无需改 import。）

- [ ] **Step 3 — auth-store.ts AuthUser 加 phone（可选字段）**

在 `AuthUser` 接口加：
```typescript
  phone?: string
```
（放在 `email?: string` 附近。仅类型补充，不改逻辑。）

- [ ] **Step 4 — 注册 schema 加 phone（必填）**

`features/auth/constants.ts` 的 `registerFormSchema` 的 `.object({...})` 内加：
```typescript
    phone: z.string().min(1, 'Phone is required').max(20, 'Phone is too long'),
```

- [ ] **Step 5 — RegisterPayload 加 phone**

`features/auth/types.ts` 的 `RegisterPayload` 接口加：
```typescript
  phone: string
```

- [ ] **Step 6 — 注册表单加 phone 字段并提交**

`features/auth/sign-up/components/sign-up-form.tsx`：
(a) 在 Username FormField 之后插入 phone 字段（始终显示、必填）：
```tsx
{/* Phone Field */}
<FormField
  control={form.control}
  name='phone'
  render={({ field }) => (
    <FormItem>
      <FormLabel>{t('Phone')}</FormLabel>
      <FormControl>
        <Input placeholder={t('Enter your phone number')} {...field} />
      </FormControl>
      <FormMessage />
    </FormItem>
  )}
/>
```
(b) 确认 `useForm` 的 `defaultValues` 含 `phone: ''`（若有显式 defaultValues 对象则补上）。
(c) `onSubmit`（约 159-166 行）内的 `register({ username, password, email, verification_code, aff_code, turnstile })` 调用对象加一行 `phone: data.phone,`。

- [ ] **Step 7 — register API 透传 phone**

检查 `features/auth/api.ts` 的 `register` 函数：若它显式挑字段组装 body，补 `phone`；若直接透传 payload 对象则无需改。

- [ ] **Step 8 — i18n**

`zh.json` 与 `en.json` 的 `"translation"` 内新增（zh 给中文）：
```json
"Supplier": "供应商",
"Phone": "手机号",
"Enter your phone number": "请输入手机号",
"Phone is required": "手机号必填",
"Phone is too long": "手机号过长"
```
（en.json 对应英文值。）

- [ ] **Step 9 — 校验**

Run（在 `web/default/`）: `bun run lint`
Run: `bun run build`（或容器内构建）
Expected: 通过，无类型错误。手动冒烟：注册页出现手机号输入框、必填校验生效。

- [ ] **Step 10 — Checkpoint**：lint/build 通过 → 停下汇报，等提交指令。

---

## Unit B2 — 菜单项 + 路由 + 超管守卫

### Files
- Modify `web/default/src/hooks/use-sidebar-data.ts`（admin 组加 Suppliers 项 + 图标 import）
- Modify `web/default/src/hooks/use-sidebar-view.ts`（item 级 minRole 过滤）
- Modify `web/default/src/components/layout/types.ts`（NavItem 类型加可选 `minRole?: number`）
- Check/Modify `web/default/src/hooks/use-sidebar-config.ts`（确保 `/suppliers` 不被模块覆盖层过滤掉）
- Create `web/default/src/routes/_authenticated/suppliers/index.tsx`

- [ ] **Step 1 — NavItem 类型加 minRole**

`web/default/src/components/layout/types.ts` 中找到单个导航项的类型（NavGroup.items 的元素类型，常见名 `NavItem`/`NavLink`）。为其加可选字段：
```typescript
  minRole?: number
```

- [ ] **Step 2 — admin 组加 Suppliers 菜单项**

`use-sidebar-data.ts`：在 admin 组 `items` 的 `Users` 项之后加：
```typescript
    {
      title: t('Suppliers'),
      url: '/suppliers',
      icon: Building,
      minRole: ROLE.SUPER_ADMIN,
    },
```
并在文件顶部 `lucide-react` import 中加 `Building`；确认已 import `ROLE`（来自 `@/lib/roles`），若无则补 `import { ROLE } from '@/lib/roles'`。

- [ ] **Step 3 — use-sidebar-view.ts 做 item 级 minRole 过滤**

在现有按 group 过滤（`group.id === 'admin' ? isAdmin : true`）的基础上，对保留的 group 再过滤其 items：丢弃 `item.minRole !== undefined && userRole < item.minRole` 的项。示例（在 rootNavGroups 的 useMemo 内，对 filter 后的 group 映射处理 items）：
```typescript
    const isAdmin = userRole !== undefined && userRole >= ROLE.ADMIN
    return configFilteredRoot
      .filter((group) => (group.id === 'admin' ? isAdmin : true))
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) =>
            item.minRole === undefined ||
            (userRole !== undefined && userRole >= item.minRole)
        ),
      }))
```
（确认 `items` 字段名与结构；若 item 还有子项 `items`，本层过滤即可满足需求。保持其余逻辑不变。）

- [ ] **Step 3.5 — 确认模块覆盖层不隐藏新菜单项**

读 `web/default/src/hooks/use-sidebar-config.ts`：若存在 `URL_TO_CONFIG_MAP` / `DEFAULT_SIDEBAR_MODULES` 之类按 URL 过滤的映射，确认未映射的 `/suppliers` 会**默认放行**（不被过滤）。若该机制会过滤未登记的 URL，则为 `/suppliers` 增加相应登记（section `admin`，默认启用）。若未映射的 URL 本就放行，则无需改动，仅记录确认结论。

- [ ] **Step 4 — 新建供应商路由（超管守卫）**

Create `web/default/src/routes/_authenticated/suppliers/index.tsx`：
```typescript
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Suppliers } from '@/features/suppliers'

const suppliersSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  filter: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/suppliers/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: suppliersSearchSchema,
  component: Suppliers,
})
```
> 依赖 Unit B3 提供 `@/features/suppliers` 的 `Suppliers` 导出；B2 与 B3 一起编译验证。TanStack 路由树（`routeTree.gen.ts`）由 dev/build 自动生成，无需手改。

- [ ] **Step 5 — 校验**（与 B3 合并）：`bun run lint && bun run build`。手动冒烟：超管登录侧栏出现「供应商管理」，普通管理员/供应商不出现；访问 `/suppliers` 非超管被重定向 `/403`。

- [ ] **Step 6 — Checkpoint**（与 B3 合并 checkpoint）。

---

## Unit B3 — 供应商管理 feature 模块（镜像 users）+ i18n

> **实现方式：完整复制 `web/default/src/features/users/` 的结构与写法，按下方映射改字段、改接口、删用不到的部分。** 读 users 各文件作模板。

### Files（全部新建于 `web/default/src/features/suppliers/`）
- `types.ts` — Supplier 列表项与 API 类型
- `api.ts` — 接 `/api/supplier/`
- `constants.ts` — 状态/结算枚举选项、消息常量
- `index.tsx` — 页面入口（镜像 users/index.tsx）
- `components/suppliers-provider.tsx`（镜像 users-provider）
- `components/suppliers-columns.tsx`
- `components/suppliers-table.tsx`
- `components/suppliers-mutate-drawer.tsx`
- `components/suppliers-primary-buttons.tsx`（可仅放标题/刷新，无「新建」——供应商由注册产生，不在此创建）

### 字段/接口映射（精确）
- 列表行类型 `Supplier`（对应后端 `SupplierListItem`）：
```typescript
export interface Supplier {
  user_id: number
  username: string
  email: string
  phone: string
  user_status: number
  priority: number
  enabled: boolean
  settlement_mode: 'manual' | 'auto'
  settlement_cycle: 'day' | 'week' | 'month'
  remark: string
}
```
- `api.ts`（镜像 users/api.ts 的 axios `api` 用法）：
```typescript
import { api } from '@/lib/api'
export async function getSuppliers(params: { p?: number; page_size?: number } = {}) {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(`/api/supplier/?p=${p}&page_size=${page_size}`)
  return res.data
}
export async function searchSuppliers(params: { keyword?: string; p?: number; page_size?: number }) {
  const { keyword = '', p = 1, page_size = 20 } = params
  const qs = new URLSearchParams()
  qs.set('keyword', keyword); qs.set('p', String(p)); qs.set('page_size', String(page_size))
  const res = await api.get(`/api/supplier/search?${qs.toString()}`)
  return res.data
}
export async function updateSupplier(payload: {
  user_id: number; priority?: number; enabled?: boolean;
  settlement_mode?: string; settlement_cycle?: string; remark?: string
}) {
  const res = await api.put('/api/supplier/', payload)
  return res.data
}
```
- **列表列**（columns）：ID(user_id) / 用户名(username) / 邮箱(email) / 手机号(phone) / 优先级(priority，数字 badge) / 启用(enabled，StatusBadge 或开关展示) / 结算(settlement_mode + settlement_cycle 文案) / 备注(remark, LongText) / 操作(编辑)。**不含"价格"列**（依赖 P2 渠道成本价，暂缺数据，后续补）。镜像 users-columns.tsx 的写法（DataTableColumnHeader、StatusBadge、LongText、DataTableRowActions）。行操作仅「编辑」（去掉删除/配额/启用切换等用户专有动作，或保留启用切换→调 updateSupplier 的 enabled）。
- **编辑抽屉**（suppliers-mutate-drawer.tsx，镜像 users-mutate-drawer 的 Sheet+RHF+zod）：字段 = 优先级(number input) / 启用(switch 或 select) / 结算方式(select: manual|auto) / 结算周期(select: day|week|month) / 备注(textarea)。**只有"更新"模式，无"创建"**。提交 → `updateSupplier({user_id: currentRow.user_id, ...})`。zod schema：
```typescript
const supplierFormSchema = z.object({
  priority: z.coerce.number().int().min(0),
  enabled: z.boolean(),
  settlement_mode: z.enum(['manual', 'auto']),
  settlement_cycle: z.enum(['day', 'week', 'month']),
  remark: z.string().max(255).optional().or(z.literal('')),
})
```
- **table**（suppliers-table.tsx，镜像 users-table.tsx）：`getRouteApi('/_authenticated/suppliers/')`、`useTableUrlState`（仅 globalFilter 'filter' + 分页，无 role/status/group 列过滤）、React Query queryKey `['suppliers', p, pageSize, globalFilter, refreshTrigger]`，有 keyword 走 `searchSuppliers` 否则 `getSuppliers`，`useDataTable` + `DataTablePage`。行主键用 `user_id`（注意 `getRowId: (row) => String(row.user_id)`）。
- **provider**：镜像 users-provider，`open` 仅需 `'update'`，`currentRow: Supplier | null`，`refreshTrigger`/`triggerRefresh`。
- **index.tsx**：镜像 users/index.tsx，标题 `t('Supplier Management')`，去掉删除对话框与新建按钮（无创建）。

- [ ] **Step 1 — 建 types.ts / constants.ts / api.ts**（按上方映射）。constants.ts 提供结算方式/周期的选项数组（用于 select 与列展示）与成功/失败消息 key。

- [ ] **Step 2 — 建 provider / columns / table / mutate-drawer / primary-buttons / index.tsx**（镜像 users 对应文件，套用字段映射）。

- [ ] **Step 3 — i18n**：`zh.json`/`en.json` 新增：
```json
"Supplier Management": "供应商管理",
"Priority": "优先级",
"Enabled": "启用",
"Settlement": "结算",
"Settlement Mode": "结算方式",
"Settlement Cycle": "结算周期",
"Manual": "手动",
"Auto": "自动",
"Daily": "按天",
"Weekly": "按周",
"Monthly": "按月",
"Remark": "备注",
"Supplier updated successfully": "供应商更新成功"
```
（去重：已存在的 key 不重复添加。）

- [ ] **Step 4 — 校验（B2+B3 合并）**

Run（`web/default/`）: `bun run lint`
Run: `bun run build`
Expected：编译通过，路由树生成含 `/suppliers`。
手动冒烟：超管访问 `/suppliers` → 列表加载（注册几个供应商后有数据）；点编辑改优先级/启用/结算/备注 → 保存成功、列表刷新；普通用户访问被挡。

- [ ] **Step 5 — Checkpoint**：lint/build 通过 + 冒烟 OK → 停下汇报。

---

## 验收标准（P1-B）
- 注册页含手机号（必填）；提交带 phone；注册成功为供应商。
- `ROLE.SUPPLIER=5` 前端常量存在；用户/供应商列表能正确显示「供应商」角色标签。
- 侧栏「供应商管理」仅超管(role≥100)可见；非超管访问 `/suppliers` 被重定向 `/403`。
- 供应商管理页：列表(用户名/邮箱/手机/优先级/启用/结算/备注)、搜索、分页、编辑(优先级/启用/结算方式/周期/备注)保存生效。
- `bun run lint` 与 `bun run build` 通过。

## 不在本计划内
- 「价格」展示列（依赖 P2 渠道成本价）。
- 供应商自助渠道管理（P2）。

## 风险
- nav item `minRole` 过滤改动的是共享 hook `use-sidebar-view.ts`：改完务必验证其他菜单项不受影响（无 minRole 的项照常显示）。
- 本地若无 `bun`/`node_modules`，`bun run build` 需先 `bun install`（或在容器内构建验证）。
- `features/users` 的部分子组件（如 DataTable、StatusBadge、LongText）为共享组件，直接复用即可，勿复制其源码。
