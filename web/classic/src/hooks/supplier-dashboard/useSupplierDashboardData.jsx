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

import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';

// Range presets: today / last 7 days / last 30 days
export const RANGE_TODAY = 'today';
export const RANGE_7D = '7d';
export const RANGE_30D = '30d';

// Compute [start, end] unix seconds for a given range key.
const computeRange = (range) => {
  const now = Math.floor(Date.now() / 1000);
  const startOfToday = (() => {
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return Math.floor(d.getTime() / 1000);
  })();
  switch (range) {
    case RANGE_TODAY:
      return { start: startOfToday, end: now };
    case RANGE_30D:
      return { start: now - 86400 * 30, end: now };
    case RANGE_7D:
    default:
      return { start: now - 86400 * 7, end: now };
  }
};

const REALTIME_POLL_MS = 10000;

export const useSupplierDashboardData = () => {
  const { t } = useTranslation();

  const [range, setRange] = useState(RANGE_7D);
  const [loading, setLoading] = useState(true);
  const [series, setSeries] = useState([]);
  const [ranking, setRanking] = useState([]);

  // Realtime metrics
  const [realtime, setRealtime] = useState({ rpm: 0, tpm: 0 });
  const realtimeTimerRef = useRef(null);

  // Load dashboard (series + ranking) for the current range
  const loadDashboard = useCallback(
    async (nextRange = range) => {
      setLoading(true);
      try {
        const { start, end } = computeRange(nextRange);
        const res = await API.get(
          `/api/supplier/self/dashboard?start_timestamp=${start}&end_timestamp=${end}`,
        );
        const { success, message, data } = res.data;
        if (success) {
          setSeries(Array.isArray(data?.series) ? data.series : []);
          setRanking(Array.isArray(data?.ranking) ? data.ranking : []);
        } else {
          showError(message);
        }
      } catch (e) {
        showError(e.message);
      } finally {
        setLoading(false);
      }
    },
    [range],
  );

  // Load realtime RPM/TPM (no loading flag, silent on poll errors)
  const loadRealtime = useCallback(async () => {
    try {
      const res = await API.get('/api/supplier/self/realtime');
      const { success, data } = res.data;
      if (success && data) {
        setRealtime({ rpm: data.rpm || 0, tpm: data.tpm || 0 });
      }
    } catch (e) {
      // Silent: realtime polling should not spam errors.
    }
  }, []);

  // Change range and refetch
  const handleRangeChange = useCallback(
    (nextRange) => {
      setRange(nextRange);
      loadDashboard(nextRange);
    },
    [loadDashboard],
  );

  // Initial dashboard load
  useEffect(() => {
    loadDashboard(RANGE_7D);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Realtime polling lifecycle
  useEffect(() => {
    loadRealtime();
    realtimeTimerRef.current = setInterval(loadRealtime, REALTIME_POLL_MS);
    return () => {
      if (realtimeTimerRef.current) {
        clearInterval(realtimeTimerRef.current);
        realtimeTimerRef.current = null;
      }
    };
  }, [loadRealtime]);

  return {
    t,
    range,
    setRange: handleRangeChange,
    loading,
    series,
    ranking,
    realtime,
    refresh: () => loadDashboard(range),
  };
};

export default useSupplierDashboardData;
