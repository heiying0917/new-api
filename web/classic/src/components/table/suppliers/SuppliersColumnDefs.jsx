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
import { Button, Space, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const formatCny = (value) => {
  const num = Number(value) || 0;
  return `¥${num.toFixed(2)}`;
};

/**
 * Render username as a clickable link to the supplier settlement review
 */
const renderUsername = (text, record, { onNavigateSettlement, t }) => {
  const remark = record.remark;
  const maxLen = 10;
  const displayRemark =
    remark && remark.length > maxLen ? remark.slice(0, maxLen) + '…' : remark;

  return (
    <Space spacing={2}>
      <Button
        theme='borderless'
        type='primary'
        size='small'
        className='!px-0'
        onClick={() => onNavigateSettlement(record)}
      >
        {text}
      </Button>
      {remark ? (
        <Tooltip content={remark} position='top' showArrow>
          <Tag color='white' shape='circle' className='!text-xs'>
            <div className='flex items-center gap-1'>
              <div
                className='w-2 h-2 flex-shrink-0 rounded-full'
                style={{ backgroundColor: '#10b981' }}
              />
              {displayRemark}
            </div>
          </Tag>
        </Tooltip>
      ) : null}
    </Space>
  );
};

/**
 * Render user status tag
 */
const renderStatus = (status, t) => {
  switch (status) {
    case 1:
      return (
        <Tag color='green' shape='circle'>
          {t('已激活')}
        </Tag>
      );
    case 2:
      return (
        <Tag color='red' shape='circle'>
          {t('已封禁')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {t('未知状态')}
        </Tag>
      );
  }
};

/**
 * Render enabled tag
 */
const renderEnabled = (enabled, t) => {
  return enabled ? (
    <Tag color='green' shape='circle'>
      {t('已启用')}
    </Tag>
  ) : (
    <Tag color='grey' shape='circle'>
      {t('已停用')}
    </Tag>
  );
};

/**
 * Render settlement mode tag
 */
const renderSettlementMode = (mode, t) => {
  if (mode === 'auto') {
    return (
      <Tag color='blue' shape='circle'>
        {t('自动结算')}
      </Tag>
    );
  }
  return (
    <Tag color='orange' shape='circle'>
      {t('手动结算')}
    </Tag>
  );
};

/**
 * Render operations column
 */
const renderOperations = (
  text,
  record,
  { setEditingSupplier, setShowEditSupplier, onInitiateSettlement, t },
) => {
  return (
    <Space>
      {onInitiateSettlement ? (
        <Button
          type='primary'
          theme='light'
          size='small'
          onClick={() => onInitiateSettlement(record)}
        >
          {t('立即结算')}
        </Button>
      ) : null}
      <Button
        type='tertiary'
        size='small'
        onClick={() => {
          setEditingSupplier(record);
          setShowEditSupplier(true);
        }}
      >
        {t('编辑')}
      </Button>
    </Space>
  );
};

/**
 * Get suppliers table column definitions
 */
export const getSuppliersColumns = ({
  t,
  onNavigateSettlement,
  setEditingSupplier,
  setShowEditSupplier,
  onInitiateSettlement,
}) => {
  return [
    {
      title: t('用户名'),
      dataIndex: 'username',
      render: (text, record) =>
        renderUsername(text, record, { onNavigateSettlement, t }),
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      render: (text) => <span>{text || '-'}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'user_status',
      render: (text) => renderStatus(text, t),
    },
    {
      title: t('优先级'),
      dataIndex: 'priority',
      sorter: true,
      render: (text) => <span>{text ?? 0}</span>,
    },
    {
      title: t('启用'),
      dataIndex: 'enabled',
      render: (text) => renderEnabled(text, t),
    },
    {
      title: t('待结算'),
      dataIndex: 'pending_cny',
      sorter: true,
      render: (text) => (
        <Text type='warning'>{formatCny(text)}</Text>
      ),
    },
    {
      title: t('已结算'),
      dataIndex: 'settled_cny',
      sorter: true,
      render: (text) => (
        <Text type='success'>{formatCny(text)}</Text>
      ),
    },
    {
      title: t('结算方式'),
      dataIndex: 'settlement_mode',
      render: (text) => renderSettlementMode(text, t),
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      width: 160,
      render: (text, record) =>
        renderOperations(text, record, {
          setEditingSupplier,
          setShowEditSupplier,
          onInitiateSettlement,
          t,
        }),
    },
  ];
};
