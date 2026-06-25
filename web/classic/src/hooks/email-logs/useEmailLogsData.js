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
import { Modal } from '@douyinfe/semi-ui';
import {
  API,
  copy,
  getTodayStartTimestamp,
  isAdmin,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useEmailLogsData = () => {
  const { t } = useTranslation();

  const COLUMN_KEYS = {
    SEND_TIME: 'send_time',
    STATUS: 'status',
    RECEIVER: 'receiver',
    SUBJECT: 'subject',
    PROVIDER: 'provider',
    DURATION: 'duration',
    ERROR_MESSAGE: 'error_message',
  };

  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [logCount, setLogCount] = useState(0);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [formApi, setFormApi] = useState(null);
  const [showColumnSelector, setShowColumnSelector] = useState(false);
  const [compactMode, setCompactMode] = useTableCompactMode('emailLogs');

  const isAdminUser = isAdmin();
  const STORAGE_KEY = 'email-logs-table-columns-admin';

  const now = new Date();
  const formInitValues = {
    receiver: '',
    subject: '',
    provider: '',
    status: '',
    dateRange: [
      timestamp2string(getTodayStartTimestamp()),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const getDefaultColumnVisibility = () => ({
    [COLUMN_KEYS.SEND_TIME]: true,
    [COLUMN_KEYS.STATUS]: true,
    [COLUMN_KEYS.RECEIVER]: true,
    [COLUMN_KEYS.SUBJECT]: true,
    [COLUMN_KEYS.PROVIDER]: true,
    [COLUMN_KEYS.DURATION]: true,
    [COLUMN_KEYS.ERROR_MESSAGE]: true,
  });

  const getInitialVisibleColumns = () => {
    const defaults = getDefaultColumnVisibility();
    const savedColumns = localStorage.getItem(STORAGE_KEY);
    if (!savedColumns) return defaults;

    try {
      return { ...defaults, ...JSON.parse(savedColumns) };
    } catch (e) {
      console.error('Failed to parse saved email log columns', e);
      return defaults;
    }
  };

  const [visibleColumns, setVisibleColumns] = useState(
    getInitialVisibleColumns,
  );

  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    let start_timestamp = formInitValues.dateRange[0];
    let end_timestamp = formInitValues.dateRange[1];

    if (
      formValues.dateRange &&
      Array.isArray(formValues.dateRange) &&
      formValues.dateRange.length === 2
    ) {
      start_timestamp = formValues.dateRange[0];
      end_timestamp = formValues.dateRange[1];
    }

    return {
      receiver: formValues.receiver || '',
      subject: formValues.subject || '',
      provider: formValues.provider || '',
      status: formValues.status || '',
      start_timestamp,
      end_timestamp,
    };
  };

  const parseTimestamp = (value) => {
    const parsed = parseInt(Date.parse(value) / 1000);
    return Number.isNaN(parsed) ? 0 : parsed;
  };

  const enrichLogs = (items) =>
    items.map((log) => ({
      ...log,
      key: String(log.id),
      send_time_text: log.created_at ? timestamp2string(log.created_at) : '-',
    }));

  const syncPageData = (payload) => {
    setLogs(enrichLogs(payload.items || []));
    setLogCount(payload.total || 0);
    setActivePage(payload.page || 1);
    setPageSize(payload.page_size || pageSize);
  };

  const loadLogs = async (page = 1, size = pageSize) => {
    setLoading(true);
    const {
      receiver,
      subject,
      provider,
      status,
      start_timestamp,
      end_timestamp,
    } = getFormValues();

    const params = new URLSearchParams({
      p: String(page),
      page_size: String(size),
      receiver,
      subject,
      provider,
      status,
      start_timestamp: String(parseTimestamp(start_timestamp)),
      end_timestamp: String(parseTimestamp(end_timestamp)),
    });

    try {
      const res = await API.get(`/api/log/email?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        syncPageData(data);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = (page) => {
    loadLogs(page, pageSize).then();
  };

  const handlePageSizeChange = async (size) => {
    localStorage.setItem('email-page-size', size + '');
    await loadLogs(1, size);
  };

  const refresh = async () => {
    await loadLogs(1, pageSize);
  };

  const copyText = async (text) => {
    if (await copy(text)) {
      showSuccess(t('已复制：') + text);
    } else {
      Modal.error({ title: t('无法复制到剪贴板，请手动复制'), content: text });
    }
  };

  const initDefaultColumns = () => {
    const defaults = getDefaultColumnVisibility();
    setVisibleColumns(defaults);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(defaults));
  };

  const handleColumnVisibilityChange = (columnKey, checked) => {
    setVisibleColumns({ ...visibleColumns, [columnKey]: checked });
  };

  const handleSelectAll = (checked) => {
    const updatedColumns = {};
    Object.values(COLUMN_KEYS).forEach((key) => {
      updatedColumns[key] = checked;
    });
    setVisibleColumns(updatedColumns);
  };

  useEffect(() => {
    if (Object.keys(visibleColumns).length > 0) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(visibleColumns));
    }
  }, [visibleColumns]);

  useEffect(() => {
    const localPageSize =
      parseInt(localStorage.getItem('email-page-size')) || ITEMS_PER_PAGE;
    setPageSize(localPageSize);
    loadLogs(1, localPageSize).then();
  }, []);

  return {
    logs,
    loading,
    activePage,
    logCount,
    pageSize,
    isAdminUser,
    formApi,
    setFormApi,
    formInitValues,
    getFormValues,
    visibleColumns,
    showColumnSelector,
    setShowColumnSelector,
    handleColumnVisibilityChange,
    handleSelectAll,
    initDefaultColumns,
    COLUMN_KEYS,
    compactMode,
    setCompactMode,
    loadLogs,
    handlePageChange,
    handlePageSizeChange,
    refresh,
    copyText,
    enrichLogs,
    syncPageData,
    t,
  };
};
