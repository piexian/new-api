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

import React, { useState } from 'react';
import { Avatar, Card, Tabs, TabPane, Typography } from '@douyinfe/semi-ui';
import { Handshake } from 'lucide-react';
import MyOffers from './MyOffers';
import MyFundings from './MyFundings';
import MarketBrowse from './MarketBrowse';

/**
 * 放贷市场：我的供给 / 收益台账 / 市场浏览 三个区块 + 常驻免责声明条。
 * disclaimerAgreed 用于在创建挂单前拦截未同意免责声明的用户。
 */
const LendingMarket = ({
  t,
  disclaimerAgreed,
  onDisclaimerAgreed,
  onBorrowOrder,
  refreshKey,
}) => {
  const [activeTab, setActiveTab] = useState('offers');

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='violet' className='mr-3 shadow-md'>
          <Handshake size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('放贷市场')}
          </Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('将闲置额度出借给其他用户赚取日利息')}
          </div>
        </div>
      </div>

      <Tabs
        type='line'
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key)}
      >
        <TabPane tab={t('我的供给')} itemKey='offers'>
          <MyOffers
            t={t}
            disclaimerAgreed={disclaimerAgreed}
            onDisclaimerAgreed={onDisclaimerAgreed}
            refreshKey={refreshKey}
          />
        </TabPane>
        <TabPane tab={t('收益台账')} itemKey='fundings'>
          <MyFundings t={t} refreshKey={refreshKey} />
        </TabPane>
        <TabPane tab={t('市场')} itemKey='market'>
          <MarketBrowse
            t={t}
            onBorrow={onBorrowOrder}
            refreshKey={refreshKey}
          />
        </TabPane>
      </Tabs>

      {/* 常驻免责声明条 */}
      <div className='mt-4 pt-3 text-xs text-gray-500 dark:text-gray-400 border-t'>
        {t(
          '免责声明：本市场放贷纯属娱乐玩法，并非真实金融。出借额度可能全部损失；平台不兜底、不追偿。',
        )}
      </div>
    </Card>
  );
};

export default LendingMarket;
