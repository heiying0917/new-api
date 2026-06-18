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

import React, { useMemo } from 'react';
import { Button, Dropdown } from '@douyinfe/semi-ui';
import { Languages } from 'lucide-react';

// 语言名按惯例以各自母语原名展示，不随界面语言翻译。
const languageOptions = [
  { key: 'zh-CN', label: '简体中文' },
  { key: 'en', label: 'English' },
];

const LanguageSelector = ({ currentLang, onLanguageChange, t }) => {
  const getItemClassName = (isSelected) =>
    isSelected
      ? '!bg-semi-color-primary-light-default !font-semibold'
      : 'hover:!bg-semi-color-fill-1';

  const activeLabel = useMemo(() => {
    const found = languageOptions.find((option) => option.key === currentLang);
    return found ? found.label : languageOptions[0].label;
  }, [currentLang]);

  return (
    <Dropdown
      position='bottomRight'
      render={
        <Dropdown.Menu>
          {languageOptions.map((option) => (
            <Dropdown.Item
              key={option.key}
              onClick={() => onLanguageChange(option.key)}
              className={getItemClassName(currentLang === option.key)}
            >
              {option.label}
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      }
    >
      <span className='inline-flex'>
        <Button
          icon={<Languages size={18} />}
          aria-label={`${t('切换语言')}（${activeLabel}）`}
          theme='borderless'
          type='tertiary'
          className='!p-1.5 !text-current focus:!bg-semi-color-fill-1 !rounded-full !bg-semi-color-fill-0 hover:!bg-semi-color-fill-1'
        />
      </span>
    </Dropdown>
  );
};

export default LanguageSelector;
