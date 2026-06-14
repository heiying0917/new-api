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

import React, { useMemo } from 'react';
import { Card, Empty } from '@douyinfe/semi-ui';
import { IllustrationNoResult, IllustrationNoResultDark } from '@douyinfe/semi-illustrations';
import { LineChart, BarChart3 } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import { renderNumber, timestamp2string } from '../../helpers';
import {
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
} from '../../constants/dashboard.constants';

// Format a unix-second day bucket to a short YYYY-MM-DD label.
const formatDay = (unixSec) => {
  const full = timestamp2string(unixSec); // 'YYYY-MM-DD HH:mm:ss'
  return typeof full === 'string' ? full.split(' ')[0] : String(unixSec);
};

const ChartEmpty = ({ t }) => (
  <div className='flex items-center justify-center h-full'>
    <Empty
      image={<IllustrationNoResult style={ILLUSTRATION_SIZE} />}
      darkModeImage={<IllustrationNoResultDark style={ILLUSTRATION_SIZE} />}
      description={t('暂无数据')}
    />
  </div>
);

const SupplierTrendCharts = ({ series, t }) => {
  const hasData = Array.isArray(series) && series.length > 0;

  // Long-format data for usage line chart: one row per (day, metric).
  const usageSpec = useMemo(() => {
    const values = [];
    (series || []).forEach((item) => {
      const day = formatDay(item.day);
      values.push({ Day: day, Metric: t('请求'), Value: item.requests || 0 });
      values.push({ Day: day, Metric: t('Token'), Value: item.tokens || 0 });
    });
    return {
      type: 'line',
      data: [{ id: 'usageData', values }],
      xField: 'Day',
      yField: 'Value',
      seriesField: 'Metric',
      legends: { visible: true, selectMode: 'single' },
      point: { visible: true },
      line: { style: { lineWidth: 2 } },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum) => datum['Metric'],
              value: (datum) => renderNumber(datum['Value']),
            },
          ],
        },
        dimension: {
          content: [
            {
              key: (datum) => datum['Metric'],
              value: (datum) => renderNumber(datum['Value']),
            },
          ],
        },
      },
    };
  }, [series, t]);

  // Bar chart for daily official USD earnings.
  const earningsSpec = useMemo(() => {
    const values = (series || []).map((item) => ({
      Day: formatDay(item.day),
      Amount: Number(item.official_usd || 0),
    }));
    return {
      type: 'bar',
      data: [{ id: 'earningsData', values }],
      xField: 'Day',
      yField: 'Amount',
      legends: { visible: false },
      bar: {
        state: { hover: { stroke: '#000', lineWidth: 1 } },
      },
      axes: [
        {
          orient: 'left',
          label: {
            formatMethod: (value) => `$${Number(value).toFixed(2)}`,
          },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: () => t('官方金额'),
              value: (datum) => `$${Number(datum['Amount'] || 0).toFixed(2)}`,
            },
          ],
        },
      },
    };
  }, [series, t]);

  return (
    <div className='grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4'>
      <Card
        {...CARD_PROPS}
        className='!rounded-2xl'
        title={
          <div className={FLEX_CENTER_GAP2}>
            <LineChart size={16} />
            {t('渠道用量趋势')}
          </div>
        }
        bodyStyle={{ padding: 0 }}
      >
        <div className='h-80 p-2'>
          {hasData ? (
            <VChart spec={usageSpec} option={CHART_CONFIG} />
          ) : (
            <ChartEmpty t={t} />
          )}
        </div>
      </Card>

      <Card
        {...CARD_PROPS}
        className='!rounded-2xl'
        title={
          <div className={FLEX_CENTER_GAP2}>
            <BarChart3 size={16} />
            {t('收益/结算趋势')}
          </div>
        }
        bodyStyle={{ padding: 0 }}
      >
        <div className='h-80 p-2'>
          {hasData ? (
            <VChart spec={earningsSpec} option={CHART_CONFIG} />
          ) : (
            <ChartEmpty t={t} />
          )}
        </div>
      </Card>
    </div>
  );
};

export default SupplierTrendCharts;
