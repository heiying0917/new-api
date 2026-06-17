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
import { SideSheet, Table, Typography, Empty, Button } from '@douyinfe/semi-ui';
import { useNavigate } from 'react-router-dom';

const { Title, Text } = Typography;

const TypeDetailSheet = ({ visible, stat, onClose, t }) => {
  const navigate = useNavigate();
  const groups = stat?.groups || [];
  const channels = Array.isArray(stat?.channels) ? stat.channels : [];

  // 点击供应商名 → 关闭抽屉 + 跳转渠道管理并按供应商名过滤（V12）。
  const goChannels = (name) => {
    onClose && onClose();
    navigate('/console/channel?supplier=' + encodeURIComponent(name || ''));
  };

  // 渠道明细列：供应商(可点)/分组/成本价(¥)/已跑金额($)，与卡片字段一致（V12）。
  const channelColumns = [
    {
      title: t('供应商'),
      dataIndex: 'supplier_name',
      render: (text) => (
        <Button
          theme='borderless'
          type='primary'
          size='small'
          className='!px-0'
          onClick={() => goChannels(text)}
        >
          {text}
        </Button>
      ),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (v) => v || '—',
    },
    {
      title: t('成本价'),
      dataIndex: 'cost_price',
      render: (v) => (v > 0 ? `¥${Number(v).toFixed(2)}` : '—'),
    },
    {
      title: t('已跑金额'),
      dataIndex: 'official_usd',
      render: (v) => `$${(Number(v) || 0).toFixed(2)}`,
    },
  ];

  const groupColumns = [
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
      width={620}
    >
      {stat ? (
        <div className='flex flex-col gap-4'>
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

          <div className='flex flex-col gap-2'>
            <Text strong style={{ fontSize: 12 }}>
              {t('渠道明细')}
            </Text>
            {channels.length > 0 ? (
              <Table
                columns={channelColumns}
                dataSource={channels}
                pagination={
                  channels.length > 10 ? { pageSize: 10 } : false
                }
                size='small'
                rowKey='channel_id'
              />
            ) : (
              <Empty description={t('暂无渠道')} />
            )}
          </div>

          <div className='flex flex-col gap-2'>
            <Text strong style={{ fontSize: 12 }}>
              {t('竞价分组最低价')}
            </Text>
            {groups.length > 0 ? (
              <Table
                columns={groupColumns}
                dataSource={groups}
                pagination={false}
                size='small'
                rowKey='group'
              />
            ) : (
              <Empty description={t('暂无分组报价')} />
            )}
          </div>
        </div>
      ) : null}
    </SideSheet>
  );
};

export default TypeDetailSheet;
