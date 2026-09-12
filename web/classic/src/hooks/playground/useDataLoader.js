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

import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  processModelsData,
  processGroupsData,
  showError,
} from '../../helpers';
import { API_ENDPOINTS } from '../../constants/playground.constants';

export const useDataLoader = (
  userState,
  inputs,
  handleInputChange,
  setModels,
  setGroups,
) => {
  const { t } = useTranslation();
  const [catalog, setCatalog] = useState({});
  const modelRequest = useRef(0);
  const [loadedModelScope, setLoadedModelScope] = useState(null);

  const loadModels = useCallback(async () => {
    const request = ++modelRequest.current;
    setLoadedModelScope(null);
    setModels([]);
    try {
      const res = await API.get(API_ENDPOINTS.USER_MODELS, {
        params: { group: inputs.group, with_capabilities: 'true' },
        // Only the current request may display errors, including HTTP failures.
        skipErrorHandler: true,
      });
      if (request !== modelRequest.current) return;
      const { success, message, data } = res.data;

      if (success) {
        const { modelOptions } = processModelsData(data);
        setModels(modelOptions);
        setLoadedModelScope({ group: inputs.group, user: userState?.user });
      } else {
        showError(t(message));
      }
    } catch (error) {
      if (request === modelRequest.current) showError(t('加载模型失败'));
    }
  }, [inputs.group, userState?.user, setModels, t]);

  const loadGroups = useCallback(async () => {
    try {
      const res = await API.get(API_ENDPOINTS.USER_GROUPS);
      const { success, message, data } = res.data;

      if (success) {
        const userGroup =
          userState?.user?.group ||
          JSON.parse(localStorage.getItem('user'))?.group;
        const groupOptions = processGroupsData(data, userGroup);
        setGroups(groupOptions);

        const hasCurrentGroup = groupOptions.some(
          (option) => option.value === inputs.group,
        );
        if (!hasCurrentGroup) {
          handleInputChange('group', groupOptions[0]?.value || '');
        }
      } else {
        showError(t(message));
      }
    } catch (error) {
      showError(t('加载分组失败'));
    }
  }, [userState, inputs.group, handleInputChange, setGroups, t]);

  useEffect(() => {
    if (!userState?.user) return;
    let active = true;
    API.get(API_ENDPOINTS.USER_MODELS_DEV_CATALOG, { skipErrorHandler: true })
      .then((res) => {
        if (active && res.data.success && res.data.data)
          setCatalog(res.data.data);
      })
      .catch(() => {}); // Unknown capabilities must remain selectable.
    return () => {
      active = false;
    };
  }, [userState?.user]);

  // Invalidate pending model requests on group changes and unmount.
  useEffect(() => {
    if (userState?.user) loadModels();
    return () => {
      modelRequest.current++;
    };
  }, [userState?.user, loadModels]);

  useEffect(() => {
    if (userState?.user) loadGroups();
  }, [userState?.user, loadGroups]);

  return {
    catalog,
    // This also guards the first render of a new group, before effects clear models.
    modelsReady:
      loadedModelScope !== null &&
      loadedModelScope.group === inputs.group &&
      loadedModelScope.user === userState?.user,
    loadModels,
    loadGroups,
  };
};
