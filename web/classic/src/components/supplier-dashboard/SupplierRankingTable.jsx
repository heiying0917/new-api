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
import { Card, Table } from '@douyinfe/semi-ui';
import { Trophy } from 'lucide-react';
import { renderNumber } from '../../helpers';
import {
  CARD_PROPS,
  FLEX_CENTER_GAP2,
} from '../../constants/dashboard.constants';

const SupplierRankingTable = ({ ranking, loading, t }) => {
  const columns = [
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (text, record) => text || `#${record.channel_id}`,
    },
    {
      title: t('请求'),
      dataIndex: 'requests',
      render: (v) => renderNumber(v || 0),
    },
    {
      title: t('Token'),
      dataIndex: 'tokens',
      render: (v) => renderNumber(v || 0),
    },
    {
      title: t('官方金额'),
      dataIndex: 'official_usd',
      render: (v) => `$${Number(v || 0).toFixed(2)}`,
    },
  ];

  return (
    <Card
      {...CARD_PROPS}
      className='!rounded-2xl'
      title={
        <div className={FLEX_CENTER_GAP2}>
          <Trophy size={16} />
          {t('渠道排行')}
        </div>
      }
    >
      <Table
        columns={columns}
        dataSource={ranking || []}
        loading={loading}
        rowKey={(record) => record.channel_id}
        pagination={false}
        empty={t('暂无数据')}
        size='small'
      />
    </Card>
  );
};

export default SupplierRankingTable;
