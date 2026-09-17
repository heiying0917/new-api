/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { Toast } from '@douyinfe/semi-ui';
import i18n from '../i18n/i18n';

// 后端 authHelper 在"权限不足"时返回的稳定错误码（message 是翻译文案，不能用于判断）。
export const INSUFFICIENT_PRIVILEGE_CODE = 'INSUFFICIENT_PRIVILEGE';
// 账号已被禁用的稳定错误码：清本地登录态回登录页。
export const USER_BANNED_CODE = 'USER_BANNED';
// 角色变更导致的请求中断：拦截器用它 reject 当前请求，showError 对它静默。
export const ROLE_CHANGE_REDIRECT_MESSAGE = 'ROLE_CHANGE_REDIRECT';

const KNOWN_ROLES = new Set([1, 3, 5, 10, 100]);
let redirecting = false;

export function getStoredUser() {
  try {
    return JSON.parse(localStorage.getItem('user') || 'null');
  } catch {
    return null;
  }
}

export function roleLabel(role) {
  switch (role) {
    case 1:
      return i18n.t('普通用户');
    case 3:
      return i18n.t('观察员');
    case 5:
      return i18n.t('供应商');
    case 10:
      return i18n.t('管理员');
    case 100:
      return i18n.t('超级管理员');
    default:
      return String(role);
  }
}

export function isRoleChangeRedirect(err) {
  if (!err) return false;
  if (typeof err === 'string') return err === ROLE_CHANGE_REDIRECT_MESSAGE;
  return err.message === ROLE_CHANGE_REDIRECT_MESSAGE;
}

function redirectOnce(path, delay) {
  if (redirecting) return;
  redirecting = true;
  setTimeout(() => window.location.assign(path), delay);
}

// 强制登出：清本地用户并回登录页（账号被禁用 / 角色不合法 / 会话失效）。
export function forceLogout(reasonText) {
  localStorage.removeItem('user');
  if (reasonText) {
    Toast.warning(reasonText);
  }
  redirectOnce('/login?expired=true', reasonText ? 800 : 0);
}

// 服务端角色与本地缓存不一致时：更新本地用户、提示、整页跳到控制台首页（所有基于角色的判断一次性重算）。
// 返回 true 表示已接管（调用方应中断当前请求处理）。
export function applyRoleChange(serverRole) {
  const user = getStoredUser();
  if (!user || typeof serverRole !== 'number') return false;
  if (user.role === serverRole) return false;
  if (redirecting) return true;
  if (!KNOWN_ROLES.has(serverRole)) {
    forceLogout(i18n.t('账号状态已变更，请重新登录'));
    return true;
  }
  localStorage.setItem('user', JSON.stringify({ ...user, role: serverRole }));
  Toast.info(
    i18n.t('你的账号角色已变更为 {{role}}，正在切换页面', {
      role: roleLabel(serverRole),
    }),
  );
  redirectOnce('/console', 800);
  return true;
}

// 拦截器入口：识别权限不足响应；若是"角色已变更"则接管并返回 true。
export function handleInsufficientPrivilegeResponse(response) {
  const d = response?.data;
  if (!d || d.success !== false) return false;
  if (d.code === USER_BANNED_CODE && getStoredUser()) {
    forceLogout(i18n.t('账号已被禁用，请联系管理员'));
    return true;
  }
  if (d.code !== INSUFFICIENT_PRIVILEGE_CODE) return false;
  return applyRoleChange(d.role);
}

// 启动核对：用 /api/user/self 的结果同步本地角色/状态。selfData 为接口 data 字段。
export function syncStoredUserWithSelf(selfData) {
  const user = getStoredUser();
  if (!user || !selfData) return false;
  if (selfData.status !== undefined && selfData.status !== 1) {
    forceLogout(i18n.t('账号已被禁用，请联系管理员'));
    return true;
  }
  return applyRoleChange(selfData.role);
}
