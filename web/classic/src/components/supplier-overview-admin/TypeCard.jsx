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
import { Card, Typography } from '@douyinfe/semi-ui';
import { useNavigate } from 'react-router-dom';

const { Text } = Typography;

// 卡片内最多展示的渠道条数；超出则折叠为「共 N 条」，全部在详情抽屉里看（应对渠道很多的场景）。
const CARD_CHANNEL_LIMIT = 5;

const TypeCard = ({ stat, onClick, t }) => {
  const navigate = useNavigate();
  const available = stat.available || 0;
  const unavailable = stat.unavailable || 0;
  const channels = Array.isArray(stat.channels) ? stat.channels : [];
  const topChannels = channels.slice(0, CARD_CHANNEL_LIMIT);
  const restCount = channels.length - topChannels.length;
  return (
    <Card
      className='!rounded-xl w-full h-full cursor-pointer transition-all duration-200 hover:-translate-y-0.5'
      bordered={false}
      bodyStyle={{ padding: 18 }}
      style={{
        border: '1px solid var(--semi-color-border)',
        background: 'var(--semi-color-bg-1)',
      }}
      onClick={() => onClick(stat)}
    >
      <div className='flex items-center justify-between gap-2 mb-2.5'>
        <Text strong className='truncate' style={{ fontSize: 16 }}>
          {stat.type_name}
        </Text>
        <span
          className='shrink-0'
          style={{
            width: 9,
            height: 9,
            borderRadius: 999,
            background:
              unavailable > 0
                ? 'var(--semi-color-warning)'
                : 'var(--semi-color-success)',
          }}
        />
      </div>
      <div className='flex items-baseline gap-1.5'>
        <span
          style={{
            fontSize: 26,
            fontWeight: 700,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {stat.supplier_count}
        </span>
        <Text type='tertiary' style={{ fontSize: 13 }}>
          {t('家供应')}
        </Text>
      </div>
      <div className='mt-1.5 flex items-center justify-between'>
        <Text type='tertiary' style={{ fontSize: 13 }}>
          {available}/{stat.channel_count} {t('可用')}
        </Text>
        <Text style={{ fontSize: 15, fontWeight: 700 }}>
          {stat.lowest_price > 0 ? `¥${stat.lowest_price.toFixed(2)}` : '—'}
        </Text>
      </div>
      {topChannels.length > 0 && (
        <div
          className='mt-3 pt-3'
          style={{ borderTop: '1px solid var(--semi-color-border)' }}
        >
          {/* 网格布局：供应商/分组/成本价/已跑金额 四列垂直对齐（分组不再紧贴供应商名） */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'minmax(0, 5.5rem) minmax(0, 1fr) auto auto',
              columnGap: 10,
              rowGap: 6,
              fontSize: 13,
              lineHeight: 1.5,
              alignItems: 'center',
              fontVariantNumeric: 'tabular-nums',
            }}
          >
            {topChannels.map((c) => {
              const firstGroup = (c.group || '').split(/[,，]/)[0].trim();
              return (
                <React.Fragment key={c.channel_id}>
                  <span
                    onClick={(e) => {
                      // 阻止冒泡到卡片 onClick；跳转渠道管理并按供应商过滤
                      e.stopPropagation();
                      navigate(
                        `/console/channel?supplier=${encodeURIComponent(c.supplier_name)}`,
                      );
                    }}
                    className='cursor-pointer truncate hover:underline'
                    title={`${t('查看该供应商渠道')}: ${c.supplier_name}`}
                    style={{
                      color: 'var(--semi-color-link)',
                      fontWeight: 500,
                      minWidth: 0,
                    }}
                  >
                    {c.supplier_name}
                  </span>
                  {firstGroup ? (
                    <span
                      onClick={(e) => {
                        // 阻止冒泡到卡片 onClick；跳转渠道管理并按分组过滤
                        e.stopPropagation();
                        navigate(
                          `/console/channel?group=${encodeURIComponent(firstGroup)}`,
                        );
                      }}
                      className='cursor-pointer truncate hover:underline'
                      title={`${t('查看该分组渠道')}: ${c.group}`}
                      style={{ color: 'var(--semi-color-link)', minWidth: 0 }}
                    >
                      {c.group}
                    </span>
                  ) : (
                    <span style={{ color: 'var(--semi-color-text-2)' }}>—</span>
                  )}
                  <span style={{ textAlign: 'right', fontWeight: 600 }}>
                    {c.cost_price > 0
                      ? `¥${Number(c.cost_price).toFixed(2)}`
                      : '—'}
                  </span>
                  <span
                    style={{
                      textAlign: 'right',
                      color: 'var(--semi-color-success)',
                    }}
                  >
                    ${(Number(c.official_usd) || 0).toFixed(2)}
                  </span>
                </React.Fragment>
              );
            })}
          </div>
          {restCount > 0 && (
            <Text
              className='hover:underline mt-1.5 inline-block'
              style={{ fontSize: 12.5, color: 'var(--semi-color-link)' }}
            >
              {t('共 ${n} 条，点击查看全部').replace('${n}', channels.length)}
            </Text>
          )}
        </div>
      )}
    </Card>
  );
};

export default TypeCard;
