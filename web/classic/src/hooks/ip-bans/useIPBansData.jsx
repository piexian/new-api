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

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const IP_BAN_TYPES = {
  PERMANENT: 'permanent',
  TEMPORARY: 'temporary',
};

export const useIPBansData = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('ip-bans');
  const [ipBans, setIPBans] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [ipBanCount, setIPBanCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [showAddIPBan, setShowAddIPBan] = useState(false);
  const [showEditIPBan, setShowEditIPBan] = useState(false);
  const [showBatchIPBan, setShowBatchIPBan] = useState(false);
  const [editingIPBan, setEditingIPBan] = useState({ id: undefined });
  const [formApi, setFormApi] = useState(null);

  const formInitValues = {
    searchKeyword: '',
    searchType: '',
  };

  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
      searchType: formValues.searchType || '',
    };
  };

  const setIPBanFormat = (items) => {
    setIPBans(
      (items || []).map((item) => ({
        ...item,
        key: item.id,
      })),
    );
  };

  const loadIPBans = async (page = 1, size = pageSize, type = '') => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.append('p', String(page));
      params.append('page_size', String(size));
      if (type) params.append('type', type);
      const res = await API.get(`/api/ip_ban/?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        setActivePage(data.page);
        setIPBanCount(data.total);
        setIPBanFormat(data.items);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const searchIPBans = async (
    page = 1,
    size = pageSize,
    searchKeyword = null,
    searchType = null,
  ) => {
    if (searchKeyword === null || searchType === null) {
      const formValues = getFormValues();
      searchKeyword = formValues.searchKeyword;
      searchType = formValues.searchType;
    }

    if (searchKeyword === '' && searchType === '') {
      await loadIPBans(page, size);
      return;
    }

    setSearching(true);
    try {
      const params = new URLSearchParams();
      if (searchKeyword) params.append('keyword', searchKeyword);
      if (searchType) params.append('type', searchType);
      params.append('p', String(page));
      params.append('page_size', String(size));
      const res = await API.get(`/api/ip_ban/search?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        setActivePage(data.page);
        setIPBanCount(data.total);
        setIPBanFormat(data.items);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSearching(false);
    }
  };

  const refresh = async (page = activePage) => {
    const { searchKeyword, searchType } = getFormValues();
    if (searchKeyword === '' && searchType === '') {
      await loadIPBans(page, pageSize);
    } else {
      await searchIPBans(page, pageSize, searchKeyword, searchType);
    }
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    const { searchKeyword, searchType } = getFormValues();
    if (searchKeyword === '' && searchType === '') {
      loadIPBans(page, pageSize).then();
    } else {
      searchIPBans(page, pageSize, searchKeyword, searchType).then();
    }
  };

  const handlePageSizeChange = async (size) => {
    localStorage.setItem('page-size', `${size}`);
    setPageSize(size);
    setActivePage(1);
    const { searchKeyword, searchType } = getFormValues();
    if (searchKeyword === '' && searchType === '') {
      await loadIPBans(1, size);
    } else {
      await searchIPBans(1, size, searchKeyword, searchType);
    }
  };

  const handleRow = (record) => {
    if (record.expires_at > 0 && record.expires_at <= Math.floor(Date.now() / 1000)) {
      return {
        style: {
          background: 'var(--semi-color-disabled-border)',
        },
      };
    }
    return {};
  };

  const closeAddIPBan = () => {
    setShowAddIPBan(false);
  };

  const closeEditIPBan = () => {
    setShowEditIPBan(false);
    setEditingIPBan({ id: undefined });
  };

  const closeBatchIPBan = () => {
    setShowBatchIPBan(false);
  };

  useEffect(() => {
    loadIPBans(1, pageSize).then();
  }, []);

  return {
    ipBans,
    loading,
    activePage,
    pageSize,
    ipBanCount,
    searching,
    showAddIPBan,
    showEditIPBan,
    showBatchIPBan,
    editingIPBan,
    setShowAddIPBan,
    setShowEditIPBan,
    setShowBatchIPBan,
    setEditingIPBan,
    formInitValues,
    setFormApi,
    compactMode,
    setCompactMode,
    loadIPBans,
    searchIPBans,
    handlePageChange,
    handlePageSizeChange,
    handleRow,
    refresh,
    closeAddIPBan,
    closeEditIPBan,
    closeBatchIPBan,
    t,
  };
};
