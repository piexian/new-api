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

import React, { useEffect, useState } from 'react';
import {
  Button,
  Form,
  Input,
  Modal,
  Space,
  Switch,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete, IconSave } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

function isHttpIcon(icon) {
  if (!icon) return false;
  const v = String(icon).trim().toLowerCase();
  return v.startsWith('http://') || v.startsWith('https://');
}

function FriendLinkIconPreview({ icon, name }) {
  const value = (icon || '').trim();
  if (value && isHttpIcon(value)) {
    return (
      <img
        src={value}
        alt=''
        style={{ width: 28, height: 28, borderRadius: 6, objectFit: 'cover' }}
      />
    );
  }
  if (value) {
    return (
      <span style={{ fontSize: 20, lineHeight: '28px' }} aria-hidden>
        {value}
      </span>
    );
  }
  return (
    <span style={{ fontWeight: 700 }}>
      {String(name || '?')
        .slice(0, 1)
        .toUpperCase()}
    </span>
  );
}

function normalizeFriendLinks(parsed) {
  const usedIds = new Set();
  let nextId =
    Math.max(
      0,
      ...parsed.map((item) =>
        Number.isInteger(item?.id) && item.id > 0 ? item.id : 0,
      ),
    ) + 1;

  return parsed.map((item, idx) => {
    const row = item && typeof item === 'object' ? item : {};
    let id =
      Number.isInteger(row.id) && row.id > 0 && !usedIds.has(row.id)
        ? row.id
        : 0;
    while (id === 0 || usedIds.has(id)) id = nextId++;
    usedIds.add(id);
    return {
      id,
      name: typeof row.name === 'string' ? row.name : '',
      url: typeof row.url === 'string' ? row.url : '',
      icon: typeof row.icon === 'string' ? row.icon : '',
      description: typeof row.description === 'string' ? row.description : '',
      order: typeof row.order === 'number' ? row.order : idx,
      enabled: row.enabled !== false,
    };
  });
}

