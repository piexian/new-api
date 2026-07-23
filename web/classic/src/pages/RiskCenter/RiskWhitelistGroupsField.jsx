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
import React, { useEffect, useMemo, useState } from 'react';
import { Form } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';

const RiskWhitelistGroupsField = ({ selectedGroups = [] }) => {
  const { t } = useTranslation();
  const [groups, setGroups] = useState([]);

  useEffect(() => {
    const fetchGroups = async () => {
      try {
        const response = await API.get('/api/group/');
        if (response.data.success && Array.isArray(response.data.data)) {
          setGroups(response.data.data);
        }
      } catch (error) {
        showError(error.message);
      }
    };

    fetchGroups();
  }, []);

  const optionList = useMemo(() => {
    const values = new Set([
      ...groups,
      ...(Array.isArray(selectedGroups) ? selectedGroups : []),
    ]);
    return Array.from(values).map((group) => ({ label: group, value: group }));
  }, [groups, selectedGroups]);

  return (
    <Form.Select
      field='whitelist_groups'
      label={t('白名单分组')}
      placeholder={t('请选择白名单分组')}
      optionList={optionList}
      multiple
      filter
    />
  );
};

export default RiskWhitelistGroupsField;
