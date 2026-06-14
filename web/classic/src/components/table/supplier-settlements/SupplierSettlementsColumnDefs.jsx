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
import { Button, Space, Tag, Popconfirm } from '@douyinfe/semi-ui';
import { timestamp2string } from '../../../helpers';

const toNumber = (v) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

// Render a unix-second timestamp as a date-time string, or '-' if empty
const renderTime = (ts) => {
  if (!ts) return '-';
  return timestamp2string(ts);
};

// Render the settlement status as a colored tag
// 1 = 已申请/待审核 (amber), 2 = 已结算 (green), 3 = 已取消 (grey)
const renderStatus = (status, t) => {
  if (status === 1) {
    return (
      <Tag color='amber' shape='circle' size='small'>
        {t('已申请')}
      </Tag>
    );
  }
  if (status === 2) {
    return (
      <Tag color='green' shape='circle' size='small'>
        {t('已结算')}
      </Tag>
    );
  }
  if (status === 3) {
    return (
      <Tag color='grey' shape='circle' size='small'>
        {t('已取消')}
      </Tag>
    );
  }
  return (
    <Tag color='grey' shape='circle' size='small'>
      {status}
    </Tag>
  );
};

// Render the billing period as "start ~ end" (dates only)
const renderPeriod = (record) => {
  const start = record.period_start ? timestamp2string(record.period_start) : '-';
  const end = record.period_end ? timestamp2string(record.period_end) : '-';
  return (
    <span>
      {start} ~ {end}
    </span>
  );
};

export const getSupplierSettlementsColumns = ({
  t,
  onDetail,
  onCancel,
  onExport,
}) => {
  return [
    {
      title: t('ID'),
      dataIndex: 'id',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (text) => renderStatus(text, t),
    },
    {
      title: t('周期'),
      dataIndex: 'period',
      render: (text, record) => renderPeriod(record),
    },
    {
      title: t('官方金额'),
      dataIndex: 'official_usd',
      render: (text) => <span>{`$${toNumber(text).toFixed(4)}`}</span>,
    },
    {
      title: t('应结算金额'),
      dataIndex: 'computed_cny',
      render: (text) => <span>{`¥${toNumber(text).toFixed(2)}`}</span>,
    },
    {
      title: t('调用次数'),
      dataIndex: 'log_count',
      render: (text) => <span>{toNumber(text)}</span>,
    },
    {
      title: t('申请时间'),
      dataIndex: 'created_at',
      render: (text) => renderTime(text),
    },
    {
      title: t('结算时间'),
      dataIndex: 'settled_at',
      render: (text) => renderTime(text),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      render: (text, record) => (
        <Space>
          <Button
            type='tertiary'
            size='small'
            onClick={() => onDetail(record)}
          >
            {t('详情')}
          </Button>
          {record.status === 1 && (
            <Popconfirm
              title={t('确定取消此结算申请？')}
              content={t('取消后该笔用量将恢复为待结算')}
              okType='danger'
              position='topRight'
              onConfirm={() => onCancel(record.id)}
            >
              <Button type='danger' size='small'>
                {t('取消')}
              </Button>
            </Popconfirm>
          )}
          <Button
            type='tertiary'
            size='small'
            onClick={() => onExport(record.id)}
          >
            {t('导出')}
          </Button>
        </Space>
      ),
    },
  ];
};
