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

export const useSupplierChannelsData = () => {
  const { t } = useTranslation();

  // List state
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');

  // Edit/modal state
  const [showEdit, setShowEdit] = useState(false);
  const [editingChannel, setEditingChannel] = useState({ id: undefined });

  // Load channels list
  const loadChannels = useCallback(
    async (page = 1, size = pageSize, kw = keyword) => {
      setLoading(true);
      try {
        const res = await API.get(
          `/api/supplier/channel/?p=${page}&page_size=${size}&keyword=${encodeURIComponent(
            kw || '',
          )}`,
        );
        const { success, message, data } = res.data;
        if (success) {
          setChannels(data?.items || []);
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
    [pageSize, keyword],
  );

  // Refresh current page
  const refresh = useCallback(
    async (page = activePage) => {
      await loadChannels(page, pageSize, keyword);
    },
    [activePage, pageSize, keyword, loadChannels],
  );

  // Search by keyword
  const searchChannels = useCallback(
    async (kw) => {
      const nextKw = kw ?? keyword;
      setKeyword(nextKw);
      await loadChannels(1, pageSize, nextKw);
    },
    [keyword, pageSize, loadChannels],
  );

  // Get a single channel (with key)
  const getChannel = useCallback(
    async (id) => {
      try {
        const res = await API.get(`/api/supplier/channel/${id}`);
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
    },
    [],
  );

  // Create a channel
  const createChannel = useCallback(
    async (values) => {
      try {
        const res = await API.post('/api/supplier/channel/', values);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('渠道创建成功！'));
          await refresh(1);
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

  // Update a channel
  const updateChannel = useCallback(
    async (values) => {
      try {
        const res = await API.put('/api/supplier/channel/', values);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('渠道更新成功！'));
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

  // Delete a channel
  const deleteChannel = useCallback(
    async (id) => {
      try {
        const res = await API.delete(`/api/supplier/channel/${id}`);
        const { success, message } = res.data;
        if (success) {
          showSuccess(t('渠道删除成功！'));
          // If we just deleted the last item on a page beyond the first, step back
          const nextPage =
            channels.length === 1 && activePage > 1 ? activePage - 1 : activePage;
          await refresh(nextPage);
          return true;
        }
        showError(message);
        return false;
      } catch (e) {
        showError(e.message);
        return false;
      }
    },
    [refresh, t, channels.length, activePage],
  );

  // Close edit modal
  const closeEdit = () => {
    setShowEdit(false);
    setTimeout(() => {
      setEditingChannel({ id: undefined });
    }, 500);
  };

  // Pagination handlers
  const handlePageChange = (page) => {
    loadChannels(page, pageSize, keyword);
  };

  const handlePageSizeChange = async (size) => {
    setPageSize(size);
    await loadChannels(1, size, keyword);
  };

  // Initial load
  useEffect(() => {
    loadChannels(1, pageSize, '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    // State
    channels,
    loading,
    activePage,
    pageSize,
    total,
    keyword,
    setKeyword,

    // Edit state
    showEdit,
    setShowEdit,
    editingChannel,
    setEditingChannel,
    closeEdit,

    // Functions
    loadChannels,
    refresh,
    searchChannels,
    getChannel,
    createChannel,
    updateChannel,
    deleteChannel,
    handlePageChange,
    handlePageSizeChange,

    // Translation
    t,
  };
};
