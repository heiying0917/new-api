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

import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';

export const useSupplierSettlementsData = () => {
  const { t } = useTranslation();

  // List state
  const [settlements, setSettlements] = useState([]);
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);
  // 当前待结算汇总 {payable_cny, official_usd, log_count}；顶部摘要卡片展示
  const [pending, setPending] = useState(null);

  // Filters: status (0=全部 / 2=已完成 / 3=已取消) + 申请时间段(unix 秒, 0=不限)
  const [statusFilter, setStatusFilter] = useState(0);
  const [startTs, setStartTs] = useState(0);
  const [endTs, setEndTs] = useState(0);

  // Load settlements list
  const loadSettlements = useCallback(
    async (page = 1, size = pageSize, filters = {}) => {
      const s = filters.status !== undefined ? filters.status : statusFilter;
      const st = filters.startTs !== undefined ? filters.startTs : startTs;
      const et = filters.endTs !== undefined ? filters.endTs : endTs;
      setLoading(true);
      try {
        let url = `/api/supplier/self/settlement/?p=${page}&page_size=${size}`;
        if (s) url += `&status=${s}`;
        if (st) url += `&start_timestamp=${st}`;
        if (et) url += `&end_timestamp=${et}`;
        const res = await API.get(url);
        const { success, message, data } = res.data;
        if (success) {
          setSettlements(data?.items || []);
          setTotal(data?.total || 0);
          setActivePage(data?.page || page);
          setPageSize(data?.page_size || size);
        } else {
          showError(message);
        }
      } catch (e) {
        showError(e.message);
      } finally {
        setLoading(false);
      }
    },
    [pageSize, statusFilter, startTs, endTs],
  );

  // Refresh current page
  const refresh = useCallback(
    async (page = activePage) => {
      await loadSettlements(page, pageSize);
    },
    [activePage, pageSize, loadSettlements],
  );

  // Get current pending (unsettled) amount — caches into state for the top summary card.
  const getPendingAmount = useCallback(async () => {
    try {
      const res = await API.get('/api/supplier/self/pending');
      const { success, message, data } = res.data;
      if (success) {
        setPending(data || null);
        return data || null;
      }
      showError(message);
      return null;
    } catch (e) {
      showError(e.message);
      return null;
    }
  }, []);

  // Apply for a new settlement (snapshot current unsettled usage)
  const applySettlement = useCallback(async () => {
    setApplying(true);
    try {
      const res = await API.post('/api/supplier/self/settlement/');
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('结算申请成功！'));
        await loadSettlements(1, pageSize);
        await getPendingAmount();
        return true;
      }
      showError(message);
      return false;
    } catch (e) {
      showError(e.message);
      return false;
    } finally {
      setApplying(false);
    }
  }, [loadSettlements, pageSize, t, getPendingAmount]);

  // Cancel a settlement (restore usage back to unsettled)
  const cancelSettlement = useCallback(
    async (id) => {
      try {
        const res = await API.post(`/api/supplier/self/settlement/${id}/cancel`);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('结算已取消！'));
          await refresh();
          await getPendingAmount();
          return true;
        }
        showError(message);
        return false;
      } catch (e) {
        showError(e.message);
        return false;
      }
    },
    [refresh, t, getPendingAmount],
  );

  // Get a single settlement detail
  const getDetail = useCallback(async (id) => {
    try {
      const res = await API.get(`/api/supplier/self/settlement/${id}`);
      const { success, message, data } = res.data;
      if (success) {
        return data;
      }
      showError(message);
      return null;
    } catch (e) {
      showError(e.message);
      return null;
    }
  }, []);

  // Get per-channel breakdown for a settlement
  const getBreakdown = useCallback(async (id) => {
    try {
      const res = await API.get(
        `/api/supplier/self/settlement/${id}/breakdown`,
      );
      const { success, message, data } = res.data;
      if (success) {
        return data || [];
      }
      showError(message);
      return [];
    } catch (e) {
      showError(e.message);
      return [];
    }
  }, []);

  // Get paginated logs inside a settlement
  const getSettlementLogs = useCallback(async (id, page = 1, size = 10) => {
    try {
      const res = await API.get(
        `/api/supplier/self/settlement/${id}/logs?p=${page}&page_size=${size}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        return {
          items: data?.items || [],
          total: data?.total || 0,
          page: data?.page || page,
          pageSize: data?.page_size || size,
        };
      }
      showError(message);
      return { items: [], total: 0, page, pageSize: size };
    } catch (e) {
      showError(e.message);
      return { items: [], total: 0, page, pageSize: size };
    }
  }, []);

  // Export a settlement as CSV (authenticated blob download)
  const exportSettlement = useCallback(
    async (id) => {
      try {
        const res = await API.get(
          `/api/supplier/self/settlement/${id}/export`,
          { responseType: 'blob' },
        );
        const blob = new Blob([res.data], {
          type: 'text/csv;charset=utf-8',
        });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `settlement_${id}.csv`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        return true;
      } catch (e) {
        showError(e.message);
        return false;
      }
    },
    [],
  );

  // Pagination handlers
  const handlePageChange = (page) => {
    loadSettlements(page, pageSize);
  };

  const handlePageSizeChange = async (size) => {
    setPageSize(size);
    await loadSettlements(1, size);
  };

  // 状态筛选：0=全部 / 2=已完成(已结算) / 3=已取消
  const handleStatusChange = (value) => {
    const v = value || 0;
    setStatusFilter(v);
    loadSettlements(1, pageSize, { status: v });
  };

  // 按申请时间段筛选：dateRange 返回 [Date, Date]，结束日覆盖到当天 23:59:59
  const handleDateRangeChange = (range) => {
    let st = 0;
    let et = 0;
    if (Array.isArray(range) && range[0] && range[1]) {
      st = Math.floor(new Date(range[0]).getTime() / 1000);
      et = Math.floor(new Date(range[1]).getTime() / 1000) + 86399;
    }
    setStartTs(st);
    setEndTs(et);
    loadSettlements(1, pageSize, { startTs: st, endTs: et });
  };

  // Initial load
  useEffect(() => {
    loadSettlements(1, pageSize);
    getPendingAmount();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 待结算金额非轮询：页面重新获得焦点/变为可见时拉一次最新值，
  // 避免长时间停留导致顶部金额陈旧（实际结算金额仍以后端原子快照为准）。
  useEffect(() => {
    const refreshPendingOnVisible = () => {
      if (document.visibilityState === 'visible') {
        getPendingAmount();
      }
    };
    window.addEventListener('focus', refreshPendingOnVisible);
    document.addEventListener('visibilitychange', refreshPendingOnVisible);
    return () => {
      window.removeEventListener('focus', refreshPendingOnVisible);
      document.removeEventListener('visibilitychange', refreshPendingOnVisible);
    };
  }, [getPendingAmount]);

  return {
    // State
    settlements,
    loading,
    applying,
    activePage,
    pageSize,
    total,
    pending,
    statusFilter,
    startTs,
    endTs,

    // Functions
    loadSettlements,
    refresh,
    getPendingAmount,
    applySettlement,
    cancelSettlement,
    getDetail,
    getBreakdown,
    getSettlementLogs,
    exportSettlement,
    handlePageChange,
    handlePageSizeChange,
    handleStatusChange,
    handleDateRangeChange,

    // Translation
    t,
  };
};
