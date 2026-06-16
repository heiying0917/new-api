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
import { Card, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const TypeCard = ({ stat, onClick, t }) => {
  const available = stat.available || 0;
  const unavailable = stat.unavailable || 0;
  return (
    <Card
      className='!rounded-xl w-full h-full cursor-pointer transition-all duration-200 hover:-translate-y-0.5'
      bordered={false}
      bodyStyle={{ padding: 14 }}
      style={{
        border: '1px solid var(--semi-color-border)',
        background: 'var(--semi-color-bg-1)',
      }}
      onClick={() => onClick(stat)}
    >
      <div className='flex items-center justify-between gap-2 mb-2'>
        <Text strong className='truncate' style={{ fontSize: 13 }}>
          {stat.type_name}
        </Text>
        <span
          className='shrink-0'
          style={{
            width: 8,
            height: 8,
            borderRadius: 999,
            background:
              unavailable > 0
                ? 'var(--semi-color-warning)'
                : 'var(--semi-color-success)',
          }}
        />
      </div>
      <div className='flex items-baseline gap-1'>
        <span
          style={{
            fontSize: 20,
            fontWeight: 700,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {stat.supplier_count}
        </span>
        <Text type='tertiary' style={{ fontSize: 11 }}>
          {t('家供应')}
        </Text>
      </div>
      <div className='mt-1 flex items-center justify-between'>
        <Text type='tertiary' style={{ fontSize: 11 }}>
          {available}/{stat.channel_count} {t('可用')}
        </Text>
        <Text style={{ fontSize: 12, fontWeight: 600 }}>
          {stat.lowest_price > 0 ? `¥${stat.lowest_price.toFixed(2)}` : '—'}
        </Text>
      </div>
    </Card>
  );
};

export default TypeCard;
