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
import { CHANNEL_OPTIONS } from '../../../constants';

// Build a lookup map from channel type value -> { label, color }
const CHANNEL_TYPE_MAP = CHANNEL_OPTIONS.reduce((acc, opt) => {
  acc[opt.value] = opt;
  return acc;
}, {});

// Render channel type as a colored tag using the shared CHANNEL_OPTIONS list
const renderType = (type) => {
  const opt = CHANNEL_TYPE_MAP[type];
  if (!opt) {
    return (
      <Tag color='grey' shape='circle' size='small'>
        {type}
      </Tag>
    );
  }
  return (
    <Tag color={opt.color || 'grey'} shape='circle' size='small'>
      {opt.label}
    </Tag>
  );
};

// Render group string (comma separated) as a list of tags
const renderGroups = (group, t) => {
  if (!group) {
    return (
      <Tag color='white' shape='circle' size='small'>
        {t('默认')}
      </Tag>
    );
  }
  const groups = String(group)
    .split(',')
    .map((g) => g.trim())
    .filter(Boolean);
  return (
    <Space wrap>
      {groups.map((g) => (
        <Tag key={g} color='light-blue' shape='circle' size='small'>
          {g}
        </Tag>
      ))}
    </Space>
  );
};

// Render status as a colored tag
const renderStatus = (status, t) => {
  if (status === 1) {
    return (
      <Tag color='green' shape='circle' size='small'>
        {t('启用')}
      </Tag>
    );
  }
  return (
    <Tag color='red' shape='circle' size='small'>
      {t('禁用')}
    </Tag>
  );
};

const toNumber = (v) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

export const getSupplierChannelsColumns = ({ t, onEdit, onDelete }) => {
  return [
    {
      title: t('名称'),
      dataIndex: 'name',
    },
    {
      title: t('类型'),
      dataIndex: 'type',
      render: (text) => renderType(text),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (text) => renderGroups(text, t),
    },
    {
      title: t('成本价'),
      dataIndex: 'cost_price',
      render: (text) => <span>{`¥${toNumber(text)}`}</span>,
    },
    {
      title: t('官方计费'),
      dataIndex: 'official_usd',
      render: (text) => <span>{`$${toNumber(text).toFixed(4)}`}</span>,
    },
    {
      title: t('应收款'),
      dataIndex: 'receivable',
      render: (text) => <span>{`¥${toNumber(text).toFixed(2)}`}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (text) => renderStatus(text, t),
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
            onClick={() => onEdit(record)}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定删除此渠道？')}
            content={t('此修改将不可逆')}
            okType='danger'
            position='topRight'
            onConfirm={() => onDelete(record.id)}
          >
            <Button type='danger' size='small'>
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];
};
