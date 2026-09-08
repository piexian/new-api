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

import React, {
  useContext,
  useEffect,
  useCallback,
  useRef,
  useState,
} from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Layout, Toast, Modal, Button, SideSheet } from '@douyinfe/semi-ui';
import { Settings, Code } from 'lucide-react';
import './playground.css';

// Context
import { UserContext } from '../../context/User';
import { useIsMobile } from '../../hooks/common/useIsMobile';

// hooks
import { usePlaygroundState } from '../../hooks/playground/usePlaygroundState';
import { useMessageActions } from '../../hooks/playground/useMessageActions';
import { useApiRequest } from '../../hooks/playground/useApiRequest';
import { useSyncMessageAndCustomBody } from '../../hooks/playground/useSyncMessageAndCustomBody';
import { useMessageEdit } from '../../hooks/playground/useMessageEdit';
import { useDataLoader } from '../../hooks/playground/useDataLoader';

// Constants and utils
import {
  MESSAGE_ROLES,
  ERROR_MESSAGES,
  CHAT_INTERFACE_OPTIONS,
  API_ENDPOINTS,
  PLAYGROUND_MODES,
} from '../../constants/playground.constants';
import {
  getLogo,
  stringToColor,
  buildMessageContent,
  createMessage,
  createLoadingAssistantMessage,
  getTextContent,
  buildApiPayload,
  encodeToBase64,
} from '../../helpers';

// Components
import {
  OptimizedSettingsPanel,
  OptimizedDebugPanel,
  OptimizedMessageContent,
  OptimizedMessageActions,
} from '../../components/playground/OptimizedComponents';
import ChatArea from '../../components/playground/ChatArea';
import { PlaygroundProvider } from '../../contexts/PlaygroundContext';
import PlaygroundImage from '../../components/playground/PlaygroundImage';
import PlaygroundVideo from '../../components/playground/PlaygroundVideo';
import PlaygroundAudio from '../../components/playground/PlaygroundAudio';

