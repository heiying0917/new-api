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
import { SideSheet, Table, Typography, Empty } from '@douyinfe/semi-ui';

const { Title, Text } = Typography;

const TypeDetailSheet = ({ visible, stat, onClose, t }) => {
  const groups = stat?.groups || [];
  const columns = [
    { title: t('分组'), dataIndex: 'group' },
    {
      title: t('最低价'),
      dataIndex: 'lowest_price',
      render: (v) => (v > 0 ? `¥${Number(v).toFixed(2)}` : '—'),
    },
  ];
  return (
    <SideSheet
      title={
        <Title heading={5} className='!mb-0'>
          {stat?.type_name} · {t('供应明细')}
        </Title>
      }
      visible={visible}
      onCancel={onClose}
      width={420}
    >
      {stat ? (
        <div className='flex flex-col gap-3'>
          <div className='flex gap-4'>
            <Text type='tertiary'>
              {t('供应商')} <b>{stat.supplier_count}</b>
            </Text>
            <Text type='tertiary'>
              {t('渠道')} <b>{stat.channel_count}</b>
            </Text>
            <Text type='tertiary'>
              {t('可用')} <b>{stat.available}</b>
            </Text>
          </div>
          {groups.length > 0 ? (
            <Table
              columns={columns}
              dataSource={groups}
              pagination={false}
              size='small'
              rowKey='group'
            />
          ) : (
            <Empty description={t('暂无分组报价')} />
          )}
        </div>
      ) : null}
    </SideSheet>
  );
};

export default TypeDetailSheet;
