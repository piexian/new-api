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

import React, { useCallback, useMemo } from 'react';
import { Card, Table, Empty } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getPricingTableColumns } from './PricingTableColumns';
import {
  PRICING_SORT_OPTIONS,
  PRICING_QUOTA_SORT_OPTIONS,
  sortPricingModels,
} from '../../../../../helpers/pricing-sort';

const PricingTable = ({
  filteredModels,
  loading,
  rowSelection,
  pageSize,
  setPageSize,
  sortBy,
  onSortChange,
  selectedGroup,
  groupRatio,
  copyText,
  setModalImageUrl,
  setIsModalOpenurl,
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  searchValue,
  showRatio,
  compactMode = false,
  openModelDetail,
  perfMetricsMap = {},
  t,
}) => {
  const sortedModels = useMemo(
    () => sortPricingModels(filteredModels, sortBy),
    [filteredModels, sortBy],
  );

  const columns = useMemo(() => {
    return getPricingTableColumns({
      t,
      selectedGroup,
      groupRatio,
      copyText,
      setModalImageUrl,
      setIsModalOpenurl,
      currency,
      siteDisplayType,
      tokenUnit,
      displayPrice,
      showRatio,
      perfMetricsMap,
    });
  }, [
    t,
    selectedGroup,
    groupRatio,
    copyText,
    setModalImageUrl,
    setIsModalOpenurl,
    currency,
    siteDisplayType,
    tokenUnit,
    displayPrice,
    showRatio,
    perfMetricsMap,
  ]);

  // 更新列定义中的 searchValue
  const processedColumns = useMemo(() => {
    const cols = columns.map((column) => {
      if (column.dataIndex === 'model_name') {
        return {
          ...column,
          filteredValue: searchValue ? [searchValue] : [],
        };
      }
      if (column.dataIndex === 'quota_type') {
        let sortOrder = false;
        if (sortBy === PRICING_QUOTA_SORT_OPTIONS.ASC) sortOrder = 'ascend';
        if (sortBy === PRICING_QUOTA_SORT_OPTIONS.DESC) sortOrder = 'descend';
        return { ...column, sortOrder };
      }
      return column;
    });

    // Remove fixed property when in compact mode (mobile view)
    if (compactMode) {
      return cols.map(({ fixed, ...rest }) => rest);
    }
    return cols;
  }, [columns, searchValue, compactMode, sortBy]);

  const handleTableChange = useCallback(
    ({ sorter, extra }) => {
      if (extra?.changeType !== 'sorter') return;
      let nextSort = PRICING_SORT_OPTIONS.NAME;
      if (sorter?.sortOrder === 'ascend')
        nextSort = PRICING_QUOTA_SORT_OPTIONS.ASC;
      if (sorter?.sortOrder === 'descend')
        nextSort = PRICING_QUOTA_SORT_OPTIONS.DESC;
      onSortChange(nextSort);
    },
    [onSortChange],
  );

  const ModelTable = useMemo(
    () => (
      <Card className='!rounded-xl overflow-hidden' bordered={false}>
        <Table
          // Keep Semi's local pagination; remount on sort to return to page 1.
          key={sortBy}
          columns={processedColumns}
          dataSource={sortedModels}
          onChange={handleTableChange}
          loading={loading}
          rowSelection={rowSelection}
          scroll={compactMode ? undefined : { x: 'max-content' }}
          onRow={(record) => ({
            onClick: () => openModelDetail && openModelDetail(record),
            style: { cursor: 'pointer' },
          })}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('搜索无结果')}
              style={{ padding: 30 }}
            />
          }
          pagination={{
            defaultPageSize: 20,
            pageSize: pageSize,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50, 100],
            onPageSizeChange: (size) => setPageSize(size),
          }}
        />
      </Card>
    ),
    [
      sortedModels,
      handleTableChange,
      sortBy,
      loading,
      processedColumns,
      rowSelection,
      pageSize,
      setPageSize,
      openModelDetail,
      t,
      compactMode,
    ],
  );

  return ModelTable;
};

export default PricingTable;
