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

import React, { useState, useCallback, useMemo, useRef, useEffect } from 'react';
import {
  Button,
  Input,
  InputNumber,
  Typography,
  Popconfirm,
  Banner,
} from '@douyinfe/semi-ui';
import { IconPlus, IconDelete } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import CardTable from '../../../../components/common/ui/CardTable';

const { Text } = Typography;

const MAX_LIMIT = 2147483647;

let _idCounter = 0;
const uid = () => `gl_${++_idCounter}`;

// mode='rate' 对应 {"组名": [最多请求次数, 最多成功次数]}；mode='concurrency' 对应 {"组名": 最大并发数}
function parseRows(str, mode) {
  if (!str || !str.trim()) return { rows: [], valid: true };
  let parsed;
  try {
    parsed = JSON.parse(str);
  } catch {
    return { rows: [], valid: false };
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { rows: [], valid: false };
  }
  const rows = [];
  for (const [name, val] of Object.entries(parsed)) {
    if (mode === 'concurrency') {
      if (typeof val !== 'number') return { rows: [], valid: false };
      rows.push({ _id: uid(), name, maxConcurrent: val });
    } else {
      if (
        !Array.isArray(val) ||
        val.length !== 2 ||
        typeof val[0] !== 'number' ||
        typeof val[1] !== 'number'
      ) {
        return { rows: [], valid: false };
      }
      rows.push({ _id: uid(), name, maxRequests: val[0], maxSuccess: val[1] });
    }
  }
  return { rows, valid: true };
}

function serializeRows(rows, mode) {
  const obj = {};
  rows.forEach((row) => {
    if (!row.name) return;
    obj[row.name] =
      mode === 'concurrency'
        ? (row.maxConcurrent ?? 0)
        : [row.maxRequests ?? 0, row.maxSuccess ?? 1];
  });
  return JSON.stringify(obj, null, 2);
}

