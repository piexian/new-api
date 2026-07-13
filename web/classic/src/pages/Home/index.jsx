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

import React, { useContext, useEffect, useState } from 'react';
import { Button, Typography } from '@douyinfe/semi-ui';
import {
  API,
  showError,
  copy,
  showSuccess,
  resolveAppRoute,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { IconCopy, IconPlay } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';
import HomeCapabilityTabs from '../../components/home/HomeCapabilityTabs';
import HomeStarfieldBackground from '../../components/home/HomeStarfieldBackground';

const { Text } = Typography;

const SERVICE_ITEMS = [
  { title: '极速响应？', desc: '全球节点均未优化' },
  { title: '稳定高可用？', desc: '私人自用服务' },
  { title: '公益免费', desc: '用爱发电 随时跑路' },
];

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const isMobile = useIsMobile();
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const isChinese = i18n.language.startsWith('zh');
  const isAuthed = Boolean(localStorage.getItem('user'));

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent('加载首页内容失败...');
    }
    setHomePageContentLoaded(true);
  };

  const handleCopyBaseURL = async () => {
    const ok = await copy(serverAddress);
    if (ok) {
      showSuccess(t('已复制到剪切板'));
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  return (
    <div className='w-full overflow-x-hidden relative min-h-[calc(100vh-64px)]'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <>
          <HomeStarfieldBackground />
          <div className='relative z-[1] w-full px-4 pt-16 pb-10 md:pt-24 md:pb-14'>
            <div className='mx-auto flex max-w-3xl flex-col items-center text-center'>
              <p className='mb-3 text-xs font-semibold tracking-[0.18em] uppercase text-semi-color-text-2'>
                ✦ Starfield Gateway
              </p>
              <h1
                className={`text-4xl md:text-5xl lg:text-6xl font-bold text-semi-color-text-0 leading-tight ${isChinese ? 'tracking-wide md:tracking-wider' : ''}`}
              >
                {t('统一的')}
                <br />
                <span className='shine-text'>{t('大模型接口网关')}</span>
              </h1>
              <p className='text-base md:text-lg text-semi-color-text-1 mt-4 md:mt-6 max-w-xl'>
                {t('多模型统一接入，只需将基址替换为：')}
              </p>

              <div
                className='mt-6 flex w-full max-w-xl items-stretch overflow-hidden rounded-2xl border border-semi-color-border bg-semi-color-bg-1/85 shadow-sm backdrop-blur'
                style={{ textAlign: 'left' }}
              >
                <div className='flex shrink-0 items-center border-r border-semi-color-border px-3 text-[11px] font-bold tracking-wider uppercase text-semi-color-text-2'>
                  BASE URL
                </div>
                <button
                  type='button'
                  onClick={handleCopyBaseURL}
                  className='min-w-0 flex-1 truncate bg-transparent px-3 py-3 text-left font-mono text-sm font-semibold text-semi-color-text-0'
                  style={{ border: 'none', cursor: 'pointer' }}
                >
                  {serverAddress}
                </button>
                <Button
                  type='primary'
                  theme='borderless'
                  icon={<IconCopy />}
                  onClick={handleCopyBaseURL}
                  className='!rounded-none px-4'
                >
                  {t('复制')}
                </Button>
              </div>

              <div className='mt-8 flex flex-row flex-wrap items-center justify-center gap-3'>
                <Link
                  to={
                    isAuthed
                      ? resolveAppRoute('dashboard')
                      : resolveAppRoute('sign_up')
                  }
                >
                  <Button
                    theme='solid'
                    type='primary'
                    size={isMobile ? 'default' : 'large'}
                    className='!rounded-xl px-8 py-2'
                    icon={<IconPlay />}
                  >
                    {isAuthed ? t('控制台') : t('获取密钥 / 控制台')}
                  </Button>
                </Link>
                <Link to={resolveAppRoute('pricing')}>
                  <Button
                    size={isMobile ? 'default' : 'large'}
                    className='!rounded-xl px-8 py-2'
                  >
                    {t('模型广场')}
                  </Button>
                </Link>
              </div>
            </div>

            <div className='mx-auto mt-10 w-full max-w-3xl px-1 text-left'>
              <HomeCapabilityTabs serverAddress={serverAddress} />
            </div>

            <div className='mx-auto mt-10 grid w-full max-w-4xl grid-cols-1 gap-3 px-1 md:grid-cols-3'>
              {SERVICE_ITEMS.map((item) => (
                <div
                  key={item.title}
                  className='rounded-2xl border border-semi-color-border bg-semi-color-bg-1/80 px-4 py-5 text-center backdrop-blur'
                >
                  <div className='text-sm font-bold text-semi-color-text-0'>
                    {t(item.title)}
                  </div>
                  <div className='mt-1.5 text-xs leading-relaxed text-semi-color-text-2'>
                    {t(item.desc)}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
              title={t('自定义首页')}
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
