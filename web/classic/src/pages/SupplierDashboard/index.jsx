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

import React, { useEffect } from 'react';
import { Button, ButtonGroup, Spin } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';

import {
  useSupplierDashboardData,
  RANGE_TODAY,
  RANGE_7D,
  RANGE_30D,
} from '../../hooks/supplier-dashboard/useSupplierDashboardData';
import RealtimeCards from '../../components/supplier-dashboard/RealtimeCards';
import SupplierTrendCharts from '../../components/supplier-dashboard/SupplierTrendCharts';
import SupplierRankingTable from '../../components/supplier-dashboard/SupplierRankingTable';

const SupplierDashboard = () => {
  const {
    t,
    range,
    setRange,
    loading,
    series,
    ranking,
    realtime,
    refresh,
  } = useSupplierDashboardData();

  // Initialize VChart Semi theme (mirrors useDashboardCharts).
  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const rangeOptions = [
    { value: RANGE_TODAY, label: t('今天') },
    { value: RANGE_7D, label: t('近7天') },
    { value: RANGE_30D, label: t('近30天') },
  ];

  return (
    <div className='classic-page-fill px-4 md:px-6 pb-6'>
      {/* Header: title + range selector + refresh */}
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-3 mb-4'>
        <h2 className='text-xl font-semibold m-0'>{t('供应商数据看板')}</h2>
        <div className='flex items-center gap-2'>
          <ButtonGroup>
            {rangeOptions.map((opt) => (
              <Button
                key={opt.value}
                theme={range === opt.value ? 'solid' : 'light'}
                type={range === opt.value ? 'primary' : 'tertiary'}
                onClick={() => setRange(opt.value)}
              >
                {opt.label}
              </Button>
            ))}
          </ButtonGroup>
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
      </div>

      {/* Realtime RPM / TPM */}
      <RealtimeCards realtime={realtime} t={t} />

      {/* Trend charts */}
      <Spin spinning={loading}>
        <SupplierTrendCharts series={series} t={t} />

        {/* Channel ranking */}
        <SupplierRankingTable ranking={ranking} loading={loading} t={t} />
      </Spin>
    </div>
  );
};

export default SupplierDashboard;