export default function GroupLimitEditor({ value, onChange, mode = 'rate' }) {
  const { t } = useTranslation();
  const isConcurrency = mode === 'concurrency';

  const [parsed, setParsed] = useState(() => parseRows(value, mode));

  // 外部 JSON 变化（JSON 模式编辑后切回、选项刷新）时重建行；
  // 与自身上次序列化结果相同则跳过，避免输入过程中光标跳动
  const lastSerializedRef = useRef(serializeRows(parsed.rows, mode));
  useEffect(() => {
    if (value !== lastSerializedRef.current) {
      const next = parseRows(value, mode);
      lastSerializedRef.current = value;
      setParsed(next);
    }
  }, [value, mode]);

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const emitAndSet = useCallback(
    (updater) => {
      setParsed((prev) => {
        const nextRows =
          typeof updater === 'function' ? updater(prev.rows) : updater;
        const serialized = serializeRows(nextRows, mode);
        lastSerializedRef.current = serialized;
        onChangeRef.current?.(serialized);
        return { rows: nextRows, valid: true };
      });
    },
    [mode],
  );

  const updateRow = useCallback(
    (id, field, val) => {
      emitAndSet((rows) =>
        rows.map((r) => (r._id === id ? { ...r, [field]: val } : r)),
      );
    },
    [emitAndSet],
  );

  const addRow = useCallback(() => {
    emitAndSet((rows) => {
      const existingNames = new Set(rows.map((r) => r.name));
      let counter = 1;
      let newName = `group_${counter}`;
      while (existingNames.has(newName)) {
        counter++;
        newName = `group_${counter}`;
      }
      return [
        ...rows,
        isConcurrency
          ? { _id: uid(), name: newName, maxConcurrent: 0 }
          : { _id: uid(), name: newName, maxRequests: 0, maxSuccess: 1 },
      ];
    });
  }, [emitAndSet, isConcurrency]);

  const removeRow = useCallback(
    (id) => {
      emitAndSet((rows) => rows.filter((r) => r._id !== id));
    },
    [emitAndSet],
  );

  const duplicateNames = useMemo(() => {
    const counts = {};
    parsed.rows.forEach((r) => {
      if (r.name) counts[r.name] = (counts[r.name] || 0) + 1;
    });
    return new Set(Object.keys(counts).filter((k) => counts[k] > 1));
  }, [parsed.rows]);

  // 通过 ref 让列渲染函数读到最新重复集合，避免 columns 每次击键重建导致光标重置
  const duplicateNamesRef = useRef(duplicateNames);
  duplicateNamesRef.current = duplicateNames;

  const columns = useMemo(() => {
    const valueColumns = isConcurrency
      ? [
          {
            title: t('最大并发数'),
            dataIndex: 'maxConcurrent',
            key: 'maxConcurrent',
            width: 160,
            render: (_, record) => (
              <InputNumber
                size='small'
                min={0}
                max={MAX_LIMIT}
                step={1}
                value={record.maxConcurrent}
                style={{ width: '100%' }}
                onChange={(v) => updateRow(record._id, 'maxConcurrent', v ?? 0)}
              />
            ),
          },
        ]
      : [
          {
            title: t('最多请求次数'),
            dataIndex: 'maxRequests',
            key: 'maxRequests',
            width: 150,
            render: (_, record) => (
              <InputNumber
                size='small'
                min={0}
                max={MAX_LIMIT}
                step={1}
                value={record.maxRequests}
                style={{ width: '100%' }}
                onChange={(v) => updateRow(record._id, 'maxRequests', v ?? 0)}
              />
            ),
          },
          {
            title: t('最多成功次数'),
            dataIndex: 'maxSuccess',
            key: 'maxSuccess',
            width: 150,
            render: (_, record) => (
              <InputNumber
                size='small'
                min={1}
                max={MAX_LIMIT}
                step={1}
                value={record.maxSuccess}
                style={{ width: '100%' }}
                onChange={(v) => updateRow(record._id, 'maxSuccess', v ?? 1)}
              />
            ),
          },
        ];

    return [
      {
        title: t('分组名称'),
        dataIndex: 'name',
        key: 'name',
        width: 180,
        render: (_, record) => (
          <Input
            size='small'
            value={record.name}
            status={
              duplicateNamesRef.current.has(record.name) ? 'warning' : undefined
            }
            onChange={(v) => updateRow(record._id, 'name', v)}
          />
        ),
      },
      ...valueColumns,
      {
        title: '',
        key: 'actions',
        width: 50,
        render: (_, record) => (
          <Popconfirm
            title={t('确认删除该分组？')}
            onConfirm={() => removeRow(record._id)}
            position='left'
          >
            <Button
              icon={<IconDelete />}
              type='danger'
              theme='borderless'
              size='small'
            />
          </Popconfirm>
        ),
      },
    ];
  }, [t, isConcurrency, updateRow, removeRow]);

  if (!parsed.valid) {
    return (
      <Banner
        type='warning'
        description={t('当前 JSON 无法解析，请切换到 JSON 模式修复后再使用可视化编辑。')}
      />
    );
  }

  return (
    <div>
      <CardTable
        columns={columns}
        dataSource={parsed.rows}
        rowKey='_id'
        hidePagination
        size='small'
        empty={<Text type='tertiary'>{t('暂无配置，点击下方按钮添加')}</Text>}
      />
      <div className='mt-3 flex justify-center'>
        <Button icon={<IconPlus />} theme='outline' onClick={addRow}>
          {t('添加分组')}
        </Button>
      </div>
      {duplicateNames.size > 0 && (
        <Text type='warning' size='small' className='mt-2 block'>
          {t('存在重复的分组名称：')}
          {Array.from(duplicateNames).join(', ')}
        </Text>
      )}
      <Text type='tertiary' size='small' className='mt-2 block'>
        {isConcurrency
          ? t('按账号+分组限制同时进行中的请求数，0 表示不限制。')
          : t(
              '最多请求次数含失败请求（0 代表不限制），最多成功次数只计成功请求；限制周期使用上方全局值。',
            )}
      </Text>
    </div>
  );
}
