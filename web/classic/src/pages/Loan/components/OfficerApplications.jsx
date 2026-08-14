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
import {
  Avatar,
  Button,
  Card,
  Modal,
  Rating,
  Select,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Bot, Plus, Send } from 'lucide-react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../../helpers';
import QueryError from './QueryError';

const PAGE_SIZE = 10;
const TOPIC_KEYS = ['credit', 'rate', 'grace', 'other'];

const topicLabel = (t, topic) => {
  if (topic === 'credit') return t('提升额度');
  if (topic === 'rate') return t('降低利率');
  if (topic === 'grace') return t('延长免息期');
  return t('其他');
};

const statusTag = (t, status) =>
  status === 'open' ? (
    <Tag color='green' size='small'>
      {t('进行中')}
    </Tag>
  ) : (
    <Tag size='small'>{t('已关闭')}</Tag>
  );

const NewApplicationModal = ({ t, visible, onClose, onCreated }) => {
  const [topic, setTopic] = useState('credit');
  const [content, setContent] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!content.trim()) {
      showError(t('请描述您的申请'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/loan/applications', {
        topic,
        content: content.trim(),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('申请已提交'));
        setContent('');
        setTopic('credit');
        onClose();
        onCreated();
        return;
      }
      // 首轮 AI 对话失败时工单可能已创建：刷新列表并引导到详情继续，
      // 具体错误信息由后端返回
      if (message) showError(message);
      onCreated();
      showInfo(t('如工单已创建，请从列表中打开继续对话。'));
    } catch {
      // 网络异常同样无法确定工单是否已创建：刷新列表 + 中性引导
      onCreated();
      showInfo(t('如工单已创建，请从列表中打开继续对话。'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={t('新建申请')}
      visible={visible}
      onCancel={onClose}
      footer={
        <>
          <Button theme='outline' onClick={onClose}>
            {t('取消')}
          </Button>
          <Button
            type='primary'
            theme='solid'
            onClick={handleSubmit}
            loading={submitting}
            disabled={submitting || !content.trim()}
          >
            {submitting ? t('提交中...') : t('提交')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <Typography.Text type='secondary' size='small'>
          {t('描述您的诉求，AI 专员将与您沟通审核。')}
        </Typography.Text>
        <div>
          <Typography.Text strong className='block mb-1'>
            {t('申请类型')}
          </Typography.Text>
          <Select
            value={topic}
            onChange={(v) => setTopic(v)}
            style={{ width: '100%' }}
            optionList={TOPIC_KEYS.map((key) => ({
              label: topicLabel(t, key),
              value: key,
            }))}
          />
        </div>
        <div>
          <Typography.Text strong className='block mb-1'>
            {t('申请详情')}
          </Typography.Text>
          <TextArea
            value={content}
            onChange={(v) => setContent(v)}
            placeholder={t('请描述您的申请...')}
            rows={5}
          />
        </div>
      </div>
    </Modal>
  );
};

const ApplicationDetailModal = ({
  t,
  applicationId,
  visible,
  onClose,
  onChanged,
}) => {
  const [detail, setDetail] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [reply, setReply] = useState('');
  const [sending, setSending] = useState(false);
  const [rating, setRating] = useState(0);
  const [ratingComment, setRatingComment] = useState('');
  const [ratingSubmitting, setRatingSubmitting] = useState(false);

  const fetchDetail = async () => {
    if (!applicationId) return;
    setLoading(true);
    setError('');
    try {
      const res = await API.get(`/api/user/loan/applications/${applicationId}`);
      const { success, message, data } = res.data;
      if (success) {
        setDetail(data);
      } else {
        // 查询失败展示后端 message + 重试，不留空白
        setError(message || t('获取申请详情失败'));
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setError(t('获取申请详情失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible && applicationId) {
      fetchDetail();
    }
  }, [visible, applicationId]);

  const application = detail?.application;
  const messages = detail?.messages || [];
  const isOpen = application?.status === 'open';
  const showRatingWidget =
    application?.status === 'closed' && application.rating === 0;

  const handleSend = async () => {
    if (!reply.trim()) return;
    setSending(true);
    try {
      const res = await API.post(
        `/api/user/loan/applications/${applicationId}/reply`,
        { content: reply.trim() },
      );
      const { success, message } = res.data;
      if (success) {
        setReply('');
        fetchDetail();
        onChanged();
      } else {
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    } finally {
      setSending(false);
    }
  };

  const handleRate = async () => {
    if (rating < 1) return;
    setRatingSubmitting(true);
    try {
      const res = await API.post(
        `/api/user/loan/applications/${applicationId}/rate`,
        { rating, comment: ratingComment.trim() },
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('感谢您的反馈'));
        fetchDetail();
        onChanged();
      } else {
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    } finally {
      setRatingSubmitting(false);
    }
  };

  return (
    <Modal
      title={
        application
          ? `${topicLabel(t, application.topic)} #${application.id}`
          : t('申请详情')
      }
      visible={visible}
      onCancel={onClose}
      footer={null}
      width={640}
    >
      {error ? (
        <QueryError t={t} message={error} onRetry={fetchDetail} />
      ) : loading && !detail ? (
        <div className='py-8 text-center text-sm text-gray-500'>
          {t('加载中...')}
        </div>
      ) : !application ? null : (
        <div className='space-y-4'>
          <div className='flex items-center gap-2'>
            {statusTag(t, application.status)}
            <Typography.Text type='tertiary' size='small'>
              {timestamp2string(application.created_at)}
            </Typography.Text>
          </div>

          {/* 对话串 */}
          <div className='max-h-[50vh] space-y-3 overflow-y-auto rounded-lg border p-3'>
            {messages.length === 0 ? (
              <div className='py-6 text-center text-sm text-gray-500'>
                {t('暂无消息，回复即可开始对话。')}
              </div>
            ) : (
              messages.map((msg) => {
                if (msg.role === 'system') {
                  return (
                    <div
                      key={msg.id}
                      className='text-center text-xs text-gray-400'
                    >
                      {msg.content}
                    </div>
                  );
                }
                const isUser = msg.role === 'user';
                return (
                  <div
                    key={msg.id}
                    className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}
                  >
                    <div
                      className={`max-w-[85%] rounded-lg px-3 py-2 text-sm break-words whitespace-pre-wrap ${
                        isUser
                          ? 'bg-blue-500 text-white'
                          : 'bg-slate-100 dark:bg-slate-700'
                      }`}
                    >
                      {msg.content}
                      <div
                        className={`mt-1 text-[10px] ${
                          isUser
                            ? 'text-blue-100'
                            : 'text-gray-400 dark:text-gray-500'
                        }`}
                      >
                        {timestamp2string(msg.created_at)}
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* open 工单可继续回复 */}
          {isOpen ? (
            <div className='flex items-end gap-2'>
              <TextArea
                value={reply}
                onChange={(v) => setReply(v)}
                placeholder={t('输入回复...')}
                rows={2}
                className='flex-1'
              />
              <Button
                type='primary'
                theme='solid'
                icon={<Send size={14} />}
                onClick={handleSend}
                loading={sending}
                disabled={sending || !reply.trim()}
                aria-label={t('发送')}
              />
            </div>
          ) : null}

          {/* closed 且未评分时显示评分组件 */}
          {showRatingWidget ? (
            <div className='space-y-3 rounded-lg border p-3'>
              <Typography.Text strong>{t('请为本次服务评分')}</Typography.Text>
              <Rating value={rating} onChange={setRating} />
              <TextArea
                value={ratingComment}
                onChange={(v) => setRatingComment(v)}
                placeholder={t('评语（可选）...')}
                rows={2}
              />
              <Button
                type='primary'
                theme='solid'
                size='small'
                onClick={handleRate}
                loading={ratingSubmitting}
                disabled={ratingSubmitting || rating < 1}
              >
                {ratingSubmitting ? t('提交中...') : t('提交评分')}
              </Button>
            </div>
          ) : null}

          {!isOpen && application.rating > 0 ? (
            <div className='space-y-1 rounded-lg border p-3'>
              <Typography.Text
                type='tertiary'
                size='small'
                className='uppercase tracking-wider'
              >
                {t('您的评分')}
              </Typography.Text>
              <Rating value={application.rating} disabled size='small' />
              {application.rating_comment ? (
                <Typography.Text type='secondary' size='small'>
                  {application.rating_comment}
                </Typography.Text>
              ) : null}
            </div>
          ) : null}
        </div>
      )}
    </Modal>
  );
};

const OfficerApplications = ({ t }) => {
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [newModalVisible, setNewModalVisible] = useState(false);
  const [detailId, setDetailId] = useState(null);

  const fetchList = async (p) => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/user/loan/applications', {
        params: { p, page_size: PAGE_SIZE },
      });
      const { success, message, data } = res.data;
      if (success) {
        setItems(data?.items || []);
        setTotal(data?.total || 0);
      } else {
        // 查询失败展示后端 message + 重试，不伪装成空数据
        setError(message || t('获取申请列表失败'));
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setError(t('获取申请列表失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList(page);
  }, [page]);

  const refreshList = () => fetchList(page);

  const columns = useMemo(
    () => [
      {
        title: t('工单'),
        dataIndex: 'topic',
        render: (v, record) => `${topicLabel(t, v)} #${record.id}`,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => statusTag(t, v),
      },
      {
        title: t('评分'),
        dataIndex: 'rating',
        render: (v) =>
          v > 0 ? <Rating value={v} disabled size='small' /> : '-',
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        render: (v) => timestamp2string(v),
      },
    ],
    [t],
  );

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center justify-between mb-4 gap-3'>
        <div className='flex items-center min-w-0'>
          <Avatar size='small' color='cyan' className='mr-3 shadow-md'>
            <Bot size={16} />
          </Avatar>
          <div className='min-w-0'>
            <Typography.Text className='text-lg font-medium'>
              {t('AI 贷款专员')}
            </Typography.Text>
            <div className='text-xs text-gray-500 dark:text-gray-400'>
              {t('可申请提升额度、降低利率或延长免息期')}
            </div>
          </div>
        </div>
        <Button
          type='primary'
          theme='solid'
          size='small'
          icon={<Plus size={14} />}
          onClick={() => setNewModalVisible(true)}
          className='shrink-0'
        >
          {t('新建申请')}
        </Button>
      </div>

      {error ? (
        <QueryError t={t} message={error} onRetry={() => fetchList(page)} />
      ) : (
        <Table
          size='small'
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          empty={t('暂无申请')}
          scroll={{ x: true }}
          onRow={(record) => ({
            onClick: () => setDetailId(record.id),
            style: { cursor: 'pointer' },
          })}
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onPageChange: (p) => setPage(p),
          }}
        />
      )}

      <NewApplicationModal
        t={t}
        visible={newModalVisible}
        onClose={() => setNewModalVisible(false)}
        onCreated={refreshList}
      />

      {/* key 保证切换工单时回复/评分等本地状态重置，避免跨工单泄漏 */}
      <ApplicationDetailModal
        key={detailId ?? 'closed'}
        t={t}
        applicationId={detailId}
        visible={detailId !== null}
        onClose={() => setDetailId(null)}
        onChanged={refreshList}
      />
    </Card>
  );
};

export default OfficerApplications;
