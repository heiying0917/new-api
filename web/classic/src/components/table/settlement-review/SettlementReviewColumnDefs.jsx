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

// Status: 1 = 已申请/待审核, 2 = 已结算, 3 = 已取消
export const renderSettlementStatus = (status, t) => {
  switch (status) {
    case 1:
      return (
        <Tag color='orange' shape='circle' size='small'>
          {t('已申请')}
        </Tag>
      );
    case 2:
      return (
        <Tag color='green' shape='circle' size='small'>
          {t('已结算')}
        </Tag>
      );
    case 3:
      return (
        <Tag color='grey' shape='circle' size='small'>
          {t('已取消')}
        </Tag>
      );
    default:
      return (
        <Tag color='red' shape='circle' size='small'>
          {t('未知状态')}
        </Tag>
      );
  }
};

// Render a settlement period (start ~ end)
const renderPeriod = (record) => {
  const start = record.period_start
    ? timestamp2string(record.period_start)
    : '-';
  const end = record.period_end ? timestamp2string(record.period_end) : '-';
  return (
    <div className='text-xs text-gray-600'>
      <div>{start}</div>
      <div>~ {end}</div>
    </div>
  );
};

// Render currency symbol prefix for actual amount
const currencySymbol = (currency) => {
  if (currency === 'USD') return '$';
  return '¥';
};

export const getSettlementReviewColumns = ({
  t,
  onDetail,
  onConfirm,
  onCancel,
  onExport,
}) => {
  return [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 70,
    },
    {
      title: t('供应商'),
      dataIndex: 'supplier_name',
      render: (text, record) => <span>{text || `#${record.supplier_id}`}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (text) => renderSettlementStatus(text, t),
    },
    {
      title: t('周期'),
      dataIndex: 'period_start',
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
      title: t('实付金额'),
      dataIndex: 'actual_amount',
      render: (text, record) => {
        if (text === null || text === undefined || text === '') {
          return <span>-</span>;
        }
        return (
          <span>{`${currencySymbol(record.actual_currency)}${toNumber(
            text,
          ).toFixed(2)}`}</span>
        );
      },
    },
    {
      title: t('调用次数'),
      dataIndex: 'log_count',
      render: (text) => <span>{toNumber(text)}</span>,
    },
    {
      title: t('结算时间'),
      dataIndex: 'settled_at',
      render: (text) => <span>{text ? timestamp2string(text) : '-'}</span>,
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      fixed: 'right',
      width: 260,
      render: (text, record) => (
        <Space>
          <Button type='tertiary' size='small' onClick={() => onDetail(record)}>
            {t('详情')}
          </Button>
          {record.status === 1 && (
            <>
              <Button
                type='primary'
                theme='light'
                size='small'
                onClick={() => onConfirm(record)}
              >
                {t('确认结算')}
              </Button>
              <Popconfirm
                title={t('确定取消此结算？')}
                content={t('取消后该结算将作废')}
                okType='danger'
                position='topRight'
                onConfirm={() => onCancel(record.id)}
              >
                <Button type='danger' theme='light' size='small'>
                  {t('取消')}
                </Button>
              </Popconfirm>
            </>
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
