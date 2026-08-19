import React, { useState, useCallback, useEffect, useRef } from 'react';
import {
  Card,
  Select,
  Button,
  Input,
  TextArea,
  Toast,
  Typography,
  Row,
  Col,
  Spin,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API_ENDPOINTS } from '../../constants/playground.constants';
import { getUserIdFromLocalStorage } from '../../helpers';

const { Text } = Typography;

// 视频生成/编辑组件，异步任务轮询
const PlaygroundVideo = ({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}) => {
  const { t } = useTranslation();
  const [videoInterface, setVideoInterface] = useState('generations');
  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState('1024x1024');
  const [duration, setDuration] = useState(5);
  const [editImage, setEditImage] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [taskId, setTaskId] = useState(null);
  const [result, setResult] = useState(null);
  const pollRef = useRef(null);

  // 轮询任务状态
  const pollTask = useCallback(
    async (id) => {
      try {
        const res = await fetch(`/api/task/${id}`, {
          headers: { 'New-Api-User': getUserIdFromLocalStorage() },
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();
        if (data.status === 'succeeded' || data.status === 'success') {
          setResult(data);
          setIsLoading(false);
          setTaskId(null);
        } else if (data.status === 'failed') {
          Toast.error(data.error || t('任务失败'));
          setIsLoading(false);
          setTaskId(null);
        }
      } catch {
        // 轮询失败，继续重试
      }
    },
    [t],
  );

  useEffect(() => {
    if (taskId) {
      pollRef.current = setInterval(() => pollTask(taskId), 3000);
    }
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [taskId, pollTask]);

  const handleSubmit = useCallback(async () => {
    if (!prompt.trim() || !selectedModel) return;
    setIsLoading(true);
    setResult(null);
    try {
      const endpoint =
        videoInterface === 'generations'
          ? API_ENDPOINTS.VIDEOS_GENERATIONS
          : API_ENDPOINTS.VIDEOS_EDITS;
      const body = {
        model: selectedModel,
        group: selectedGroup,
        prompt,
        size,
        duration,
      };
      if (videoInterface === 'edits') {
        body.image = editImage;
      }
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err?.error?.message || `HTTP ${res.status}`);
      }
      const data = await res.json();
      // 异步任务返回 task_id
      if (data.task_id || data.id) {
        setTaskId(data.task_id || data.id);
      } else if (data.data?.[0]?.url) {
        // 同步返回结果
        setResult(data);
        setIsLoading(false);
      }
    } catch (err) {
      Toast.error(err.message || String(err));
      setIsLoading(false);
    }
  }, [prompt, selectedModel, selectedGroup, videoInterface, size, duration, editImage]);

  const handleStop = useCallback(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    setIsLoading(false);
    setTaskId(null);
  }, []);

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: 16 }}>
      <Card>
        <Row gutter={[8, 8]}>
          <Col span={8}>
            <Text strong>{t('接口')}</Text>
            <Select
              value={videoInterface}
              onChange={setVideoInterface}
              style={{ width: '100%', marginTop: 4 }}
              optionList={[
                { value: 'generations', label: t('视频生成') },
                { value: 'edits', label: t('视频编辑') },
              ]}
            />
          </Col>
          <Col span={10}>
            <Text strong>{t('模型')}</Text>
            <Select
              value={selectedModel}
              onChange={onModelChange}
              style={{ width: '100%', marginTop: 4 }}
              filter
              optionList={models.map((m) => ({ value: m.value, label: m.label }))}
            />
          </Col>
          <Col span={6}>
            <Text strong>{t('分组')}</Text>
            <Select
              value={selectedGroup}
              onChange={onGroupChange}
              style={{ width: '100%', marginTop: 4 }}
              optionList={groups.map((g) => ({ value: g.value, label: g.label }))}
            />
          </Col>
        </Row>

        <div style={{ marginTop: 12 }}>
          <TextArea
            placeholder={t('描述你想生成的视频...')}
            value={prompt}
            onChange={setPrompt}
            rows={4}
          />
        </div>

        {videoInterface === 'edits' && (
          <div style={{ marginTop: 8 }}>
            <Input
              placeholder={t('输入图片 URL...')}
              value={editImage}
              onChange={setEditImage}
            />
          </div>
        )}

        <Row gutter={[8, 8]} style={{ marginTop: 12 }}>
          <Col span={6}>
            <Text strong>{t('尺寸')}</Text>
            <Select
              value={size}
              onChange={setSize}
              style={{ width: '100%', marginTop: 4 }}
              optionList={[
                { value: '1024x1024', label: '1024x1024' },
                { value: '1280x720', label: '1280x720' },
                { value: '720x1280', label: '720x1280' },
              ]}
            />
          </Col>
          <Col span={6}>
            <Text strong>{t('时长(秒)')}</Text>
            <Input
              type='number'
              min={1}
              max={30}
              value={duration}
              onChange={(v) => setDuration(Number(v))}
              style={{ marginTop: 4 }}
            />
          </Col>
          <Col span={6} style={{ display: 'flex', alignItems: 'flex-end' }}>
            {isLoading ? (
              <Button theme='solid' type='danger' onClick={handleStop} style={{ width: '100%' }}>
                {t('停止')}
              </Button>
            ) : (
              <Button
                theme='solid'
                disabled={!prompt.trim()}
                onClick={handleSubmit}
                style={{ width: '100%' }}
              >
                {t('生成')}
              </Button>
            )}
          </Col>
        </Row>

        {isLoading && (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin size='large' />
            <div style={{ marginTop: 12 }}>
              <Text type='tertiary'>{t('视频生成中，请耐心等待...')}</Text>
            </div>
          </div>
        )}

        {result && (
          <div style={{ marginTop: 16 }}>
            {result.data?.[0]?.url ? (
              <video
                src={result.data[0].url}
                controls
                style={{ width: '100%', maxHeight: 480, objectFit: 'contain' }}
              />
            ) : result.url ? (
              <video
                src={result.url}
                controls
                style={{ width: '100%', maxHeight: 480, objectFit: 'contain' }}
              />
            ) : (
              <Text type='tertiary'>{t('无预览')}</Text>
            )}
          </div>
        )}
      </Card>
    </div>
  );
};

export default PlaygroundVideo;
