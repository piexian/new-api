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
import { Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';

const Loan = () => {
  const { t } = useTranslation();
  return (
    <div className='classic-page-fill flex justify-center items-center p-8'>
      <Empty
        image={
          <IllustrationConstruction style={{ width: 250, height: 250 }} />
        }
        darkModeImage={
          <IllustrationConstructionDark style={{ width: 250, height: 250 }} />
        }
        title={t('词元贷')}
        description={t('功能建设中，敬请期待')}
      />
    </div>
  );
};

export default Loan;
