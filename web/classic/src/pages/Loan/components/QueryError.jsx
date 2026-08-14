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
import { Button, Typography } from '@douyinfe/semi-ui';

// 查询失败占位：展示后端 message + 重试，对齐 default 主题 QueryErrorState
const QueryError = ({ t, message, onRetry }) => (
  <div className='flex flex-col items-center gap-3 py-8'>
    <Typography.Text type='danger'>{message || t('加载失败')}</Typography.Text>
    <Button theme='outline' onClick={onRetry}>
      {t('重试')}
    </Button>
  </div>
);

export default QueryError;
