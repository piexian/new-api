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
import { Empty, Descriptions } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getLogsColumns } from './UsageLogsColumnDefs';
import { getLoginAuditColumns } from './LoginAuditColumnDefs';

const LogsTable = (logsData) => {
  const {
    logs,
    expandData,
    loading,
    activePage,
    pageSize,
    logCount,
    compactMode,
    visibleColumns,
    handlePageChange,
    handlePageSizeChange,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    hasExpandableRows,
    isAdminUser,
    billingDisplayMode,
    logScope,
    t,
    COLUMN_KEYS,
  } = logsData;

  // Get all columns
  const allColumns = useMemo(() => {
    return getLogsColumns({
      t,
      COLUMN_KEYS,
      copyText,
      showUserInfoFunc,
      openChannelAffinityUsageCacheModal,
      isAdminUser,
      billingDisplayMode,
    });
  }, [
    t,
    COLUMN_KEYS,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
  ]);

  // 登录审计视图使用固定的审计列，不受消费日志列设置影响
  const auditColumns = useMemo(() => {
    return getLoginAuditColumns({ t, isAdminUser, copyText });
  }, [t, isAdminUser, copyText]);

  // Filter columns based on visibility settings
  const visibleColumnsList = useMemo(() => {
    if (logScope === 'login') {
      return auditColumns;
    }
    return allColumns.filter((column) => visibleColumns[column.key]);
  }, [logScope, auditColumns, visibleColumns, allColumns]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? visibleColumnsList.map(({ fixed, ...rest }) => rest)
      : visibleColumnsList;
  }, [compactMode, visibleColumnsList]);

  const expandRowRender = (record, index) => {
    // 登录审计详情单独占一格，给长 UA 留出宽度
    const isLoginAudit = record.type === 7 || record.type === 8;
    return (
      <Descriptions
        data={expandData[record.key]}
        {...(isLoginAudit ? { column: 1, align: 'plain' } : null)}
      />
    );
  };

  return (
    <CardTable
      columns={tableColumns}
      {...(hasExpandableRows() && {
        expandedRowRender: expandRowRender,
        expandRowByClick: true,
        rowExpandable: (record) =>
          expandData[record.key] && expandData[record.key].length > 0,
      })}
      dataSource={logs}
      rowKey='key'
      loading={loading}
      scroll={compactMode ? undefined : { x: 'max-content' }}
      className='rounded-xl overflow-hidden'
      size='small'
      empty={
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={t('搜索无结果')}
          style={{ padding: 30 }}
        />
      }
      pagination={{
        currentPage: activePage,
        pageSize: pageSize,
        total: logCount,
        pageSizeOptions: [10, 20, 50, 100],
        showSizeChanger: true,
        onPageSizeChange: (size) => {
          handlePageSizeChange(size);
        },
        onPageChange: handlePageChange,
      }}
      hidePagination={true}
    />
  );
};

export default LogsTable;