// 生成头像
const generateAvatarDataUrl = (username) => {
  if (!username) {
    return 'https://lf3-static.bytednsdoc.com/obj/eden-cn/ptlz_zlp/ljhwZthlaukjlkulzlp/docs-icon.png';
  }
  const firstLetter = username[0].toUpperCase();
  const bgColor = stringToColor(username);
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
      <circle cx="16" cy="16" r="16" fill="${bgColor}" />
      <text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" font-size="16" fill="#ffffff" font-family="sans-serif">${firstLetter}</text>
    </svg>
  `;
  return `data:image/svg+xml;base64,${encodeToBase64(svg)}`;
};

const Playground = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const isMobile = useIsMobile();
  const styleState = { isMobile };
  const [searchParams] = useSearchParams();

  const state = usePlaygroundState();
  const {
    inputs,
    parameterEnabled,
    showDebugPanel,
    customRequestMode,
    customRequestBody,
    showSettings,
    models,
    groups,
    status,
    message,
    debugData,
    activeDebugTab,
    previewPayload,
    sseSourceRef,
    chatRef,
    handleInputChange,
    handleParameterToggle,
    debouncedSaveConfig,
    saveMessagesImmediately,
    handleConfigImport,
    handleConfigReset,
    setShowSettings,
    setModels,
    setGroups,
    setStatus,
    setMessage,
    setDebugData,
    setActiveDebugTab,
    setPreviewPayload,
    setShowDebugPanel,
    setCustomRequestMode,
    setCustomRequestBody,
  } = state;

  const [mode, setMode] = useState('chat');

  const { sendRequest, onStopGenerator } = useApiRequest(
    setMessage,
    setDebugData,
    setActiveDebugTab,
    sseSourceRef,
    saveMessagesImmediately,
  );

  // 数据加载
  useDataLoader(userState, inputs, handleInputChange, setModels, setGroups);

  // 消息编辑
  const {
    editingMessageId,
    editValue,
    setEditValue,
    handleMessageEdit,
    handleEditSave,
    handleEditCancel,
  } = useMessageEdit(
    setMessage,
    inputs,
    parameterEnabled,
    sendRequest,
    saveMessagesImmediately,
  );

  // 消息和自定义请求体同步
  const { syncMessageToCustomBody, syncCustomBodyToMessage } =
    useSyncMessageAndCustomBody(
      customRequestMode,
      customRequestBody,
      message,
      inputs,
      setCustomRequestBody,
      setMessage,
      debouncedSaveConfig,
    );

  // 角色信息
  const roleInfo = {
    user: {
      name: userState?.user?.username || 'User',
      avatar: generateAvatarDataUrl(userState?.user?.username),
    },
    assistant: {
      name: 'Assistant',
      avatar: getLogo(),
    },
    system: {
      name: 'System',
      avatar: getLogo(),
    },
  };

  // 消息操作
  const messageActions = useMessageActions(
    message,
    setMessage,
    onMessageSend,
    saveMessagesImmediately,
  );

  // 构建预览请求体
  const constructPreviewPayload = useCallback(() => {
    try {
      // 如果是自定义请求体模式且有自定义内容，直接返回解析后的自定义请求体
      if (customRequestMode && customRequestBody && customRequestBody.trim()) {
        try {
          return JSON.parse(customRequestBody);
        } catch (parseError) {
          console.warn('自定义请求体JSON解析失败，回退到默认预览:', parseError);
        }
      }

      // 默认预览逻辑
      let messages = [...message];

      // 如果存在用户消息
      if (
        !(
          messages.length === 0 ||
          messages.every((msg) => msg.role !== MESSAGE_ROLES.USER)
        )
      ) {
        // 处理最后一个用户消息的图片
        for (let i = messages.length - 1; i >= 0; i--) {
          if (messages[i].role === MESSAGE_ROLES.USER) {
            if (inputs.imageEnabled && inputs.imageUrls) {
              const validImageUrls = inputs.imageUrls.filter(
                (url) => url.trim() !== '',
              );
              if (validImageUrls.length > 0) {
                const textContent = getTextContent(messages[i]) || '示例消息';
                const content = buildMessageContent(
                  textContent,
                  validImageUrls,
                  true,
                );
                messages[i] = { ...messages[i], content };
              }
            }
            break;
          }
        }
      }

      return buildApiPayload(messages, null, inputs, parameterEnabled);
    } catch (error) {
      console.error('构造预览请求体失败:', error);
      return null;
    }
  }, [inputs, parameterEnabled, message, customRequestMode, customRequestBody]);

  // 根据接口类型获取 endpoint
  const getChatEndpoint = (chatInterface) => {
    switch (chatInterface) {
      case 'openai-response':
        return API_ENDPOINTS.RESPONSES;
      case 'anthropic':
        return API_ENDPOINTS.MESSAGES;
      case 'gemini':
        return `${API_ENDPOINTS.RESPONSES}?format=gemini`;
      default:
        return undefined;
    }
  };

  // 发送消息
  function onMessageSend(content, attachment) {
    console.log('attachment: ', attachment);

    const chatEndpoint = getChatEndpoint(inputs.chatInterface);

    // 创建用户消息和加载消息
    const userMessage = createMessage(MESSAGE_ROLES.USER, content);
    const loadingMessage = createLoadingAssistantMessage();

    // 自定义请求体模式
    if (customRequestMode && customRequestBody) {
      try {
        const customPayload = JSON.parse(customRequestBody);

        setMessage((prevMessage) => {
          const newMessages = [...prevMessage, userMessage, loadingMessage];
          sendRequest(
            customPayload,
            customPayload.stream !== false,
            chatEndpoint,
          );
          setTimeout(() => saveMessagesImmediately(newMessages), 0);
          return newMessages;
        });
        return;
      } catch (error) {
        console.error('自定义请求体JSON解析失败:', error);
        Toast.error(ERROR_MESSAGES.JSON_PARSE_ERROR);
        return;
      }
    }

    // 默认模式
    const validImageUrls = inputs.imageUrls.filter((url) => url.trim() !== '');
    const messageContent = buildMessageContent(
      content,
      validImageUrls,
      inputs.imageEnabled,
    );
    const userMessageWithImages = createMessage(
      MESSAGE_ROLES.USER,
      messageContent,
    );

    setMessage((prevMessage) => {
      const newMessages = [...prevMessage, userMessageWithImages];
      const payload = buildApiPayload(
        newMessages,
        null,
        inputs,
        parameterEnabled,
      );
      sendRequest(payload, inputs.stream, chatEndpoint);

      // 禁用图片模式
      if (inputs.imageEnabled) {
        setTimeout(() => {
          handleInputChange('imageEnabled', false);
        }, 100);
      }

      const messagesWithLoading = [...newMessages, loadingMessage];
      setTimeout(() => saveMessagesImmediately(messagesWithLoading), 0);
      return messagesWithLoading;
    });
  }

  // 切换推理展开状态
  const toggleReasoningExpansion = useCallback(
    (messageId) => {
      setMessage((prevMessages) =>
        prevMessages.map((msg) =>
          msg.id === messageId && msg.role === MESSAGE_ROLES.ASSISTANT
            ? { ...msg, isReasoningExpanded: !msg.isReasoningExpanded }
            : msg,
        ),
      );
    },
    [setMessage],
  );

  // 渲染函数
  const renderCustomChatContent = useCallback(
    ({ message, className }) => {
      const isCurrentlyEditing = editingMessageId === message.id;

      return (
        <OptimizedMessageContent
          message={message}
          className={className}
          styleState={styleState}
          onToggleReasoningExpansion={toggleReasoningExpansion}
          isEditing={isCurrentlyEditing}
          onEditSave={handleEditSave}
          onEditCancel={handleEditCancel}
          editValue={editValue}
          onEditValueChange={setEditValue}
        />
      );
    },
    [
      styleState,
      editingMessageId,
      editValue,
      handleEditSave,
      handleEditCancel,
      setEditValue,
      toggleReasoningExpansion,
    ],
  );

  const renderChatBoxAction = useCallback(
    (props) => {
      const { message: currentMessage } = props;
      const isAnyMessageGenerating = message.some(
        (msg) => msg.status === 'loading' || msg.status === 'incomplete',
      );
      const isCurrentlyEditing = editingMessageId === currentMessage.id;

      return (
        <OptimizedMessageActions
          message={currentMessage}
          styleState={styleState}
          onMessageReset={messageActions.handleMessageReset}
          onMessageCopy={messageActions.handleMessageCopy}
          onMessageDelete={messageActions.handleMessageDelete}
          onRoleToggle={messageActions.handleRoleToggle}
          onMessageEdit={handleMessageEdit}
          isAnyMessageGenerating={isAnyMessageGenerating}
          isEditing={isCurrentlyEditing}
        />
      );
    },
    [messageActions, styleState, message, editingMessageId, handleMessageEdit],
  );

  // Effects

  // 同步消息和自定义请求体
  useEffect(() => {
    syncMessageToCustomBody();
  }, [message, syncMessageToCustomBody]);

  useEffect(() => {
    syncCustomBodyToMessage();
  }, [customRequestBody, syncCustomBodyToMessage]);

  // 处理URL参数
  useEffect(() => {
    if (searchParams.get('expired')) {
      Toast.warning(t('登录过期，请重新登录！'));
    }
  }, [searchParams, t]);

  // Playground 组件无需再监听窗口变化，isMobile 由 useIsMobile Hook 自动更新

  // 构建预览payload
  useEffect(() => {
    const timer = setTimeout(() => {
      const preview = constructPreviewPayload();
      setPreviewPayload(preview);
      setDebugData((prev) => ({
        ...prev,
        previewRequest: preview ? JSON.stringify(preview, null, 2) : null,
        previewTimestamp: preview ? new Date().toISOString() : null,
      }));
    }, 300);

    return () => clearTimeout(timer);
  }, [
    message,
    inputs,
    parameterEnabled,
    customRequestMode,
    customRequestBody,
    constructPreviewPayload,
    setPreviewPayload,
    setDebugData,
  ]);

  // 自动保存配置
  useEffect(() => {
    debouncedSaveConfig();
  }, [
    inputs,
    parameterEnabled,
    showDebugPanel,
    customRequestMode,
    customRequestBody,
    debouncedSaveConfig,
  ]);

  // 清空对话的处理函数
  const handleClearMessages = useCallback(() => {
    setMessage([]);
    // 清空对话后保存，传入空数组
    setTimeout(() => saveMessagesImmediately([]), 0);
  }, [setMessage, saveMessagesImmediately]);

  // 处理粘贴图片
  const handlePasteImage = useCallback(
    (base64Data) => {
      if (!inputs.imageEnabled) {
        return;
      }
      // 添加图片到 imageUrls 数组
      const newUrls = [...(inputs.imageUrls || []), base64Data];
      handleInputChange('imageUrls', newUrls);
    },
    [inputs.imageEnabled, inputs.imageUrls, handleInputChange],
  );

  // Playground Context 值
  const playgroundContextValue = {
    onPasteImage: handlePasteImage,
    imageUrls: inputs.imageUrls || [],
    imageEnabled: inputs.imageEnabled || false,
  };

  const settingsPanel = (
    <OptimizedSettingsPanel
      inputs={inputs}
      parameterEnabled={parameterEnabled}
      models={models}
      groups={groups}
      styleState={styleState}
      showSettings={showSettings}
      showDebugPanel={showDebugPanel}
      customRequestMode={customRequestMode}
      customRequestBody={customRequestBody}
      onInputChange={handleInputChange}
      onParameterToggle={handleParameterToggle}
      onCloseSettings={() => setShowSettings(false)}
      onConfigImport={handleConfigImport}
      onConfigReset={handleConfigReset}
      onCustomRequestModeChange={setCustomRequestMode}
      onCustomRequestBodyChange={setCustomRequestBody}
      previewPayload={previewPayload}
      messages={message}
    />
  );

  return (
    <PlaygroundProvider value={playgroundContextValue}>
      <div className='classic-playground'>
        <div className='classic-playground-toolbar'>
          <div className='classic-playground-modes'>
            {PLAYGROUND_MODES.map((item) => (
              <Button
                key={item.mode}
                theme={mode === item.mode ? 'solid' : 'borderless'}
                type={mode === item.mode ? 'primary' : 'tertiary'}
                aria-pressed={mode === item.mode}
                onClick={() => setMode(item.mode)}
              >
                {t(item.labelKey)}
              </Button>
            ))}
          </div>
          {mode === 'chat' && isMobile && (
            <div className='classic-playground-actions'>
              <Button
                theme='light'
                icon={<Settings size={16} />}
                aria-label={t('模型配置')}
                onClick={() => setShowSettings(true)}
              />
              <Button
                theme='light'
                icon={<Code size={16} />}
                aria-label={t('调试信息')}
                onClick={() => setShowDebugPanel(true)}
              />
            </div>
          )}
        </div>

        <Layout className='classic-playground-workspace'>
          {mode === 'chat' && !isMobile && (
            <Layout.Sider className='classic-playground-settings' width={304}>
              {settingsPanel}
            </Layout.Sider>
          )}
          <Layout.Content className='classic-playground-main'>
            {mode === 'chat' ? (
              <ChatArea
                chatRef={chatRef}
                message={message}
                inputs={inputs}
                styleState={styleState}
                showDebugPanel={showDebugPanel}
                roleInfo={roleInfo}
                onMessageSend={onMessageSend}
                onMessageCopy={messageActions.handleMessageCopy}
                onMessageReset={messageActions.handleMessageReset}
                onMessageDelete={messageActions.handleMessageDelete}
                onStopGenerator={onStopGenerator}
                onClearMessages={handleClearMessages}
                onToggleDebugPanel={() => setShowDebugPanel(!showDebugPanel)}
                renderCustomChatContent={renderCustomChatContent}
                renderChatBoxAction={renderChatBoxAction}
              />
            ) : (
              <div className='classic-playground-media'>
                {mode === 'image' && (
                  <PlaygroundImage
                    models={models}
                    groups={groups}
                    selectedModel={inputs.model}
                    selectedGroup={inputs.group}
                    onModelChange={(value) => handleInputChange('model', value)}
                    onGroupChange={(value) => handleInputChange('group', value)}
                  />
                )}
                {mode === 'video' && (
                  <PlaygroundVideo
                    models={models}
                    groups={groups}
                    selectedModel={inputs.model}
                    selectedGroup={inputs.group}
                    onModelChange={(value) => handleInputChange('model', value)}
                    onGroupChange={(value) => handleInputChange('group', value)}
                  />
                )}
                {mode === 'audio' && (
                  <PlaygroundAudio
                    models={models}
                    groups={groups}
                    selectedModel={inputs.model}
                    selectedGroup={inputs.group}
                    onModelChange={(value) => handleInputChange('model', value)}
                    onGroupChange={(value) => handleInputChange('group', value)}
                  />
                )}
              </div>
            )}
          </Layout.Content>
        </Layout>
      </div>

      <SideSheet
        visible={mode === 'chat' && isMobile && showSettings}
        closeOnEsc
        aria-label={t('模型配置')}
        onCancel={() => setShowSettings(false)}
        width='100%'
        headerStyle={{ display: 'none' }}
        bodyStyle={{ padding: 0 }}
      >
        {settingsPanel}
      </SideSheet>
      <SideSheet
        visible={mode === 'chat' && showDebugPanel}
        closeOnEsc
        aria-label={t('调试信息')}
        onCancel={() => setShowDebugPanel(false)}
        width={isMobile ? '100%' : 480}
        headerStyle={{ display: 'none' }}
        bodyStyle={{ padding: 0 }}
      >
        <OptimizedDebugPanel
          debugData={debugData}
          activeDebugTab={activeDebugTab}
          onActiveDebugTabChange={setActiveDebugTab}
          styleState={{ isMobile: true }}
          showDebugPanel={showDebugPanel}
          onCloseDebugPanel={() => setShowDebugPanel(false)}
          customRequestMode={customRequestMode}
        />
      </SideSheet>
    </PlaygroundProvider>
  );
};

export default Playground;
