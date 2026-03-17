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

import React, { useCallback, useContext, useEffect, useState } from 'react';
import { Button, Tag, Typography } from '@douyinfe/semi-ui';
import { API, getLogo, getSystemName, showError } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import {
  IconFile,
  IconGithubLogo,
  IconGridView,
  IconKey,
} from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';

const { Title, Text } = Typography;

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const isMobile = useIsMobile();
  const isDemoSiteMode = statusState?.status?.demo_site_enabled || false;
  const docsLink = statusState?.status?.docs_link || '';
  const logo = getLogo();
  const systemName = getSystemName();

  const displayHomePageContent = useCallback(async () => {
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
  }, [actualTheme, i18n.language]);

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
  }, [displayHomePageContent]);

  return (
    <div className='w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='w-full overflow-x-hidden'>
          <div className='w-full border-b border-semi-color-border min-h-[560px] md:min-h-[640px] lg:min-h-[720px] relative overflow-x-hidden'>
            <div className='blur-ball blur-ball-indigo' />
            <div className='blur-ball blur-ball-teal' />
            <div className='flex items-center justify-center h-full px-4 py-20 md:py-24 lg:py-32 mt-10'>
              <div className='w-full max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-[1.2fr_0.8fr] gap-8 lg:gap-10 items-center'>
                <div className='text-center lg:text-left flex flex-col items-center lg:items-start'>
                  <div className='inline-flex items-center gap-2 px-4 py-2 rounded-full border border-semi-color-border bg-[var(--semi-color-fill-0)] mb-5'>
                    <span className='w-2.5 h-2.5 rounded-full bg-emerald-500' />
                    <Text>{t('GPT 专用入口')}</Text>
                  </div>
                  <div className='w-20 h-20 md:w-24 md:h-24 rounded-3xl bg-white/90 shadow-lg border border-white/60 flex items-center justify-center overflow-hidden mb-6'>
                    <img src={logo} alt={systemName} className='w-full h-full object-contain p-3' />
                  </div>
                  <Title
                    heading={1}
                    className='!mb-4 !text-4xl md:!text-5xl lg:!text-6xl !leading-tight max-w-3xl'
                  >
                    {systemName}
                  </Title>
                  <Text className='!text-base md:!text-lg lg:!text-xl text-semi-color-text-1 max-w-2xl'>
                    {t('专注 GPT 模型访问、兑换码开通和稳定使用体验。登录后可直接查看价格、获取默认令牌并开始使用。')}
                  </Text>
                  <div className='flex flex-wrap gap-2 mt-6 justify-center lg:justify-start'>
                    <Tag color='blue' shape='circle'>
                      {t('仅展示 GPT 模型')}
                    </Tag>
                    <Tag color='green' shape='circle'>
                      {t('默认分组 default')}
                    </Tag>
                    <Tag color='orange' shape='circle'>
                      {t('新账户自动创建 default token')}
                    </Tag>
                  </div>
                  <div className='flex flex-wrap gap-3 mt-8 justify-center lg:justify-start'>
                    <Link to='/console'>
                      <Button
                        theme='solid'
                        type='primary'
                        size={isMobile ? 'default' : 'large'}
                        className='!rounded-3xl px-8 py-2'
                        icon={<IconKey />}
                      >
                        {t('进入控制台')}
                      </Button>
                    </Link>
                    <Link to='/pricing'>
                      <Button
                        size={isMobile ? 'default' : 'large'}
                        className='!rounded-3xl px-6 py-2'
                        icon={<IconGridView />}
                      >
                        {t('查看价格')}
                      </Button>
                    </Link>
                    {isDemoSiteMode && statusState?.status?.version ? (
                      <Button
                        size={isMobile ? 'default' : 'large'}
                        className='!rounded-3xl px-6 py-2'
                        icon={<IconGithubLogo />}
                        onClick={() =>
                          window.open(
                            'https://github.com/QuantumNous/new-api',
                            '_blank',
                          )
                        }
                      >
                        {statusState.status.version}
                      </Button>
                    ) : (
                      docsLink && (
                        <Button
                          size={isMobile ? 'default' : 'large'}
                          className='!rounded-3xl px-6 py-2'
                          icon={<IconFile />}
                          onClick={() => window.open(docsLink, '_blank')}
                        >
                          {t('文档')}
                        </Button>
                      )
                    )}
                  </div>
                </div>

                <div className='rounded-[28px] border border-semi-color-border bg-[color:var(--semi-color-bg-1)]/90 backdrop-blur-sm p-6 md:p-8 shadow-xl'>
                  <Title heading={4} className='!mb-2'>
                    {t('默认使用方式')}
                  </Title>
                  <Text type='secondary' className='block !mb-6'>
                    {t('管理端配置仍然优先；这里展示的是当前默认入口与使用路径。')}
                  </Text>
                  <div className='space-y-4'>
                    {[
                      {
                        title: t('1. 登录后直接使用'),
                        desc: t('新用户创建后会自动拥有一个不过期的 default token，无需再手动新建。'),
                      },
                      {
                        title: t('2. 价格以后台定价为准'),
                        desc: t('用户侧价格页直接跟随系统设置中的分组与模型定价，只展示当前默认范围。'),
                      },
                      {
                        title: t('3. 订阅与额度统一走兑换码'),
                        desc: t('购买兑换码后回到控制台兑换，即可完成充值或订阅开通。'),
                      },
                    ].map((item) => (
                      <div
                        key={item.title}
                        className='rounded-2xl border border-semi-color-border bg-[var(--semi-color-fill-0)] px-4 py-4'
                      >
                        <Text strong className='block !mb-1'>
                          {item.title}
                        </Text>
                        <Text type='secondary'>{item.desc}</Text>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe src={homePageContent} title={t('自定义首页内容')} className='w-full h-screen border-none' />
          ) : (
            // eslint-disable-next-line react/no-danger
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
