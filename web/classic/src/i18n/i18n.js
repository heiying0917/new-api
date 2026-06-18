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

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

import zhCNTranslation from './locales/zh-CN.json';
import enTranslation from './locales/en.json';
import { detectInitialLanguage, normalizeLanguage } from './language';

// 本项目支持简体中文（zh-CN）与英文（en）。
// 初始语言：已保存偏好优先，否则按浏览器语言（非中文即英文）。
// fallbackLng=zh-CN：英文缺失的 key 回退到中文，绝不暴露原始 key。
i18n.use(initReactI18next).init({
  load: 'currentOnly',
  lng: detectInitialLanguage(),
  supportedLngs: ['zh-CN', 'en'],
  resources: {
    'zh-CN': zhCNTranslation,
    en: enTranslation,
  },
  fallbackLng: 'zh-CN',
  nsSeparator: false,
  interpolation: {
    escapeValue: false,
  },
});

window.__i18n = i18n;

// 同步 <html lang>，供按语言差异化样式（如英文侧栏更宽以容纳更长的菜单文案）与无障碍。
const syncHtmlLang = (lng) => {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = normalizeLanguage(lng) || 'zh-CN';
  }
};
syncHtmlLang(i18n.language);
i18n.on('languageChanged', syncHtmlLang);

export default i18n;
