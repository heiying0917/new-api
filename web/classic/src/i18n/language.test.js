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

import { describe, test, expect } from 'bun:test';
import {
  normalizeLanguage,
  pickLanguage,
  supportedLanguages,
} from './language';

describe('normalizeLanguage', () => {
  test('any Chinese variant collapses to zh-CN', () => {
    expect(normalizeLanguage('zh')).toBe('zh-CN');
    expect(normalizeLanguage('zh-CN')).toBe('zh-CN');
    expect(normalizeLanguage('zh-TW')).toBe('zh-CN');
    expect(normalizeLanguage('zh-HK')).toBe('zh-CN');
    expect(normalizeLanguage('zh-Hans-CN')).toBe('zh-CN');
    expect(normalizeLanguage('ZH-cn')).toBe('zh-CN');
  });

  test('any non-Chinese language maps to en', () => {
    expect(normalizeLanguage('en')).toBe('en');
    expect(normalizeLanguage('en-US')).toBe('en');
    expect(normalizeLanguage('en-GB')).toBe('en');
    expect(normalizeLanguage('fr')).toBe('en');
    expect(normalizeLanguage('ja')).toBe('en');
    expect(normalizeLanguage('ru-RU')).toBe('en');
    expect(normalizeLanguage('vi')).toBe('en');
  });

  test('empty / nullish input returns null so callers can fall through', () => {
    expect(normalizeLanguage('')).toBeNull();
    expect(normalizeLanguage(undefined)).toBeNull();
    expect(normalizeLanguage(null)).toBeNull();
  });
});

describe('pickLanguage', () => {
  test('a valid stored preference wins over the browser', () => {
    expect(pickLanguage({ stored: 'en', languages: ['zh-CN'] })).toBe('en');
    expect(pickLanguage({ stored: 'zh-TW', languages: ['en-US'] })).toBe(
      'zh-CN',
    );
  });

  test('no stored preference: non-Chinese browser shows English', () => {
    expect(pickLanguage({ stored: '', languages: ['fr-FR', 'de'] })).toBe('en');
    expect(pickLanguage({ stored: null, languages: ['en-US'] })).toBe('en');
    expect(pickLanguage({ stored: null, languages: ['ja'] })).toBe('en');
  });

  test('no stored preference: Chinese browser shows Chinese', () => {
    expect(pickLanguage({ stored: '', languages: ['zh-CN'] })).toBe('zh-CN');
    expect(pickLanguage({ stored: null, languages: ['zh-Hans', 'en'] })).toBe(
      'zh-CN',
    );
  });

  test('accepts a single language string as well as an array', () => {
    expect(pickLanguage({ stored: '', languages: 'fr' })).toBe('en');
    expect(pickLanguage({ stored: '', languages: 'zh-CN' })).toBe('zh-CN');
  });

  test('no signal at all defaults to zh-CN', () => {
    expect(pickLanguage({ stored: '', languages: [] })).toBe('zh-CN');
    expect(pickLanguage({})).toBe('zh-CN');
    expect(pickLanguage()).toBe('zh-CN');
  });
});

describe('supportedLanguages', () => {
  test('exposes exactly Chinese and English', () => {
    expect(supportedLanguages).toEqual(['zh-CN', 'en']);
  });
});
