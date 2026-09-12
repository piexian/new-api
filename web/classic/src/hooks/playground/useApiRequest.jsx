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

import { useCallback, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { SSE } from 'sse.js';
import {
  NATIVE_STREAM_EVENTS,
  normalizeNativeResponse,
  parseNativeStreamResponse,
} from '../../helpers/playground/native-response';
import { getResponseFormat } from '../../helpers/playground/request';
import {
  API_ENDPOINTS,
  MESSAGE_STATUS,
  DEBUG_TABS,
} from '../../constants/playground.constants';
import {
  getUserIdFromLocalStorage,
  handleApiError,
  processThinkTags,
  processIncompleteThinkTags,
} from '../../helpers';

export const useApiRequest = (
  setMessage,
  setDebugData,
  setActiveDebugTab,
  sseSourceRef,
  saveMessages,
) => {
  const { t } = useTranslation();
  const nonStreamControllerRef = useRef(null);
  const nonStreamRequestId = useRef(0);
  const cancelNonStreamRequest = useCallback(() => {
    nonStreamRequestId.current += 1;
    nonStreamControllerRef.current?.abort();
    nonStreamControllerRef.current = null;
  }, []);

  useEffect(() => cancelNonStreamRequest, [cancelNonStreamRequest]);

  // 处理消息自动关闭逻辑的公共函数
  const applyAutoCollapseLogic = useCallback(
    (message, isThinkingComplete = true) => {
      const shouldAutoCollapse =
        isThinkingComplete && !message.hasAutoCollapsed;
      return {
        isThinkingComplete,
        hasAutoCollapsed: shouldAutoCollapse || message.hasAutoCollapsed,
        isReasoningExpanded: shouldAutoCollapse
          ? false
          : message.isReasoningExpanded,
      };
    },
    [],
  );

  // 流式消息更新
  const streamMessageUpdate = useCallback(
    (textChunk, type) => {
      setMessage((prevMessage) => {
        const lastMessage = prevMessage[prevMessage.length - 1];
        if (!lastMessage) return prevMessage;
        if (lastMessage.role !== 'assistant') return prevMessage;
        if (lastMessage.status === MESSAGE_STATUS.ERROR) {
          return prevMessage;
        }

        if (
          lastMessage.status === MESSAGE_STATUS.LOADING ||
          lastMessage.status === MESSAGE_STATUS.INCOMPLETE
        ) {
          let newMessage = { ...lastMessage };

          if (type === 'reasoning') {
            newMessage = {
              ...newMessage,
              reasoningContent:
                (lastMessage.reasoningContent || '') + textChunk,
              status: MESSAGE_STATUS.INCOMPLETE,
              isThinkingComplete: false,
            };
          } else if (type === 'content') {
            const shouldCollapseReasoning =
              !lastMessage.content && lastMessage.reasoningContent;
            const newContent = (lastMessage.content || '') + textChunk;

            let shouldCollapseFromThinkTag = false;
            let thinkingCompleteFromTags = lastMessage.isThinkingComplete;

            if (
              lastMessage.isReasoningExpanded &&
              newContent.includes('</think>')
            ) {
              const thinkMatches = newContent.match(/<think>/g);
              const thinkCloseMatches = newContent.match(/<\/think>/g);
              if (
                thinkMatches &&
                thinkCloseMatches &&
                thinkCloseMatches.length >= thinkMatches.length
              ) {
                shouldCollapseFromThinkTag = true;
                thinkingCompleteFromTags = true; // think标签闭合也标记思考完成
              }
            }

            // 如果开始接收content内容，且之前有reasoning内容，或者think标签已闭合，则标记思考完成
            const isThinkingComplete =
              (lastMessage.reasoningContent &&
                !lastMessage.isThinkingComplete) ||
              thinkingCompleteFromTags;

            const autoCollapseState = applyAutoCollapseLogic(
              lastMessage,
              isThinkingComplete,
            );

            newMessage = {
              ...newMessage,
              content: newContent,
              status: MESSAGE_STATUS.INCOMPLETE,
              ...autoCollapseState,
            };
          }

          return [...prevMessage.slice(0, -1), newMessage];
        }

        return prevMessage;
      });
    },
    [setMessage, applyAutoCollapseLogic],
  );

  // 完成消息
  const completeMessage = useCallback(
    (status = MESSAGE_STATUS.COMPLETE) => {
      setMessage((prevMessage) => {
        const lastMessage = prevMessage[prevMessage.length - 1];
        if (
          !lastMessage ||
          lastMessage.status === MESSAGE_STATUS.COMPLETE ||
          lastMessage.status === MESSAGE_STATUS.ERROR
        ) {
          return prevMessage;
        }

        const autoCollapseState = applyAutoCollapseLogic(lastMessage, true);

        const updatedMessages = [
          ...prevMessage.slice(0, -1),
          {
            ...lastMessage,
            status: status,
            ...autoCollapseState,
          },
        ];

        // 在消息完成时保存，传入更新后的消息列表
        if (
          status === MESSAGE_STATUS.COMPLETE ||
          status === MESSAGE_STATUS.ERROR
        ) {
          setTimeout(() => saveMessages(updatedMessages), 0);
        }

        return updatedMessages;
      });
    },
    [setMessage, applyAutoCollapseLogic, saveMessages],
  );

  // 非流式请求
  const handleNonStreamRequest = useCallback(
    async (payload, endpoint) => {
      cancelNonStreamRequest();
      const controller = new AbortController();
      const requestId = nonStreamRequestId.current;
      nonStreamControllerRef.current = controller;
      const isCurrentRequest = () =>
        nonStreamRequestId.current === requestId && !controller.signal.aborted;
      const url = endpoint || API_ENDPOINTS.CHAT_COMPLETIONS;
      setDebugData((prev) => ({
        ...prev,
        request: payload,
        timestamp: new Date().toISOString(),
        response: null,
        sseMessages: null, // 非流式请求清除 SSE 消息
        isStreaming: false,
      }));
      setActiveDebugTab(DEBUG_TABS.REQUEST);

      try {
        const response = await fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'New-Api-User': getUserIdFromLocalStorage(),
          },
          body: JSON.stringify(payload),
          signal: controller.signal,
        });

        if (!isCurrentRequest()) return;
        if (!response.ok) {
          let errorBody = '';
          let parsedError = null;
          try {
            errorBody = await response.text();
            const errorJson = JSON.parse(errorBody);
            if (errorJson?.error) {
              parsedError = errorJson.error;
            }
          } catch (e) {
            if (!errorBody) {
              errorBody = '无法读取错误响应体';
            }
          }

          if (!isCurrentRequest()) return;
          const errorInfo = handleApiError(
            new Error(
              `HTTP error! status: ${response.status}, body: ${errorBody}`,
            ),
            response,
          );

          setDebugData((prev) => ({
            ...prev,
            response: JSON.stringify(errorInfo, null, 2),
          }));
          setActiveDebugTab(DEBUG_TABS.RESPONSE);

          const err = new Error(
            parsedError?.message ||
              `HTTP error! status: ${response.status}, body: ${errorBody}`,
          );
          err.errorCode = parsedError?.code || null;
          err.errorType = parsedError?.type || null;
          throw err;
        }

        const rawData = await response.json();
        if (!isCurrentRequest()) return;

        setDebugData((prev) => ({
          ...prev,
          response: JSON.stringify(rawData, null, 2),
        }));
        setActiveDebugTab(DEBUG_TABS.RESPONSE);

        const data = normalizeNativeResponse(rawData, getResponseFormat(url));
        if (!data.choices?.[0]) throw new Error(t('解析响应数据时发生错误'));
        if (data.choices?.[0]) {
          const choice = data.choices[0];
          let content = choice.message?.content || '';
          let reasoningContent =
            choice.message?.reasoning_content ||
            choice.message?.reasoning ||
            '';

          const processed = processThinkTags(content, reasoningContent);

          setMessage((prevMessage) => {
            if (!isCurrentRequest()) return prevMessage;
            const newMessages = [...prevMessage];
            const lastMessage = newMessages[newMessages.length - 1];
            if (lastMessage?.status === MESSAGE_STATUS.LOADING) {
              const autoCollapseState = applyAutoCollapseLogic(
                lastMessage,
                true,
              );

              newMessages[newMessages.length - 1] = {
                ...lastMessage,
                content: processed.content,
                reasoningContent: processed.reasoningContent,
                status: MESSAGE_STATUS.COMPLETE,
                ...autoCollapseState,
              };
            }
            setTimeout(() => saveMessages(newMessages), 0);
            return newMessages;
          });
        }
      } catch (error) {
        if (!isCurrentRequest()) return;
        console.error('Non-stream request error:', error);

        const errorInfo = handleApiError(error);
        setDebugData((prev) => ({
          ...prev,
          response: JSON.stringify(errorInfo, null, 2),
        }));
        setActiveDebugTab(DEBUG_TABS.RESPONSE);

        setMessage((prevMessage) => {
          if (!isCurrentRequest()) return prevMessage;
          const newMessages = [...prevMessage];
          const lastMessage = newMessages[newMessages.length - 1];
          if (lastMessage?.status === MESSAGE_STATUS.LOADING) {
            const autoCollapseState = applyAutoCollapseLogic(lastMessage, true);

            newMessages[newMessages.length - 1] = {
              ...lastMessage,
              content: t('请求发生错误: ') + error.message,
              errorCode: error.errorCode || null,
              status: MESSAGE_STATUS.ERROR,
              ...autoCollapseState,
            };
          }
          setTimeout(() => saveMessages(newMessages), 0);
          return newMessages;
        });
      }
    },
    [
      setDebugData,
      setActiveDebugTab,
      setMessage,
      t,
      applyAutoCollapseLogic,
      saveMessages,
      cancelNonStreamRequest,
    ],
  );

  // SSE请求
  const handleSSE = useCallback(
    (payload, endpoint) => {
      const url = endpoint || API_ENDPOINTS.CHAT_COMPLETIONS;

      setDebugData((prev) => ({
        ...prev,
        request: payload,
        timestamp: new Date().toISOString(),
        response: null,
        sseMessages: [],
        isStreaming: true,
      }));
      setActiveDebugTab(DEBUG_TABS.REQUEST);

      const source = new SSE(url, {
        headers: {
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        method: 'POST',
        payload: JSON.stringify(payload),
        start: false,
      });

      let responseData = '';
      let hasReceivedFirstResponse = false;
      let isStreamComplete = false;
      sseSourceRef.current = {
        close: () => {
          isStreamComplete = true;
          source.close();
        },
      };

      const onStreamMessage = (e) => {
        if (isStreamComplete) return;
        if (e.data === '[DONE]') {
          isStreamComplete = true; // 标记流正常完成
          source.close();
          sseSourceRef.current = null;
          setDebugData((prev) => ({
            ...prev,
            response: responseData,
            sseMessages: [...(prev.sseMessages || []), '[DONE]'], // 添加 DONE 标记
            isStreaming: false,
          }));
          completeMessage();
          return;
        }

        try {
          const payload = JSON.parse(e.data);
          responseData += e.data + '\n';

          if (!hasReceivedFirstResponse) {
            setActiveDebugTab(DEBUG_TABS.RESPONSE);
            hasReceivedFirstResponse = true;
          }

          // 新增：将 SSE 消息添加到数组
          setDebugData((prev) => ({
            ...prev,
            sseMessages: [...(prev.sseMessages || []), e.data],
          }));

          const parsed = parseNativeStreamResponse({
            ...payload,
            type: payload.type || e.type,
          });
          if (parsed.error) throw new Error(parsed.error);
          for (const update of parsed.updates) {
            streamMessageUpdate(update.chunk, update.type);
          }
          if (parsed.done) {
            isStreamComplete = true;
            source.close();
            sseSourceRef.current = null;
            setDebugData((prev) => ({
              ...prev,
              response: responseData,
              isStreaming: false,
            }));
            completeMessage();
          }
        } catch (error) {
          console.error('Failed to parse SSE message:', error);
          const errorInfo = `解析错误: ${error.message}`;

          setDebugData((prev) => ({
            ...prev,
            response: responseData + `\n\nError: ${errorInfo}`,
            sseMessages: [...(prev.sseMessages || []), e.data], // 即使解析失败也保存原始数据
            isStreaming: false,
          }));
          setActiveDebugTab(DEBUG_TABS.RESPONSE);

          isStreamComplete = true;
          source.close();
          sseSourceRef.current = null;
          streamMessageUpdate(
            t('解析响应数据时发生错误') + ': ' + error.message,
            'content',
          );
          completeMessage(MESSAGE_STATUS.ERROR);
        }
      };
      for (const event of ['message', ...NATIVE_STREAM_EVENTS]) {
        source.addEventListener(event, onStreamMessage);
      }

      source.addEventListener('error', (e) => {
        // 只有在流没有正常完成且连接状态异常时才处理错误
        if (!isStreamComplete) {
          isStreamComplete = true;
          console.error('SSE Error:', e);
          let errorMessage = e.data || t('请求发生错误');
          let errorCode = null;

          if (e.data) {
            try {
              const errorJson = JSON.parse(e.data);
              if (errorJson?.error) {
                errorMessage = errorJson.error.message || errorMessage;
                errorCode = errorJson.error.code || null;
              }
            } catch (_) {
              // not JSON, use raw data as error message
            }
          }

          const errorInfo = handleApiError(new Error(errorMessage));
          errorInfo.readyState = source.readyState;

          setDebugData((prev) => ({
            ...prev,
            response:
              responseData +
              '\n\nSSE Error:\n' +
              JSON.stringify(errorInfo, null, 2),
            isStreaming: false,
          }));
          setActiveDebugTab(DEBUG_TABS.RESPONSE);

          setMessage((prevMessage) => {
            const newMessages = [...prevMessage];
            const lastMessage = newMessages[newMessages.length - 1];
            if (
              lastMessage &&
              lastMessage.status !== MESSAGE_STATUS.COMPLETE &&
              lastMessage.status !== MESSAGE_STATUS.ERROR
            ) {
              newMessages[newMessages.length - 1] = {
                ...lastMessage,
                content: (lastMessage.content || '') + errorMessage,
                errorCode: errorCode,
                status: MESSAGE_STATUS.ERROR,
              };
            }
            setTimeout(() => saveMessages(newMessages), 0);
            return newMessages;
          });
          sseSourceRef.current = null;
          source.close();
        }
      });

      source.addEventListener('readystatechange', (e) => {
        // 检查 HTTP 状态错误，但避免与正常关闭重复处理
        if (e.readyState >= 2 && !isStreamComplete) {
          isStreamComplete = true;
          const errorInfo = handleApiError(new Error('HTTP状态错误'));
          errorInfo.status = source.status;
          errorInfo.readyState = source.readyState;

          setDebugData((prev) => ({
            ...prev,
            response:
              responseData +
              '\n\nHTTP Error:\n' +
              JSON.stringify(errorInfo, null, 2),
          }));
          setActiveDebugTab(DEBUG_TABS.RESPONSE);

          source.close();
          sseSourceRef.current = null;
          setDebugData((prev) => ({ ...prev, isStreaming: false }));
          streamMessageUpdate(t('连接已断开'), 'content');
          completeMessage(MESSAGE_STATUS.ERROR);
        }
      });

      try {
        source.stream();
      } catch (error) {
        console.error('Failed to start SSE stream:', error);
        const errorInfo = handleApiError(error);

        setDebugData((prev) => ({
          ...prev,
          response: 'Stream启动失败:\n' + JSON.stringify(errorInfo, null, 2),
        }));
        setActiveDebugTab(DEBUG_TABS.RESPONSE);

        isStreamComplete = true;
        source.close();
        sseSourceRef.current = null;
        setDebugData((prev) => ({ ...prev, isStreaming: false }));
        streamMessageUpdate(t('建立连接时发生错误'), 'content');
        completeMessage(MESSAGE_STATUS.ERROR);
      }
    },
    [
      setDebugData,
      setActiveDebugTab,
      setMessage,
      streamMessageUpdate,
      completeMessage,
      t,
      sseSourceRef,
      saveMessages,
    ],
  );

  // 停止生成
  const onStopGenerator = useCallback(() => {
    cancelNonStreamRequest();
    // 如果仍有活动的 SSE 连接，首先关闭
    if (sseSourceRef.current) {
      sseSourceRef.current.close();
      sseSourceRef.current = null;
    }

    setDebugData((prev) => ({ ...prev, isStreaming: false }));

    // 无论是否存在 SSE 连接，都尝试处理最后一条正在生成的消息
    setMessage((prevMessage) => {
      if (prevMessage.length === 0) return prevMessage;
      const lastMessage = prevMessage[prevMessage.length - 1];

      if (
        lastMessage.status === MESSAGE_STATUS.LOADING ||
        lastMessage.status === MESSAGE_STATUS.INCOMPLETE
      ) {
        const processed = processIncompleteThinkTags(
          lastMessage.content || '',
          lastMessage.reasoningContent || '',
        );

        const autoCollapseState = applyAutoCollapseLogic(lastMessage, true);

        const updatedMessages = [
          ...prevMessage.slice(0, -1),
          {
            ...lastMessage,
            status: MESSAGE_STATUS.COMPLETE,
            reasoningContent: processed.reasoningContent || null,
            content: processed.content,
            ...autoCollapseState,
          },
        ];

        // 停止生成时也保存，传入更新后的消息列表
        setTimeout(() => saveMessages(updatedMessages), 0);

        return updatedMessages;
      }
      return prevMessage;
    });
  }, [
    setMessage,
    applyAutoCollapseLogic,
    saveMessages,
    setDebugData,
    sseSourceRef,
    cancelNonStreamRequest,
  ]);

  // 发送请求
  const sendRequest = useCallback(
    (payload, isStream, endpoint) => {
      if (isStream) {
        cancelNonStreamRequest();
        return handleSSE(payload, endpoint);
      }
      return handleNonStreamRequest(payload, endpoint);
    },
    [handleSSE, handleNonStreamRequest, cancelNonStreamRequest],
  );

  return {
    sendRequest,
    onStopGenerator,
    streamMessageUpdate,
    completeMessage,
  };
};
