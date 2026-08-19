import React, { useState, useCallback, useRef } from 'react';
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
  Radio,
  RadioGroup,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API_ENDPOINTS, AUDIO_VOICE_OPTIONS, AUDIO_FORMAT_OPTIONS } from '../../constants/playground.constants';
import { getUserIdFromLocalStorage } from '../../helpers';

const { Text } = Typography;

// 语音组件：TTS + 音频转写
const PlaygroundAudio = ({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}) => {
  const { t } = useTranslation();
  const [audioMode, setAudioMode] = useState('tts');
  const [ttsText, setTtsText] = useState('');
  const [voice, setVoice] = useState('alloy');
  const [format, setFormat] = useState('mp3');
  const [speed, setSpeed] = useState(1);
  const [transcribeUrl, setTranscribeUrl] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [audioUrl, setAudioUrl] = useState(null);
  const [transcription, setTranscription] = useState(null);
  const audioRef = useRef(null);

  const handleTTS = useCallback(async () => {
    if (!ttsText.trim() || !selectedModel) return;
    setIsLoading(true);
    setAudioUrl(null);
    try {
      const res = await fetch(API_ENDPOINTS.AUDIO_SPEECH, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        body: JSON.stringify({
          model: selectedModel,
          group: selectedGroup,
          input: ttsText,
          voice,
          response_format: format,
          speed,
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err?.error?.message || `HTTP ${res.status}`);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      setAudioUrl(url);
    } catch (err) {
      Toast.error(err.message || String(err));
    } finally {
      setIsLoading(false);
    }
  }, [ttsText, selectedModel, selectedGroup, voice, format, speed]);

  const handleTranscribe = useCallback(async () => {
    if (!transcribeUrl.trim() || !selectedModel) return;
    setIsLoading(true);
    setTranscription(null);
    try {
      const res = await fetch(API_ENDPOINTS.AUDIO_TRANSCRIPTIONS, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        body: JSON.stringify({
          model: selectedModel,
          group: selectedGroup,
          url: transcribeUrl,
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err?.error?.message || `HTTP ${res.status}`);
      }
      const data = await res.json();
      setTranscription(data);
    } catch (err) {
      Toast.error(err.message || String(err));
    } finally {
      setIsLoading(false);
    }
  }, [transcribeUrl, selectedModel, selectedGroup]);

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: 16 }}>
      <Card>
        <Row gutter={[8, 8]}>
          <Col span={8}>
            <Text strong>{t('模式')}</Text>
            <RadioGroup
              value={audioMode}
              onChange={(e) => setAudioMode(e.target.value)}
              style={{ marginTop: 4 }}
            >
              <Radio value='tts'>{t('语音合成')}</Radio>
              <Radio value='transcribe'>{t('音频转写')}</Radio>
            </RadioGroup>
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

        {audioMode === 'tts' ? (
          <>
            <div style={{ marginTop: 12 }}>
              <TextArea
                placeholder={t('输入要转换为语音的文本...')}
                value={ttsText}
                onChange={setTtsText}
                rows={4}
              />
            </div>
            <Row gutter={[8, 8]} style={{ marginTop: 12 }}>
              <Col span={6}>
                <Text strong>{t('音色')}</Text>
                <Select
                  value={voice}
                  onChange={setVoice}
                  style={{ width: '100%', marginTop: 4 }}
                  optionList={AUDIO_VOICE_OPTIONS.map((v) => ({ value: v, label: v }))}
                />
              </Col>
              <Col span={5}>
                <Text strong>{t('格式')}</Text>
                <Select
                  value={format}
                  onChange={setFormat}
                  style={{ width: '100%', marginTop: 4 }}
                  optionList={AUDIO_FORMAT_OPTIONS.map((f) => ({ value: f, label: f }))}
                />
              </Col>
              <Col span={5}>
                <Text strong>{t('语速')}</Text>
                <Input
                  type='number'
                  min={0.25}
                  max={4}
                  step={0.25}
                  value={speed}
                  onChange={(v) => setSpeed(Number(v))}
                  style={{ marginTop: 4 }}
                />
              </Col>
              <Col span={8} style={{ display: 'flex', alignItems: 'flex-end' }}>
                <Button
                  theme='solid'
                  loading={isLoading}
                  disabled={!ttsText.trim()}
                  onClick={handleTTS}
                  style={{ width: '100%' }}
                >
                  {isLoading ? t('合成中...') : t('合成语音')}
                </Button>
              </Col>
            </Row>

            {audioUrl && (
              <div style={{ marginTop: 16 }}>
                <audio ref={audioRef} src={audioUrl} controls style={{ width: '100%' }} />
              </div>
            )}
          </>
        ) : (
          <>
            <div style={{ marginTop: 12 }}>
              <Input
                placeholder={t('输入音频文件 URL...')}
                value={transcribeUrl}
                onChange={setTranscribeUrl}
              />
            </div>
            <div style={{ marginTop: 12 }}>
              <Button
                theme='solid'
                loading={isLoading}
                disabled={!transcribeUrl.trim()}
                onClick={handleTranscribe}
              >
                {isLoading ? t('转写中...') : t('开始转写')}
              </Button>
            </div>

            {transcription && (
              <Card style={{ marginTop: 16 }}>
                <Text strong>{t('转写结果')}</Text>
                <div style={{ marginTop: 8 }}>
                  <Text>{transcription.text}</Text>
                </div>
                {transcription.segments && (
                  <div style={{ marginTop: 8, maxHeight: 300, overflow: 'auto' }}>
                    {transcription.segments.map((seg, i) => (
                      <div key={i} style={{ marginBottom: 4 }}>
                        <Text type='tertiary' size='small'>
                          [{seg.start?.toFixed(1)}s - {seg.end?.toFixed(1)}s]
                        </Text>{' '}
                        <Text>{seg.text}</Text>
                      </div>
                    ))}
                  </div>
                )}
              </Card>
            )}
          </>
        )}
      </Card>
    </div>
  );
};

export default PlaygroundAudio;
