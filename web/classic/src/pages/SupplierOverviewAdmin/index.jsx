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

import React, { useState } from 'react';
import { Row, Col, Card, Typography, Empty, Spin, Button } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { Users, Plug } from 'lucide-react';
import { useSupplierOverviewData } from '../../hooks/supplier-overview-admin/useSupplierOverviewData';
import TypeCard from '../../components/supplier-overview-admin/TypeCard';
import TypeDetailSheet from '../../components/supplier-overview-admin/TypeDetailSheet';

const { Title, Text } = Typography;

const SummaryCard = ({ icon: Icon, color, label, value, sub }) => (
  <Card
    className='!rounded-2xl w-full h-full'
    bordered={false}
    bodyStyle={{ padding: 16 }}
    style={{
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-1)',
    }}
  >
    <div className='flex items-center gap-3'>
      <div
        className='flex items-center justify-center shrink-0'
        style={{
          width: 40,
          height: 40,
          borderRadius: 11,
          background: `${color}22`,
          color,
        }}
      >
        <Icon size={20} strokeWidth={2.2} />
      </div>
      <div>
        <Text type='tertiary' style={{ fontSize: 11, fontWeight: 600 }}>
          {label}
        </Text>
        <div style={{ fontSize: 22, fontWeight: 700 }}>{value}</div>
        {sub ? (
          <Text type='tertiary' style={{ fontSize: 12 }}>
            {sub}
          </Text>
        ) : null}
      </div>
    </div>
  </Card>
);

const SupplierOverviewAdmin = () => {
  const { t, loading, data, refresh } = useSupplierOverviewData();
  const [detail, setDetail] = useState(null);
  const summary = data?.summary || {};
  const byType = Array.isArray(data?.by_type) ? data.by_type : [];

  return (
    <div className='classic-page-fill px-4 md:px-6 pb-6 mt-[60px]'>
      <div className='flex items-center justify-between mb-4'>
        <Title heading={4} className='!mb-0'>
          {t('供应商概览')}
        </Title>
        <Button
          icon={<IconRefresh />}
          theme='light'
          type='tertiary'
          onClick={refresh}
          loading={loading}
        >
          {t('刷新')}
        </Button>
      </div>

      <Row gutter={[12, 12]} className='mb-4'>
        <Col xs={24} sm={12}>
          <SummaryCard
            icon={Users}
            color='#3b82f6'
            label={t('供应商')}
            value={summary.supplier_total || 0}
            sub={`${summary.supplier_enabled || 0} ${t('启用')}`}
          />
        </Col>
        <Col xs={24} sm={12}>
          <SummaryCard
            icon={Plug}
            color='#10b981'
            label={t('供应商渠道')}
            value={summary.channel_total || 0}
            sub={`${summary.channel_available || 0} ${t('可用')} · ${summary.channel_unavailable || 0} ${t('不可用')}`}
          />
        </Col>
      </Row>

      <Spin spinning={loading}>
        {byType.length === 0 ? (
          <Card
            className='!rounded-2xl'
            bordered={false}
            style={{ border: '1px solid var(--semi-color-border)' }}
          >
            <Empty
              description={t('暂无供应商渠道')}
              style={{ padding: '48px 0' }}
            />
          </Card>
        ) : (
          <Row gutter={[16, 16]}>
            {byType.map((stat) => (
              <Col key={stat.type} xs={24} sm={12} lg={8}>
                <TypeCard stat={stat} onClick={setDetail} t={t} />
              </Col>
            ))}
          </Row>
        )}
      </Spin>

      <TypeDetailSheet
        visible={!!detail}
        stat={detail}
        onClose={() => setDetail(null)}
        t={t}
      />
    </div>
  );
};

export default SupplierOverviewAdmin;
