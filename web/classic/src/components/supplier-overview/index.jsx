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

import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Typography, Empty, Skeleton, Tooltip } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  Wallet,
  CheckCircle2,
  Plug,
  Activity,
  Gavel,
  TrendingDown,
  Info,
} from 'lucide-react';
import { API, showError } from '../../helpers';
import { CHANNEL_OPTIONS } from '../../constants';

const { Title, Text } = Typography;

// 数字千分位格式化
const formatThousands = (num) => {
  const n = Number(num);
  if (!Number.isFinite(n)) return '0';
  return n.toLocaleString('en-US');
};

// 安全的 toFixed，并加上千分位
const fixed2 = (num) => {
  const n = Number(num);
  if (!Number.isFinite(n)) return '0.00';
  return n.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
};

// 渠道类型 value -> label 映射
const getTypeLabel = (type, fallback) => {
  const option = CHANNEL_OPTIONS.find((opt) => opt.value === type);
  return option?.label || fallback || String(type);
};

// 配色方案 —— 使用低透明度强调色，兼容明暗主题
const ACCENTS = {
  amber: {
    chipBg: 'rgba(245, 158, 11, 0.14)',
    chip: '#f59e0b',
    ring: 'rgba(245, 158, 11, 0.45)',
  },
  emerald: {
    chipBg: 'rgba(16, 185, 129, 0.14)',
    chip: '#10b981',
    ring: 'rgba(16, 185, 129, 0.45)',
  },
  blue: {
    chipBg: 'rgba(59, 130, 246, 0.14)',
    chip: '#3b82f6',
    ring: 'rgba(59, 130, 246, 0.45)',
  },
  violet: {
    chipBg: 'rgba(139, 92, 246, 0.14)',
    chip: '#8b5cf6',
    ring: 'rgba(139, 92, 246, 0.45)',
  },
};

