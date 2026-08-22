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
import { Form, Select } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';

// 传入 onChange 时渲染受控 Select，用于非 Form 绑定场景（如规则编辑对话框）。
const RiskWhitelistGroupsField = ({
  selectedGroups = [],
  field = 'whitelist_groups',
  label,
  placeholder,
  onChange,
}) => {
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

  const resolvedPlaceholder = placeholder ?? t('请选择白名单分组');
  if (onChange) {
    return (
      <Select
        value={selectedGroups}
        onChange={onChange}
        optionList={optionList}
        multiple
        filter
        placeholder={resolvedPlaceholder}
        style={{ width: '100%' }}
      />
    );
  }
  return (
    <Form.Select
      field={field}
      label={label ?? t('白名单分组')}
      placeholder={resolvedPlaceholder}
      optionList={optionList}
      multiple
      filter
    />
  );
};

export default RiskWhitelistGroupsField;
