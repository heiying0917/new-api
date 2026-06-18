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

// 本项目支持简体中文（zh-CN）与英文（en）。
// 归一化规则：任何中文变体（zh / zh-CN / zh-TW / zh-HK / zh-Hans…）归为 zh-CN，
// 其余一切语言归为 en（"非中文即英文"）。
export const supportedLanguages = ['zh-CN', 'en'];

const DEFAULT_LANGUAGE = 'zh-CN';

/**
 * 把任意 BCP-47 语言标记归一化为受支持的语言代码。
 * @param {string|null|undefined} lng
 * @returns {'zh-CN'|'en'|null} 受支持的代码；空输入返回 null（让调用方继续向下回退）。
 */
export const normalizeLanguage = (lng) => {
  if (!lng || typeof lng !== 'string') return null;
  const lower = lng.trim().toLowerCase();
  if (!lower) return null;
  if (lower.startsWith('zh')) return 'zh-CN';
  return 'en';
};

/**
 * 决定初始展示语言：已保存的用户偏好优先，其次按浏览器语言（非中文即英文），
 * 都没有时回退到默认语言。纯函数，便于测试。
 * @param {{stored?: string|null, languages?: string|string[]|null}} [opts]
 * @returns {'zh-CN'|'en'}
 */
export const pickLanguage = ({ stored, languages } = {}) => {
  const fromStored = normalizeLanguage(stored);
  if (fromStored) return fromStored;

  const list = Array.isArray(languages)
    ? languages
    : languages
      ? [languages]
      : [];
  for (const candidate of list) {
    const normalized = normalizeLanguage(candidate);
    if (normalized) return normalized;
  }
  return DEFAULT_LANGUAGE;
};

/**
 * 在浏览器环境下读取 localStorage 用户偏好与 navigator 语言，得到初始语言。
 * 薄封装：真正的决策逻辑在 pickLanguage 中（已被单测覆盖）。
 * @returns {'zh-CN'|'en'}
 */
export const detectInitialLanguage = () => {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') {
    return DEFAULT_LANGUAGE;
  }
  let stored = null;
  try {
    stored = window.localStorage.getItem('i18nextLng');
  } catch (e) {
    stored = null;
  }
  const languages =
    navigator.languages && navigator.languages.length
      ? navigator.languages
      : navigator.language
        ? [navigator.language]
        : [];
  return pickLanguage({ stored, languages });
};
