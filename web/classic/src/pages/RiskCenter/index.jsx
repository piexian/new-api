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
import { Navigate } from 'react-router-dom';
import { TabPane, Tabs, Layout } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { ShieldAlert, Ban, ScrollText } from 'lucide-react';
import { isRoot } from '../../helpers';
import ProbeGuardTab from './ProbeGuardTab';
import ErrorBanTab from './ErrorBanTab';
import BanLogsTab from './BanLogsTab';

const RiskCenter = () => {
  const { t } = useTranslation();
  const [tabActiveKey, setTabActiveKey] = React.useState('probe-guard');

  if (!isRoot()) {
    return <Navigate to='/forbidden' replace />;
  }

  const panes = [
    {
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <ShieldAlert size={18} />
          {t('探针防护')}
        </span>
      ),
      content: <ProbeGuardTab />,
      itemKey: 'probe-guard',
    },
    {
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Ban size={18} />
          {t('错误封禁')}
        </span>
      ),
      content: <ErrorBanTab />,
      itemKey: 'error-ban',
    },
    {
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <ScrollText size={18} />
          {t('封禁日志')}
        </span>
      ),
      content: <BanLogsTab />,
      itemKey: 'ban-logs',
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Layout>
        <Layout.Content>
          <Tabs
            type='card'
            collapsible
            activeKey={tabActiveKey}
            onChange={(key) => setTabActiveKey(key)}
          >
            {panes.map((pane) => (
              <TabPane itemKey={pane.itemKey} tab={pane.tab} key={pane.itemKey}>
                {tabActiveKey === pane.itemKey && pane.content}
              </TabPane>
            ))}
          </Tabs>
        </Layout.Content>
      </Layout>
    </div>
  );
};

export default RiskCenter;
