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
import { useSearchParams } from 'react-router-dom';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';

export const useSettlementReviewData = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();

  // List state
  const [settlements, setSettlements] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);

  // Filter state
  // status: 0 = all, 1 = 已申请, 2 = 已结算, 3 = 已取消
  const [statusFilter, setStatusFilter] = useState(0);
  // supplierId: '' or 0 = all. Pre-filled from URL ?supplier_id=
  const [supplierId, setSupplierId] = useState('');
  // keyword: fuzzy match supplier username/email
  const [keyword, setKeyword] = useState('');

  // Detail SideSheet state
  const [showDetail, setShowDetail] = useState(false);
  const [detailRecord, setDetailRecord] = useState(null);

  // Confirm modal state
  const [showConfirm, setShowConfirm] = useState(false);
  const [confirmRecord, setConfirmRecord] = useState(null);

  // Load settlement list
  const loadSettlements = useCallback(
    async (
      page = 1,
      size = pageSize,
      status = statusFilter,
      sid = supplierId,
      kw = keyword,
    ) => {
      setLoading(true);
      try {
        const params = new URLSearchParams();
        params.set('p', page);
        params.set('page_size', size);
        params.set('status', status || 0);
        if (
          sid !== '' &&
          sid !== null &&
          sid !== undefined &&
          Number(sid) > 0
        ) {
          params.set('supplier_id', sid);
        }
        const trimmedKw = (kw || '').trim();
        if (trimmedKw !== '') {
          params.set('keyword', trimmedKw);
        }
        const res = await API.get(
          `/api/admin/settlement/?${params.toString()}`,
        );
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
    [pageSize, statusFilter, supplierId, keyword],
  );

  // Refresh current page with current filters
  const refresh = useCallback(
    async (page = activePage) => {
      await loadSettlements(page, pageSize, statusFilter, supplierId, keyword);
    },
    [activePage, pageSize, statusFilter, supplierId, keyword, loadSettlements],
  );

  // Change status filter and reload from page 1 (keeps keyword composable)
  const changeStatus = useCallback(
    async (status) => {
      const next = status || 0;
      setStatusFilter(next);
      await loadSettlements(1, pageSize, next, supplierId, keyword);
    },
    [pageSize, supplierId, keyword, loadSettlements],
  );

  // Clear supplier filter (drop URL-derived filter)
  const clearSupplierFilter = useCallback(async () => {
    setSupplierId('');
    await loadSettlements(1, pageSize, statusFilter, '', keyword);
  }, [pageSize, statusFilter, keyword, loadSettlements]);

  // Search by keyword: keyword supersedes the supplier_id drill-down,
  // so clear the supplier chip when searching. Status stays composable.
  const searchByKeyword = useCallback(async () => {
    setSupplierId('');
    await loadSettlements(1, pageSize, statusFilter, '', keyword);
  }, [pageSize, statusFilter, keyword, loadSettlements]);

  // Reset filters: clear keyword (and status, and supplier chip) and reload
  const resetFilters = useCallback(async () => {
    setKeyword('');
    setStatusFilter(0);
    setSupplierId('');
    await loadSettlements(1, pageSize, 0, '', '');
  }, [pageSize, loadSettlements]);

  // Pagination handlers
  const handlePageChange = (page) => {
    loadSettlements(page, pageSize, statusFilter, supplierId, keyword);
  };

  const handlePageSizeChange = async (size) => {
    setPageSize(size);
    await loadSettlements(1, size, statusFilter, supplierId, keyword);
  };

  // Detail control
  const openDetail = (record) => {
    setDetailRecord(record);
    setShowDetail(true);
  };

  const closeDetail = () => {
    setShowDetail(false);
    setTimeout(() => {
      setDetailRecord(null);
    }, 300);
  };

  // Confirm settlement control
  const openConfirm = (record) => {
    setConfirmRecord(record);
    setShowConfirm(true);
  };

  const closeConfirm = () => {
    setShowConfirm(false);
    setTimeout(() => {
      setConfirmRecord(null);
    }, 300);
  };

  // Submit confirm settlement
  const confirmSettlement = useCallback(
    async (id, values) => {
      try {
        const res = await API.post(
          `/api/admin/settlement/${id}/confirm`,
          values,
        );
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('结算确认成功！'));
          await refresh();
          return true;
        }
        showError(message);
        return false;
      } catch (e) {
        showError(e.message);
        return false;
      }
    },
    [refresh, t],
  );

  // Cancel a settlement
  const cancelSettlement = useCallback(
    async (id) => {
      try {
        const res = await API.post(`/api/admin/settlement/${id}/cancel`);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('结算已取消'));
          await refresh();
          return true;
        }
        showError(message);
        return false;
      } catch (e) {
        showError(e.message);
        return false;
      }
    },
    [refresh, t],
  );

  // Export a settlement as CSV (open in new tab)
  const exportSettlement = useCallback((id) => {
    const url = `/api/admin/settlement/${id}/export`;
    window.open(url, '_blank');
  }, []);

  // Fetch per-channel breakdown for a settlement
  const fetchBreakdown = useCallback(async (id) => {
    try {
      const res = await API.get(`/api/admin/settlement/${id}/breakdown`);
      const { success, message, data } = res.data;
      if (success) {
        return Array.isArray(data) ? data : [];
      }
      showError(message);
      return [];
    } catch (e) {
      showError(e.message);
      return [];
    }
  }, []);

  // Fetch paginated logs for a settlement
  const fetchLogs = useCallback(async (id, page = 1, size = 10) => {
    try {
      const res = await API.get(
        `/api/admin/settlement/${id}/logs?p=${page}&page_size=${size}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        return data || { items: [], total: 0, page, page_size: size };
      }
      showError(message);
      return { items: [], total: 0, page, page_size: size };
    } catch (e) {
      showError(e.message);
      return { items: [], total: 0, page, page_size: size };
    }
  }, []);

  // Initial load: read supplier_id from URL and pre-filter
  useEffect(() => {
    const urlSupplierId = searchParams.get('supplier_id') || '';
    const initialSid =
      urlSupplierId && Number(urlSupplierId) > 0 ? urlSupplierId : '';
    setSupplierId(initialSid);
    loadSettlements(1, pageSize, 0, initialSid, '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    // List state
    settlements,
    loading,
    activePage,
    pageSize,
    total,

    // Filter state
    statusFilter,
    supplierId,
    keyword,
    setKeyword,
    changeStatus,
    clearSupplierFilter,
    searchByKeyword,
    resetFilters,

    // Detail state
    showDetail,
    detailRecord,
    openDetail,
    closeDetail,

    // Confirm state
    showConfirm,
    confirmRecord,
    openConfirm,
    closeConfirm,

    // Actions
    loadSettlements,
    refresh,
    handlePageChange,
    handlePageSizeChange,
    confirmSettlement,
    cancelSettlement,
    exportSettlement,
    fetchBreakdown,
    fetchLogs,

    // Translation
    t,
  };
};
