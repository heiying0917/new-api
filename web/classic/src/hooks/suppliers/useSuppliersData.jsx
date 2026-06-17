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

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useSuppliersData = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('suppliers');

  // State management
  const [suppliers, setSuppliers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [searching, setSearching] = useState(false);
  const [supplierCount, setSupplierCount] = useState(0);

  // Summary bar data
  const [summary, setSummary] = useState(null);

  // Sorting state
  const [sortBy, setSortBy] = useState('');
  const [sortOrder, setSortOrder] = useState('desc');

  // Modal states
  const [showEditSupplier, setShowEditSupplier] = useState(false);
  const [editingSupplier, setEditingSupplier] = useState({
    user_id: undefined,
  });

  // Immediate settlement confirm modal state
  const [showConfirm, setShowConfirm] = useState(false);
  const [confirmRecord, setConfirmRecord] = useState(null);

  // Form initial values
  const formInitValues = {
    searchKeyword: '',
  };

  // Form API reference
  const [formApi, setFormApi] = useState(null);

  // Get form values helper function
  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
    };
  };

  // Set supplier format with key field
  const setSupplierFormat = (list) => {
    for (let i = 0; i < list.length; i++) {
      list[i].key = list[i].user_id;
    }
    setSuppliers(list);
  };

  // Load suppliers data
  const loadSuppliers = async (
    startIdx,
    pageSize,
    nextSortBy = sortBy,
    nextSortOrder = sortOrder,
  ) => {
    setLoading(true);
    const sortParam = nextSortBy
      ? `&sort_by=${nextSortBy}&sort_order=${nextSortOrder}`
      : '';
    const res = await API.get(
      `/api/supplier/?p=${startIdx}&page_size=${pageSize}${sortParam}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      const newPageData = data.items || [];
      setActivePage(data.page);
      setSupplierCount(data.total);
      setSupplierFormat(newPageData);
    } else {
      showError(message);
    }
    setLoading(false);
  };

  // Search suppliers with keyword
  const searchSuppliers = async (
    startIdx,
    pageSize,
    searchKeyword = null,
    nextSortBy = sortBy,
    nextSortOrder = sortOrder,
  ) => {
    if (searchKeyword === null) {
      const formValues = getFormValues();
      searchKeyword = formValues.searchKeyword;
    }

    if (searchKeyword === '') {
      await loadSuppliers(startIdx, pageSize, nextSortBy, nextSortOrder);
      return;
    }
    setSearching(true);
    const sortParam = nextSortBy
      ? `&sort_by=${nextSortBy}&sort_order=${nextSortOrder}`
      : '';
    const res = await API.get(
      `/api/supplier/search?keyword=${searchKeyword}&p=${startIdx}&page_size=${pageSize}${sortParam}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      const newPageData = data.items || [];
      setActivePage(data.page);
      setSupplierCount(data.total);
      setSupplierFormat(newPageData);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  // Load top summary bar metrics
  const loadSummary = async () => {
    const res = await API.get('/api/supplier/summary');
    const { success, data } = res.data;
    if (success) setSummary(data);
  };

  // Handle column sort change (server-side); reload first page with new sort
  const handleSortChange = (nextSortBy, nextOrder) => {
    setSortBy(nextSortBy);
    setSortOrder(nextOrder);
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      loadSuppliers(0, pageSize, nextSortBy, nextOrder).then();
    } else {
      searchSuppliers(0, pageSize, searchKeyword, nextSortBy, nextOrder).then();
    }
  };

  // Immediate settlement: initiate a settlement bill for the supplier, then open confirm modal
  const initiateSettlement = async (record) => {
    const res = await API.post('/api/admin/settlement/initiate', {
      supplier_id: record.user_id,
    });
    const { success, message, data } = res.data;
    if (!success) {
      showError(message);
      return;
    }
    setConfirmRecord(data);
    setShowConfirm(true);
  };

  // Confirm (settle) the bill
  const confirmSettlement = async (id, values) => {
    const res = await API.post(`/api/admin/settlement/${id}/confirm`, values);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('结算确认成功！'));
      await refresh();
      await loadSummary();
      return true;
    }
    showError(message);
    return false;
  };

  const closeConfirm = () => {
    setShowConfirm(false);
    setTimeout(() => setConfirmRecord(null), 300);
  };

  // Update supplier (priority / enabled / settlement_mode / settlement_cycle / remark)
  const updateSupplier = async (payload) => {
    const res = await API.put('/api/supplier/', payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('供应商信息更新成功！'));
    } else {
      showError(message);
    }
    return success;
  };

  // Handle page change
  const handlePageChange = (page) => {
    setActivePage(page);
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      loadSuppliers(page, pageSize, sortBy, sortOrder).then();
    } else {
      searchSuppliers(page, pageSize, searchKeyword, sortBy, sortOrder).then();
    }
  };

  // Handle page size change (reset to first page, preserve sort & active search)
  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    setActivePage(1);
    const { searchKeyword } = getFormValues();
    try {
      if (searchKeyword === '') {
        await loadSuppliers(1, size, sortBy, sortOrder);
      } else {
        await searchSuppliers(1, size, searchKeyword, sortBy, sortOrder);
      }
    } catch (reason) {
      showError(reason);
    }
  };

  // Handle table row styling for disabled suppliers
  const handleRow = (record, index) => {
    if (!record.enabled) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    }
    return {};
  };

  // Refresh data
  const refresh = async (page = activePage) => {
    const { searchKeyword } = getFormValues();
    if (searchKeyword === '') {
      await loadSuppliers(page, pageSize, sortBy, sortOrder);
    } else {
      await searchSuppliers(page, pageSize, searchKeyword, sortBy, sortOrder);
    }
  };

  // Modal control functions
  const closeEditSupplier = () => {
    setShowEditSupplier(false);
    setEditingSupplier({
      user_id: undefined,
    });
  };

  // Initialize data on component mount
  useEffect(() => {
    loadSuppliers(0, pageSize)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    loadSummary()
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, []);

  return {
    // Data state
    suppliers,
    loading,
    activePage,
    pageSize,
    supplierCount,
    searching,

    // Summary bar
    summary,
    loadSummary,

    // Sorting
    sortBy,
    sortOrder,
    handleSortChange,

    // Immediate settlement
    initiateSettlement,
    confirmSettlement,
    showConfirm,
    confirmRecord,
    closeConfirm,

    // Modal state
    showEditSupplier,
    editingSupplier,
    setShowEditSupplier,
    setEditingSupplier,
    closeEditSupplier,

    // Form state
    formInitValues,
    formApi,
    setFormApi,

    // UI state
    compactMode,
    setCompactMode,

    // Actions
    loadSuppliers,
    searchSuppliers,
    updateSupplier,
    handlePageChange,
    handlePageSizeChange,
    handleRow,
    refresh,
    getFormValues,

    // Translation
    t,
  };
};