// 顶部统计卡片
const StatCard = ({ accent, icon: Icon, label, value, children }) => {
  const a = ACCENTS[accent];
  return (
    <Card
      className='!rounded-2xl w-full h-full transition-all duration-200 hover:-translate-y-0.5'
      bordered={false}
      bodyStyle={{ padding: 18 }}
      style={{
        border: '1px solid var(--semi-color-border)',
        boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      <div className='flex items-start gap-3'>
        <div
          className='flex items-center justify-center shrink-0'
          style={{
            width: 42,
            height: 42,
            borderRadius: 12,
            background: a.chipBg,
            color: a.chip,
            boxShadow: `inset 0 0 0 1px ${a.ring}`,
          }}
        >
          <Icon size={20} strokeWidth={2.2} />
        </div>
        <div className='min-w-0 flex-1'>
          <Text
            type='tertiary'
            className='block'
            style={{
              fontSize: 11,
              fontWeight: 600,
              letterSpacing: '0.06em',
              textTransform: 'uppercase',
            }}
          >
            {label}
          </Text>
          <div
            className='mt-1 leading-none truncate'
            style={{
              fontSize: 26,
              fontWeight: 700,
              color: 'var(--semi-color-text-0)',
              fontVariantNumeric: 'tabular-nums',
            }}
          >
            {value}
          </div>
          <div className='mt-1.5'>{children}</div>
        </div>
      </div>
    </Card>
  );
};

// 单个渠道类型卡片（行=该类型下各分组的市场行情）
const BidCard = ({ bid }) => {
  const { t } = useTranslation();
  const typeLabel = getTypeLabel(bid.type, bid.type_name);
  const groups = Array.isArray(bid.groups) ? bid.groups : [];
  const prices = groups
    .map((g) => Number(g.lowest_price))
    .filter((p) => Number.isFinite(p) && p > 0);
  const minPrice = prices.length ? Math.min(...prices) : 0;
  const maxPrice = prices.length ? Math.max(...prices) : 0;

  // 价格越低，条越长（最低=100%）。区间退化时给满条。
  const barWidth = (price) => {
    const p = Number(price);
    if (!Number.isFinite(p) || p <= 0) return 12;
    if (maxPrice === minPrice) return 100;
    const ratio = (maxPrice - p) / (maxPrice - minPrice);
    return Math.max(12, Math.round(ratio * 100));
  };

  const footer = (
    <div
      className='inline-flex items-center gap-1.5 rounded-full'
      style={{
        padding: '4px 10px',
        background: 'var(--semi-color-primary-light-default)',
        color: 'var(--semi-color-primary)',
        fontSize: 12,
        fontWeight: 600,
      }}
    >
      {t('已上架')}
      <span style={{ fontVariantNumeric: 'tabular-nums' }}>
        {bid.my_count || 0}/{bid.total || 0} {t('分组')}
      </span>
    </div>
  );

  return (
    <Card
      className='!rounded-2xl w-full h-full transition-all duration-200 hover:-translate-y-0.5'
      bordered={false}
      bodyStyle={{ padding: 16 }}
      style={{
        border: '1px solid var(--semi-color-border)',
        boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      {/* 卡片头部：渠道类型 + 分组数 */}
      <div className='flex items-center justify-between gap-2 mb-3'>
        <div className='min-w-0'>
          <div
            className='truncate'
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: 'var(--semi-color-text-0)',
            }}
          >
            {typeLabel}
          </div>
        </div>
        <div
          className='shrink-0 rounded-full'
          style={{
            padding: '3px 9px',
            background: 'var(--semi-color-fill-0)',
            color: 'var(--semi-color-text-2)',
            fontSize: 11,
            fontWeight: 600,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {bid.total} {t('个分组')}
        </div>
      </div>

      {/* 分组行情（行=分组） */}
      <div className='flex flex-col gap-1'>
        {groups.length === 0 && (
          <Text type='tertiary' style={{ fontSize: 12, padding: '8px 0' }}>
            {t('暂无报价')}
          </Text>
        )}
        {groups.map((g, i) => {
          const mine = !!g.mine;
          const price = Number(g.lowest_price);
          const hasPrice = Number.isFinite(price) && price > 0;
          return (
            <div
              key={i}
              className='relative flex items-center gap-2.5 rounded-xl overflow-hidden'
              style={{
                padding: '7px 10px',
                background: mine
                  ? 'var(--semi-color-primary-light-default)'
                  : 'transparent',
                boxShadow: mine
                  ? 'inset 0 0 0 1px var(--semi-color-primary-light-active)'
                  : 'none',
              }}
            >
              {/* 相对价格条（背景层，价低条长） */}
              {hasPrice && (
                <div
                  className='absolute left-0 top-0 bottom-0 pointer-events-none'
                  style={{
                    width: `${barWidth(price)}%`,
                    background: mine
                      ? 'rgba(59, 130, 246, 0.10)'
                      : 'var(--semi-color-fill-0)',
                    opacity: mine ? 1 : 0.6,
                    borderRadius: 12,
                    transition: 'width 0.3s ease',
                  }}
                />
              )}
              <div className='relative flex items-center gap-2 flex-1 min-w-0'>
                <span
                  className='truncate'
                  title={g.group}
                  style={{
                    fontSize: 13,
                    fontWeight: mine ? 600 : 500,
                    color: 'var(--semi-color-text-0)',
                    minWidth: 0,
                  }}
                >
                  {g.group}
                </span>
                {mine && (
                  <span
                    className='shrink-0 rounded-md'
                    style={{
                      padding: '1px 7px',
                      background: 'var(--semi-color-primary)',
                      color: '#fff',
                      fontSize: 11,
                      fontWeight: 700,
                    }}
                  >
                    {t('你')}
                  </span>
                )}
                <span
                  className='ml-auto shrink-0'
                  style={{
                    fontSize: 15,
                    fontWeight: mine ? 700 : 600,
                    color: hasPrice
                      ? mine
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-text-0)'
                      : 'var(--semi-color-text-2)',
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  {hasPrice ? `¥${fixed2(price)}` : t('暂无报价')}
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {/* 卡片底部：排名状态 */}
      <div
        className='mt-3 pt-3'
        style={{ borderTop: '1px solid var(--semi-color-border)' }}
      >
        {footer}
      </div>
    </Card>
  );
};

// 加载骨架屏
const LoadingSkeleton = () => (
  <div className='w-full'>
    <Row gutter={[16, 16]} className='mb-6'>
      {[0, 1, 2, 3].map((i) => (
        <Col xs={24} sm={12} lg={6} key={i}>
          <Card
            className='!rounded-2xl w-full h-full'
            bordered={false}
            bodyStyle={{ padding: 18 }}
            style={{ border: '1px solid var(--semi-color-border)' }}
          >
            <div className='flex items-start gap-3'>
              <Skeleton.Image
                style={{ width: 42, height: 42, borderRadius: 12 }}
              />
              <div className='flex-1'>
                <Skeleton.Title style={{ width: '40%', marginBottom: 10 }} />
                <Skeleton.Title style={{ width: '70%', height: 22 }} />
              </div>
            </div>
          </Card>
        </Col>
      ))}
    </Row>
    <Row gutter={[16, 16]}>
      {[0, 1, 2].map((i) => (
        <Col xs={24} sm={12} lg={8} key={i}>
          <Card
            className='!rounded-2xl w-full h-full'
            bordered={false}
            bodyStyle={{ padding: 16 }}
            style={{ border: '1px solid var(--semi-color-border)' }}
          >
            <Skeleton.Title style={{ width: '50%', marginBottom: 16 }} />
            <Skeleton.Paragraph rows={4} />
          </Card>
        </Col>
      ))}
    </Row>
  </div>
);

const SupplierOverview = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState(null);

  const loadOverview = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/supplier/self/overview');
      const { success, message, data: payload } = res.data;
      if (success) {
        setData(payload);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOverview();
  }, []);

  if (loading) {
    return <LoadingSkeleton />;
  }

  const pending = data?.pending || {};
  const settled = data?.settled || {};
  const channels = data?.channels || {};
  const todayUsage = data?.today_usage || {};
  const bids = Array.isArray(data?.bids) ? data.bids : [];
  const hasUnavailable = Number(channels.unavailable) > 0;

  return (
    <div className='w-full'>
      {/* 顶部统计卡片 */}
      <Row gutter={[16, 16]} className='mb-6'>
        <Col xs={24} sm={12} lg={6}>
          <StatCard
            accent='amber'
            icon={Wallet}
            label={t('待结算')}
            value={`¥${fixed2(pending.payable_cny)}`}
          >
            <Tooltip
              position='bottom'
              content={
                <div style={{ lineHeight: 1.8, maxWidth: 260 }}>
                  <div>
                    <b>{t('应收')} (¥)</b>: 
                    {t('按成本价×用量折算的人民币，即你将收到的金额')}
                  </div>
                  <div>
                    <b>{t('官方价')} ($)</b>: {t('上游官方计费的美元金额')}
                  </div>
                  <div>
                    <b>{t('笔数')}</b>: {t('当前待结算的调用条数')}
                  </div>
                </div>
              }
            >
              <Text
                type='tertiary'
                style={{
                  fontSize: 12,
                  cursor: 'help',
                  borderBottom: '1px dashed var(--semi-color-border)',
                }}
              >
                ${fixed2(pending.official_usd)} · {pending.log_count || 0}{' '}
                {t('条')}
                <Info
                  size={11}
                  style={{
                    marginLeft: 4,
                    verticalAlign: 'middle',
                    opacity: 0.6,
                  }}
                />
              </Text>
            </Tooltip>
          </StatCard>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <StatCard
            accent='emerald'
            icon={CheckCircle2}
            label={t('已结算')}
            value={`¥${fixed2(settled.total)}`}
          >
            <Text type='tertiary' style={{ fontSize: 12 }}>
              {t('今日')} ¥{fixed2(settled.today)} · {t('近7天')} ¥
              {fixed2(settled.last7)}
            </Text>
          </StatCard>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <StatCard
            accent='blue'
            icon={Plug}
            label={t('渠道')}
            value={
              <>
                {channels.available || 0}{' '}
                <span
                  style={{
                    fontSize: 14,
                    fontWeight: 500,
                    color: 'var(--semi-color-text-2)',
                  }}
                >
                  {t('可用')}
                </span>
              </>
            }
          >
            <Text
              style={{
                fontSize: 12,
                color: hasUnavailable
                  ? 'var(--semi-color-danger)'
                  : 'var(--semi-color-text-2)',
                fontWeight: hasUnavailable ? 600 : 400,
              }}
            >
              {channels.unavailable || 0} {t('不可用')}
            </Text>
          </StatCard>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <StatCard
            accent='violet'
            icon={Activity}
            label={t('今日用量')}
            value={
              <>
                {formatThousands(todayUsage.requests)}{' '}
                <span
                  style={{
                    fontSize: 14,
                    fontWeight: 500,
                    color: 'var(--semi-color-text-2)',
                  }}
                >
                  {t('请求')}
                </span>
              </>
            }
          >
            <Text type='tertiary' style={{ fontSize: 12 }}>
              {formatThousands(todayUsage.tokens)} Token
            </Text>
          </StatCard>
        </Col>
      </Row>

      {/* 实时市场竞价 */}
      <div className='mb-4 w-full'>
        <div className='flex items-center gap-3 mb-4 w-full'>
          <div
            className='flex items-center justify-center shrink-0'
            style={{
              width: 38,
              height: 38,
              borderRadius: 11,
              background: 'var(--semi-color-primary-light-default)',
              color: 'var(--semi-color-primary)',
            }}
          >
            <Gavel size={19} strokeWidth={2.2} />
          </div>
          <div>
            <Title heading={5} className='!mb-0'>
              {t('实时市场竞价')}
            </Title>
            <div className='flex items-center gap-1 mt-0.5'>
              <TrendingDown
                size={13}
                strokeWidth={2.4}
                style={{ color: 'var(--semi-color-success)' }}
              />
              <Text type='tertiary' style={{ fontSize: 12 }}>
                {t('调低成本价可提升排名')}
              </Text>
            </div>
          </div>
        </div>

        {bids.length === 0 ? (
          <Card
            className='!rounded-2xl'
            bordered={false}
            style={{ border: '1px solid var(--semi-color-border)' }}
          >
            <Empty
              image={
                <Gavel
                  size={48}
                  strokeWidth={1.4}
                  style={{ color: 'var(--semi-color-text-3)' }}
                />
              }
              title={t('暂无报价')}
              description={t('目前还没有可参与的竞价分组')}
              style={{ padding: '48px 0' }}
            />
          </Card>
        ) : (
          <Row gutter={[16, 16]}>
            {bids.map((bid, groupIdx) => (
              <Col xs={24} sm={12} lg={8} key={groupIdx}>
                <BidCard bid={bid} />
              </Col>
            ))}
          </Row>
        )}
      </div>
    </div>
  );
};

export default SupplierOverview;
