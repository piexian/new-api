import React, { useState, useCallback } from 'react';
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
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API_ENDPOINTS, IMAGE_SIZE_OPTIONS, IMAGE_QUALITY_OPTIONS, IMAGE_STYLE_OPTIONS } from '../../constants/playground.constants';
import { getUserIdFromLocalStorage } from '../../helpers';

const { Title, Text } = Typography;

const PlaygroundImage = ({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}) => {
  const { t } = useTranslation();
  const [imageInterface, setImageInterface] = useState('generations');
  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState('1024x1024');
  const [quality, setQuality] = useState('standard');
  const [style, setStyle] = useState('vivid');
  const [count, setCount] = useState(1);
  const [editImage, setEditImage] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [results, setResults] = useState(null);

  const handleSubmit = useCallback(async () => {
    if (!prompt.trim() || !selectedModel) return;
    setIsLoading(true);
    setResults(null);
    try {
      const endpoint =
        imageInterface === 'generations'
          ? API_ENDPOINTS.IMAGES_GENERATIONS
          : API_ENDPOINTS.IMAGES_EDITS;
      const body = {
        model: selectedModel,
        group: selectedGroup,
        prompt,
        n: count,
        size,
        response_format: 'url',
      };
      if (imageInterface === 'generations') {
        body.quality = quality;
        body.style = style;
      } else {
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
      setResults(data);
    } catch (err) {
      Toast.error(err.message || String(err));
    } finally {
      setIsLoading(false);
    }
  }, [prompt, selectedModel, selectedGroup, imageInterface, count, size, quality, style, editImage]);

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: 16 }}>
      <Card>
        <Row gutter={[8, 8]}>
          <Col span={8}>
            <Text strong>{t('接口')}</Text>
            <Select
              value={imageInterface}
              onChange={setImageInterface}
              style={{ width: '100%', marginTop: 4 }}
              optionList={[
                { value: 'generations', label: t('图片生成') },
                { value: 'edits', label: t('图片编辑') },
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
            placeholder={t('描述你想生成的图片...')}
            value={prompt}
            onChange={setPrompt}
            rows={4}
          />
        </div>

        {imageInterface === 'edits' && (
          <div style={{ marginTop: 8 }}>
            <Input
              placeholder={t('输入图片 URL 或粘贴 base64...')}
              value={editImage}
              onChange={setEditImage}
            />
          </div>
        )}

        <Row gutter={[8, 8]} style={{ marginTop: 12 }}>
          <Col span={5}>
            <Text strong>{t('尺寸')}</Text>
            <Select value={size} onChange={setSize} style={{ width: '100%', marginTop: 4 }} optionList={IMAGE_SIZE_OPTIONS.map((s) => ({ value: s, label: s }))} />
          </Col>
          {imageInterface === 'generations' && (
            <>
              <Col span={5}>
                <Text strong>{t('质量')}</Text>
                <Select value={quality} onChange={setQuality} style={{ width: '100%', marginTop: 4 }} optionList={IMAGE_QUALITY_OPTIONS.map((q) => ({ value: q, label: q }))} />
              </Col>
              <Col span={5}>
                <Text strong>{t('风格')}</Text>
                <Select value={style} onChange={setStyle} style={{ width: '100%', marginTop: 4 }} optionList={IMAGE_STYLE_OPTIONS.map((s) => ({ value: s, label: s }))} />
              </Col>
            </>
          )}
          <Col span={4}>
            <Text strong>{t('数量')}</Text>
            <Input type='number' min={1} max={4} value={count} onChange={(v) => setCount(Number(v))} style={{ marginTop: 4 }} />
          </Col>
          <Col span={5} style={{ display: 'flex', alignItems: 'flex-end' }}>
            <Button theme='solid' loading={isLoading} disabled={!prompt.trim()} onClick={handleSubmit} style={{ width: '100%' }}>
              {isLoading ? t('生成中...') : t('生成')}
            </Button>
          </Col>
        </Row>

        {results && (
          <Row gutter={[8, 8]} style={{ marginTop: 16 }}>
            {results.data?.map((img, i) => (
              <Col key={i} span={8}>
                <Card cover={img.url ? <img src={img.url} alt='' style={{ objectFit: 'cover' }} /> : img.b64_json ? <img src={`data:image/png;base64,${img.b64_json}`} alt='' /> : null}>
                  {img.revised_prompt && <Text type='tertiary' size='small'>{img.revised_prompt}</Text>}
                </Card>
              </Col>
            ))}
          </Row>
        )}
      </Card>
    </div>
  );
};

export default PlaygroundImage;
