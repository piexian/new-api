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
import { Layout } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import EmailLogsActions from './EmailLogsActions';
import EmailLogsFilters from './EmailLogsFilters';
import EmailLogsTable from './EmailLogsTable';
import ColumnSelectorModal from './modals/ColumnSelectorModal';
import { useEmailLogsData } from '../../../hooks/email-logs/useEmailLogsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const EmailLogsPage = () => {
  const emailLogsData = useEmailLogsData();
  const isMobile = useIsMobile();

  return (
    <>
      <ColumnSelectorModal {...emailLogsData} />
      <Layout>
        <CardPro
          type='type2'
          statsArea={<EmailLogsActions {...emailLogsData} />}
          searchArea={<EmailLogsFilters {...emailLogsData} />}
          paginationArea={createCardProPagination({
            currentPage: emailLogsData.activePage,
            pageSize: emailLogsData.pageSize,
            total: emailLogsData.logCount,
            onPageChange: emailLogsData.handlePageChange,
            onPageSizeChange: emailLogsData.handlePageSizeChange,
            isMobile: isMobile,
            t: emailLogsData.t,
          })}
          t={emailLogsData.t}
        >
          <EmailLogsTable {...emailLogsData} />
        </CardPro>
      </Layout>
    </>
  );
};

export default EmailLogsPage;
