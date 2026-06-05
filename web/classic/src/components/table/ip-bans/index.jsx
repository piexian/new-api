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

import React from 'react';
import CardPro from '../../common/ui/CardPro';
import IPBansActions from './IPBansActions';
import IPBansDescription from './IPBansDescription';
import IPBansFilters from './IPBansFilters';
import IPBansTable from './IPBansTable';
import BatchIPBanModal from './modals/BatchIPBanModal';
import EditIPBanModal from './modals/EditIPBanModal';
import { useIPBansData } from '../../../hooks/ip-bans/useIPBansData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const IPBansPage = () => {
  const ipBansData = useIPBansData();
  const isMobile = useIsMobile();
  const {
    showAddIPBan,
    showEditIPBan,
    showBatchIPBan,
    editingIPBan,
    closeAddIPBan,
    closeEditIPBan,
    closeBatchIPBan,
    refresh,
    formInitValues,
    setFormApi,
    searchIPBans,
    loadIPBans,
    pageSize,
    loading,
    searching,
    compactMode,
    setCompactMode,
    setShowAddIPBan,
    setShowBatchIPBan,
    t,
  } = ipBansData;

  return (
    <>
      <EditIPBanModal
        visible={showAddIPBan}
        editingIPBan={{ id: undefined }}
        handleClose={closeAddIPBan}
        refresh={refresh}
      />
      <EditIPBanModal
        visible={showEditIPBan}
        editingIPBan={editingIPBan}
        handleClose={closeEditIPBan}
        refresh={refresh}
      />
      <BatchIPBanModal
        visible={showBatchIPBan}
        handleClose={closeBatchIPBan}
        refresh={refresh}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <IPBansDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <IPBansActions
              setShowAddIPBan={setShowAddIPBan}
              setShowBatchIPBan={setShowBatchIPBan}
              t={t}
            />
            <IPBansFilters
              formInitValues={formInitValues}
              setFormApi={setFormApi}
              searchIPBans={searchIPBans}
              loadIPBans={loadIPBans}
              pageSize={pageSize}
              loading={loading}
              searching={searching}
              t={t}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: ipBansData.activePage,
          pageSize: ipBansData.pageSize,
          total: ipBansData.ipBanCount,
          onPageChange: ipBansData.handlePageChange,
          onPageSizeChange: ipBansData.handlePageSizeChange,
          isMobile,
          t,
        })}
        t={t}
      >
        <IPBansTable {...ipBansData} />
      </CardPro>
    </>
  );
};

export default IPBansPage;
