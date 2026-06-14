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

import React, { useEffect, useMemo, useState } from 'react';
import {
  SideSheet,
  Space,
  Tag,
  Typography,
  Descriptions,
  Empty,
  Divider,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../../common/ui/CardTable';
import { timestamp2string } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { renderSettlementStatus } from '../SettlementReviewColumnDefs';

const { Text } = Typography;

const toNumber = (v) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

const currencySymbol = (currency) => (currency === 'USD' ? '$' : '¥');

const DetailModal = ({
  visible,
  record,
  onCancel,
  fetchBreakdown,
  fetchLogs,
  t,
}) => {
  const isMobile = useIsMobile();

  const [breakdown, setBreakdown] = useState([]);
  const [breakdownLoading, setBreakdownLoading] = useState(false);

  const [logs, setLogs] = useState([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsPage, setLogsPage] = useState(1);
  const [logsTotal, setLogsTotal] = useState(0);
  const logsPageSize = 10;

  const loadBreakdown = async (id) => {
    setBreakdownLoading(true);
    const data = await fetchBreakdown(id);
    setBreakdown(data || []);
    setBreakdownLoading(false);
  };

  const loadLogs = async (id, page) => {
    setLogsLoading(true);
    const data = await fetchLogs(id, page, logsPageSize);
    setLogs(data?.items || []);
    setLogsTotal(data?.total || 0);
    setLogsPage(data?.page || page);
    setLogsLoading(false);
  };

  useEffect(() => {
    if (visible && record?.id) {
      setLogsPage(1);
      loadBreakdown(record.id);
      loadLogs(record.id, 1);
    } else if (!visible) {
      setBreakdown([]);
      setLogs([]);
      setLogsTotal(0);
      setLogsPage(1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, record?.id]);

  const handleLogsPageChange = (page) => {
    if (record?.id) {
      loadLogs(record.id, page);
    }
  };

  const breakdownColumns = useMemo(
    () => [
      {
        title: t('渠道'),
        dataIndex: 'channel_name',
        render: (text, row) => <span>{text || `#${row.channel_id}`}</span>,
      },
      {
        title: t('调用次数'),
        dataIndex: 'requests',
        render: (text) => <span>{toNumber(text)}</span>,
      },
      {
        title: 'Token',
        dataIndex: 'tokens',
        render: (text) => <span>{toNumber(text)}</span>,
      },
      {
        title: t('官方金额'),
        dataIndex: 'official_usd',
        render: (text) => <span>{`$${toNumber(text).toFixed(4)}`}</span>,
      },
      {
        title: t('成本价'),
        dataIndex: 'cost_price',
        render: (text) => <span>{`¥${toNumber(text)}`}</span>,
      },
      {
        title: t('应收款'),
        dataIndex: 'receivable',
        render: (text) => <span>{`¥${toNumber(text).toFixed(2)}`}</span>,
      },
    ],
    [t],
  );

  const logsColumns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (text) => <span>{text ? timestamp2string(text) : '-'}</span>,
      },
      {
        title: t('渠道'),
        dataIndex: 'channel_name',
        render: (text, row) => <span>{text || `#${row.channel}`}</span>,
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (text) => <span>{text || '-'}</span>,
      },
      {
        title: t('提示'),
        dataIndex: 'prompt_tokens',
        render: (text) => <span>{toNumber(text)}</span>,
      },
      {
        title: t('补全'),
        dataIndex: 'completion_tokens',
        render: (text) => <span>{toNumber(text)}</span>,
      },
      {
        title: t('官方金额'),
        dataIndex: 'official_usd',
        render: (text) => <span>{`$${toNumber(text).toFixed(4)}`}</span>,
      },
    ],
    [t],
  );

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 980}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('结算详情')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {record?.supplier_name || `#${record?.supplier_id || '-'}`}
          </Typography.Title>
          <Text type='tertiary' className='ml-2'>
            ID: {record?.id || '-'}
          </Text>
        </Space>
      }
    >
      <div className='p-4'>
        {/* 概要信息 */}
        <Descriptions
          row
          size='small'
          data={
            record
              ? [
                  {
                    key: t('状态'),
                    value: renderSettlementStatus(record.status, t),
                  },
                  {
                    key: t('周期'),
                    value: `${
                      record.period_start
                        ? timestamp2string(record.period_start)
                        : '-'
                    } ~ ${
                      record.period_end
                        ? timestamp2string(record.period_end)
                        : '-'
                    }`,
                  },
                  {
                    key: t('官方金额'),
                    value: `$${toNumber(record.official_usd).toFixed(4)}`,
                  },
                  {
                    key: t('应结算金额'),
                    value: `¥${toNumber(record.computed_cny).toFixed(2)}`,
                  },
                  {
                    key: t('实付金额'),
                    value:
                      record.actual_amount === null ||
                      record.actual_amount === undefined ||
                      record.actual_amount === ''
                        ? '-'
                        : `${currencySymbol(
                            record.actual_currency,
                          )}${toNumber(record.actual_amount).toFixed(2)}`,
                  },
                  {
                    key: t('结算方式'),
                    value: record.settle_method || '-',
                  },
                  {
                    key: t('调用次数'),
                    value: toNumber(record.log_count),
                  },
                  {
                    key: t('结算时间'),
                    value: record.settled_at
                      ? timestamp2string(record.settled_at)
                      : '-',
                  },
                  {
                    key: t('备注'),
                    value: record.remark || '-',
                  },
                ]
              : []
          }
        />

        <Divider margin='12px' />

        {/* 按渠道明细 */}
        <Typography.Title heading={6} className='mb-2'>
          {t('按渠道明细')}
        </Typography.Title>
        <CardTable
          columns={breakdownColumns}
          dataSource={breakdown}
          rowKey={(row) => row.channel_id}
          loading={breakdownLoading}
          scroll={{ x: 'max-content' }}
          hidePagination={true}
          pagination={false}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 120, height: 120 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
              }
              description={t('暂无明细')}
              style={{ padding: 20 }}
            />
          }
          size='small'
        />

        <Divider margin='12px' />

        {/* 调用日志 */}
        <Typography.Title heading={6} className='mb-2'>
          {t('调用日志')}
        </Typography.Title>
        <CardTable
          columns={logsColumns}
          dataSource={logs}
          rowKey={(row) => row.id}
          loading={logsLoading}
          scroll={{ x: 'max-content' }}
          hidePagination={false}
          pagination={{
            currentPage: logsPage,
            pageSize: logsPageSize,
            total: logsTotal,
            showSizeChanger: false,
            onPageChange: handleLogsPageChange,
          }}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 120, height: 120 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
              }
              description={t('暂无日志')}
              style={{ padding: 20 }}
            />
          }
          size='small'
        />
      </div>
    </SideSheet>
  );
};

export default DetailModal;
