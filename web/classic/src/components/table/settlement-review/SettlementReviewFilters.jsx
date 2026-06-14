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

import React from 'react';
import { Select, Tag, Input, Button } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

const SettlementReviewFilters = ({
  statusFilter,
  changeStatus,
  supplierId,
  clearSupplierFilter,
  keyword,
  setKeyword,
  searchByKeyword,
  resetFilters,
  loading,
  t,
}) => {
  const statusOptions = [
    { label: t('全部'), value: 0 },
    { label: t('已申请'), value: 1 },
    { label: t('已结算'), value: 2 },
    { label: t('已取消'), value: 3 },
  ];

  const hasSupplierFilter =
    supplierId !== '' &&
    supplierId !== null &&
    supplierId !== undefined &&
    Number(supplierId) > 0;

  return (
    <div className='flex flex-col md:flex-row items-center gap-2 w-full md:w-auto order-1 md:order-2'>
      {hasSupplierFilter && (
        <Tag
          color='blue'
          shape='circle'
          closable
          onClose={() => clearSupplierFilter()}
        >
          {`${t('供应商')}: #${supplierId}`}
        </Tag>
      )}
      <div className='w-full md:w-64'>
        <Input
          value={keyword}
          onChange={(value) => setKeyword(value)}
          onEnterPress={() => searchByKeyword()}
          prefix={<IconSearch />}
          placeholder={t('支持搜索供应商的用户名、邮箱')}
          showClear
          size='small'
          className='w-full'
        />
      </div>
      <div className='w-full md:w-48'>
        <Select
          value={statusFilter}
          optionList={statusOptions}
          onChange={(value) => changeStatus(value)}
          className='w-full'
          size='small'
          placeholder={t('选择状态')}
        />
      </div>
      <div className='flex gap-2 w-full md:w-auto'>
        <Button
          type='tertiary'
          onClick={() => searchByKeyword()}
          loading={loading}
          className='flex-1 md:flex-initial md:w-auto'
          size='small'
        >
          {t('查询')}
        </Button>
        <Button
          type='tertiary'
          onClick={() => resetFilters()}
          className='flex-1 md:flex-initial md:w-auto'
          size='small'
        >
          {t('重置')}
        </Button>
      </div>
    </div>
  );
};

export default SettlementReviewFilters;
