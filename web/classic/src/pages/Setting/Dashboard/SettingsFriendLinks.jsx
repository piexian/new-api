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

const SettingsFriendLinks = ({ options, refresh }) => {
  const { t } = useTranslation();
  const [list, setList] = useState([]);
  const [loading, setLoading] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [panelEnabled, setPanelEnabled] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState(null);
  const [formApi, setFormApi] = useState(null);

  const parseList = (raw) => {
    if (!raw) {
      setList([]);
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      setList(Array.isArray(parsed) ? parsed : []);
    } catch {
      setList([]);
    }
  };

  useEffect(() => {
    parseList(options['console_setting.friend_links']);
  }, [options['console_setting.friend_links']]);

  useEffect(() => {
    const enabledStr = options['console_setting.friend_links_enabled'];
    setPanelEnabled(enabledStr === undefined ? true : enabledStr === true || enabledStr === 'true');
  }, [options['console_setting.friend_links_enabled']]);

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

  const openAdd = () => {
    setEditing(null);
    setShowModal(true);
    setTimeout(() => {
      formApi?.setValues({
        name: '',
        url: '',
        icon: '',
        description: '',
        order: list.length,
        enabled: true,
      });
    }, 0);
  };

  const openEdit = (row) => {
    setEditing(row);
    setShowModal(true);
    setTimeout(() => {
      formApi?.setValues({ ...row });
    }, 0);
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
        setList((prev) => [...prev, values]);
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
      title: t('名称'),
      dataIndex: 'name',
    },
    {
      title: t('URL'),
      dataIndex: 'url',
      render: (text) => (
        <Typography.Text ellipsis={{ showTooltip: true }} style={{ maxWidth: 220 }}>
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
          <Button size='small' onClick={() => openEdit({ ...record, __idx: idx })}>
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
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
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
        dataSource={list.map((item, idx) => ({ ...item, key: idx }))}
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
        <Form getFormApi={setFormApi} labelPosition='top'>
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
          <Form.Input field='icon' label={t('图标 URL')} />
          <Form.Input field='description' label={t('描述')} />
          <Form.InputNumber field='order' label={t('排序')} />
          <Form.Switch field='enabled' label={t('启用')} />
        </Form>
      </Modal>
    </>
  );
};

export default SettingsFriendLinks;
