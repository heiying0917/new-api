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
import { Card, Row, Col, Typography } from '@douyinfe/semi-ui';
import { Wallet, FileClock, CheckCircle2 } from 'lucide-react';

const { Text } = Typography;
const cny = (v) =>
  `¥${(Number(v) || 0).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
const usd = (v) =>
  `$${(Number(v) || 0).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;

const StatCard = ({ icon: Icon, color, label, mainCny, subUsd, extra }) => (
  <Card
    className='!rounded-2xl w-full h-full'
    bordered={false}
    bodyStyle={{ padding: 16 }}
    style={{
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-1)',
    }}
  >
    <div className='flex items-start gap-3'>
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
      <div className='min-w-0'>
        <Text
          type='tertiary'
          style={{ fontSize: 11, fontWeight: 600, letterSpacing: '0.05em' }}
        >
          {label}
        </Text>
        <div
          style={{
            fontSize: 22,
            fontWeight: 700,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {cny(mainCny)}
        </div>
        <Text type='tertiary' style={{ fontSize: 12 }}>
          {usd(subUsd)} {extra}
        </Text>
      </div>
    </div>
  </Card>
);

const SuppliersSummaryBar = ({ summary, t }) => {
  const pending = summary?.pending || {};
  const applied = summary?.applied || {};
  const settled = summary?.settled || {};
  return (
    <Row gutter={[12, 12]} className='mb-3 w-full'>
      <Col xs={24} sm={8}>
        <StatCard
          icon={Wallet}
          color='#f59e0b'
          label={t('待结算总额')}
          mainCny={pending.payable_cny}
          subUsd={pending.official_usd}
          extra={`· ${pending.supplier_count || 0} ${t('家')}`}
        />
      </Col>
      <Col xs={24} sm={8}>
        <StatCard
          icon={FileClock}
          color='#3b82f6'
          label={t('已申请结算')}
          mainCny={applied.computed_cny}
          subUsd={applied.official_usd}
          extra={`· ${applied.count || 0} ${t('单')}`}
        />
      </Col>
      <Col xs={24} sm={8}>
        <StatCard
          icon={CheckCircle2}
          color='#10b981'
          label={t('已结算')}
          mainCny={settled.actual_cny}
          subUsd={settled.official_usd}
          extra={
            settled.actual_usd > 0
              ? `· ${t('另含')} $${Number(settled.actual_usd).toFixed(2)}`
              : `· ${settled.count || 0} ${t('单')}`
          }
        />
      </Col>
    </Row>
  );
};

export default SuppliersSummaryBar;
