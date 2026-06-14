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

import React, { useEffect, useState, useCallback } from 'react';
import {
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Descriptions,
  Table,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCreditCard,
  IconHistogram,
  IconList,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { timestamp2string } from '../../../../helpers';

const { Text, Title } = Typography;

const toNumber = (v) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

const renderTime = (ts) => {
  if (!ts) return '-';
  return timestamp2string(ts);
};

const renderStatusTag = (status, t) => {
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

const LOGS_PAGE_SIZE = 10;

const SettlementDetailModal = (props) => {
  const {
    visible,
    settlementId,
    handleClose,
    getDetail,
    getBreakdown,
    getSettlementLogs,
  } = props;
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const [breakdown, setBreakdown] = useState([]);

  // Logs state
  const [logs, setLogs] = useState([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsTotal, setLogsTotal] = useState(0);
  const [logsPage, setLogsPage] = useState(1);

  const loadLogs = useCallback(
    async (id, page) => {
      setLogsLoading(true);
      try {
        const res = await getSettlementLogs(id, page, LOGS_PAGE_SIZE);
        setLogs(res.items || []);
        setLogsTotal(res.total || 0);
        setLogsPage(res.page || page);
      } finally {
        setLogsLoading(false);
      }
    },
    [getSettlementLogs],
  );

  useEffect(() => {
    let cancelled = false;
    const loadAll = async () => {
      if (!visible || !settlementId) return;
      setLoading(true);
      try {
        const [d, b] = await Promise.all([
          getDetail(settlementId),
          getBreakdown(settlementId),
        ]);
        if (cancelled) return;
        setDetail(d);
        setBreakdown(b || []);
      } finally {
        if (!cancelled) setLoading(false);
      }
      await loadLogs(settlementId, 1);
    };
    if (visible && settlementId) {
      loadAll();
    } else {
      // Reset on close
      setDetail(null);
      setBreakdown([]);
      setLogs([]);
      setLogsTotal(0);
      setLogsPage(1);
    }
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, settlementId]);

  const breakdownColumns = [
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (text, record) => text || `#${record.channel_id}`,
    },
    {
      title: t('调用次数'),
      dataIndex: 'requests',
      render: (text) => toNumber(text),
    },
    {
      title: t('Token'),
      dataIndex: 'tokens',
      render: (text) => toNumber(text),
    },
    {
      title: t('官方金额'),
      dataIndex: 'official_usd',
      render: (text) => `$${toNumber(text).toFixed(4)}`,
    },
    {
      title: t('成本价'),
      dataIndex: 'cost_price',
      render: (text) => `¥${toNumber(text)}`,
    },
    {
      title: t('应收款'),
      dataIndex: 'receivable',
      render: (text) => `¥${toNumber(text).toFixed(2)}`,
    },
  ];

  const logsColumns = [
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (text) => renderTime(text),
    },
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (text) => text || '-',
    },
    {
      title: t('模型'),
      dataIndex: 'model_name',
      render: (text) => text || '-',
    },
    {
      title: t('提示'),
      dataIndex: 'prompt_tokens',
      render: (text) => toNumber(text),
    },
    {
      title: t('补全'),
      dataIndex: 'completion_tokens',
      render: (text) => toNumber(text),
    },
    {
      title: t('官方金额'),
      dataIndex: 'official_usd',
      render: (text) => `$${toNumber(text).toFixed(4)}`,
    },
  ];

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('详情')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('结算详情')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 720}
      footer={
        <div className='flex justify-end bg-white'>
          <Button
            theme='light'
            className='!rounded-lg'
            type='primary'
            onClick={handleClose}
            icon={<IconClose />}
          >
            {t('关闭')}
          </Button>
        </div>
      }
      closeIcon={null}
      onCancel={handleClose}
    >
      <Spin spinning={loading}>
        <div className='p-2'>
          {/* 概要 */}
          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center mb-2'>
              <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                <IconCreditCard size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>{t('结算概要')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('本次结算的基本信息')}
                </div>
              </div>
            </div>
            <Descriptions
              row
              size='small'
              data={[
                {
                  key: t('状态'),
                  value: renderStatusTag(detail?.status, t),
                },
                {
                  key: t('周期'),
                  value: `${renderTime(detail?.period_start)} ~ ${renderTime(
                    detail?.period_end,
                  )}`,
                },
                {
                  key: t('官方金额'),
                  value: `$${toNumber(detail?.official_usd).toFixed(4)}`,
                },
                {
                  key: t('应结算金额'),
                  value: `¥${toNumber(detail?.computed_cny).toFixed(2)}`,
                },
                {
                  key: t('调用次数'),
                  value: toNumber(detail?.log_count),
                },
                {
                  key: t('结算时间'),
                  value: renderTime(detail?.settled_at),
                },
              ]}
            />
          </Card>

          {/* 按渠道明细 */}
          <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
            <div className='flex items-center mb-2'>
              <Avatar size='small' color='green' className='mr-2 shadow-md'>
                <IconHistogram size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>{t('按渠道明细')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('每个渠道的用量与应收款')}
                </div>
              </div>
            </div>
            <Table
              columns={breakdownColumns}
              dataSource={breakdown}
              rowKey={(record) => record.channel_id}
              pagination={false}
              size='small'
              empty={t('暂无渠道明细')}
            />
          </Card>

          {/* 日志 */}
          <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
            <div className='flex items-center mb-2'>
              <Avatar size='small' color='orange' className='mr-2 shadow-md'>
                <IconList size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>{t('日志')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('本次结算包含的调用日志')}
                </div>
              </div>
            </div>
            <Table
              columns={logsColumns}
              dataSource={logs}
              loading={logsLoading}
              rowKey={(record, index) => record.id ?? index}
              pagination={{
                currentPage: logsPage,
                pageSize: LOGS_PAGE_SIZE,
                total: logsTotal,
                onPageChange: (page) => loadLogs(settlementId, page),
              }}
              size='small'
              empty={t('暂无日志')}
            />
          </Card>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default SettlementDetailModal;