const SettingsFriendLinks = ({ options, refresh }) => {
  const { t } = useTranslation();
  const [list, setList] = useState([]);
  const [loading, setLoading] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [panelEnabled, setPanelEnabled] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState(null);
  const [formApi, setFormApi] = useState(null);
  const [draftForm, setDraftForm] = useState({
    name: '',
    url: '',
    icon: '',
    description: '',
    order: 0,
    enabled: true,
  });

  const parseList = (raw) => {
    if (!raw) {
      setList([]);
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      setList(Array.isArray(parsed) ? normalizeFriendLinks(parsed) : []);
    } catch {
      setList([]);
    }
  };

  useEffect(() => {
    parseList(options['console_setting.friend_links']);
  }, [options['console_setting.friend_links']]);

  useEffect(() => {
    const enabledStr = options['console_setting.friend_links_enabled'];
    setPanelEnabled(
      enabledStr === undefined
        ? true
        : enabledStr === true || enabledStr === 'true',
    );
  }, [options['console_setting.friend_links_enabled']]);

  // Modal 打开后回填表单（等 formApi 就绪，避免空白表单）
  useEffect(() => {
    if (!showModal || !formApi) return;
    formApi.setValues(draftForm);
  }, [showModal, formApi, draftForm]);

  const handleToggleEnabled = async (checked) => {
    try {
      const res = await API.put('/api/option/', {
        key: 'console_setting.friend_links_enabled',
        value: checked ? 'true' : 'false',
      });
      if (res.data.success) {
        setPanelEnabled(checked);
        showSuccess(t('设置已更新'));
        refresh?.();
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      showError(e.message || 'error');
    }
  };

  const submit = async () => {
    try {
      setLoading(true);
      if (list.length > 30) {
        showError(t('友链最多 30 条'));
        return;
      }
      const res = await API.put('/api/option/', {
        key: 'console_setting.friend_links',
        value: JSON.stringify(list),
      });
      if (res.data.success) {
        setHasChanges(false);
        showSuccess(t('保存成功'));
        refresh?.();
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      showError(e.message || 'error');
    } finally {
      setLoading(false);
    }
  };

  const emptyForm = () => ({
    name: '',
    url: '',
    icon: '',
    description: '',
    order: list.length,
    enabled: true,
  });

  const openAdd = () => {
    setEditing(null);
    setDraftForm(emptyForm());
    setShowModal(true);
  };

  const openEdit = (row) => {
    setEditing(row);
    setDraftForm({
      name: row.name || '',
      url: row.url || '',
      icon: row.icon || '',
      description: row.description || '',
      order: typeof row.order === 'number' ? row.order : 0,
      enabled: row.enabled !== false,
    });
    setShowModal(true);
  };

  const saveRow = () => {
    formApi?.validate().then((values) => {
      if (editing) {
        setList((prev) =>
          prev.map((item, idx) =>
            idx === editing.__idx ? { ...item, ...values } : item,
          ),
        );
      } else {
        if (list.length >= 30) {
          showError(t('友链最多 30 条'));
          return;
        }
        const id = Math.max(0, ...list.map((item) => item.id || 0)) + 1;
        setList((prev) => [...prev, { id, ...values }]);
      }
      setHasChanges(true);
      setShowModal(false);
      showSuccess(t('已更新，请点击保存设置'));
    });
  };

  const removeRow = (idx) => {
    setList((prev) => prev.filter((_, i) => i !== idx));
    setHasChanges(true);
  };

  const move = (idx, dir) => {
    setList((prev) => {
      const next = [...prev];
      const j = idx + dir;
      if (j < 0 || j >= next.length) return prev;
      const tmp = next[idx];
      next[idx] = next[j];
      next[j] = tmp;
      return next.map((item, order) => ({ ...item, order }));
    });
    setHasChanges(true);
  };

  const columns = [
    {
      title: t('图标'),
      dataIndex: 'icon',
      width: 64,
      render: (icon, record) => (
        <FriendLinkIconPreview icon={icon} name={record.name} />
      ),
    },
    {
      title: t('名称'),
      dataIndex: 'name',
    },
    {
      title: t('URL'),
      dataIndex: 'url',
      render: (text) => (
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 220 }}
        >
          {text}
        </Typography.Text>
      ),
    },
    {
      title: t('描述'),
      dataIndex: 'description',
    },
    {
      title: t('启用'),
      dataIndex: 'enabled',
      render: (v) => (v === false ? t('否') : t('是')),
    },
    {
      title: t('操作'),
      render: (_, record, idx) => (
        <Space>
          <Button size='small' onClick={() => move(idx, -1)}>
            ↑
          </Button>
          <Button size='small' onClick={() => move(idx, 1)}>
            ↓
          </Button>
          <Button
            size='small'
            onClick={() => openEdit({ ...record, __idx: idx })}
          >
            {t('编辑')}
          </Button>
          <Button size='small' type='danger' onClick={() => removeRow(idx)}>
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          marginBottom: 12,
        }}
      >
        <Space>
          <Switch checked={panelEnabled} onChange={handleToggleEnabled} />
          <span>{t('启用友链悬浮球')}</span>
        </Space>
        <Space>
          <Button icon={<IconPlus />} onClick={openAdd}>
            {t('添加')}
          </Button>
          <Button
            icon={<IconSave />}
            type='secondary'
            loading={loading}
            disabled={!hasChanges}
            onClick={submit}
          >
            {t('保存设置')}
          </Button>
        </Space>
      </div>
      <Table
        columns={columns}
        dataSource={list.map((item) => ({ ...item, key: item.id }))}
        pagination={false}
      />

      <Modal
        title={editing ? t('编辑友链') : t('添加友链')}
        visible={showModal}
        onOk={saveRow}
        onCancel={() => setShowModal(false)}
        okText={t('确定')}
        cancelText={t('取消')}
      >
        <Form
          key={editing ? `edit-${editing.__idx}` : 'add'}
          getFormApi={setFormApi}
          initValues={draftForm}
          labelPosition='top'
        >
          <Form.Input
            field='name'
            label={t('名称')}
            rules={[{ required: true, message: t('必填') }]}
          />
          <Form.Input
            field='url'
            label='URL'
            rules={[
              { required: true, message: t('必填') },
              {
                validator: (_, v) =>
                  !v || /^https?:\/\//.test(v)
                    ? Promise.resolve()
                    : Promise.reject(t('URL 格式不正确')),
              },
            ]}
          />
          <Form.Input
            field='icon'
            label={t('图标（URL 或 emoji）')}
            placeholder='https://... 或 🤖'
          />
          <Form.Input field='description' label={t('描述')} />
          <Form.InputNumber field='order' label={t('排序')} />
          <Form.Switch field='enabled' label={t('启用')} />
        </Form>
      </Modal>
    </>
  );
};

export default SettingsFriendLinks;
