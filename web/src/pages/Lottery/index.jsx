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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Card, Input, Modal, Tag, Typography } from '@douyinfe/semi-ui';
import { Crown, Gem, Gift, Sparkles, Star } from 'lucide-react';
import fireworks from 'react-fireworks';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';

const DRAW_DURATION_MS = 1500;

const VISUALS = [
  {
    key: 'common',
    accent: '#5fa8ff',
    icon: Gift,
    glow: 'rgba(95,168,255,0.30)',
    card: 'linear-gradient(135deg, rgba(95,168,255,0.20), rgba(255,255,255,0.95))',
    copy: '普通档位，先稳稳拿下。',
  },
  {
    key: 'rare',
    accent: '#35c4c4',
    icon: Sparkles,
    glow: 'rgba(53,196,196,0.30)',
    card: 'linear-gradient(135deg, rgba(53,196,196,0.22), rgba(255,255,255,0.95))',
    copy: '稀有亮灯，手感开始上来了。',
  },
  {
    key: 'epic',
    accent: '#8a63ff',
    icon: Gem,
    glow: 'rgba(138,99,255,0.30)',
    card: 'linear-gradient(135deg, rgba(138,99,255,0.22), rgba(255,255,255,0.95))',
    copy: '史诗出货，明天可用额度已经很有存在感。',
  },
  {
    key: 'legendary',
    accent: '#ff9f1a',
    icon: Star,
    glow: 'rgba(255,159,26,0.30)',
    card: 'linear-gradient(135deg, rgba(255,159,26,0.24), rgba(255,255,255,0.95))',
    copy: '传说档位，够亮眼了。',
  },
  {
    key: 'mythic',
    accent: '#ff4f88',
    icon: Crown,
    glow: 'rgba(255,79,136,0.34)',
    card: 'linear-gradient(135deg, rgba(255,79,136,0.26), rgba(255,255,255,0.95))',
    copy: '神话开门，这一发该放烟花。',
  },
];

const DEFAULT_POOL = [
  { name: '普通', amount: 3, probability: 82 },
  { name: '稀有', amount: 8, probability: 13 },
  { name: '史诗', amount: 20, probability: 4 },
  { name: '传说', amount: 50, probability: 0.8 },
  { name: '神话', amount: 200, probability: 0.2 },
];

function formatTime(ts) {
  return ts ? new Date(ts * 1000).toLocaleString() : '--';
}

function formatBeijingDate(ts) {
  if (!ts) {
    return '--';
  }
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date(ts * 1000));
  const values = Object.fromEntries(
    parts
      .filter((part) => part.type !== 'literal')
      .map((part) => [part.type, part.value]),
  );
  return `${values.year}-${values.month}-${values.day}`;
}

function formatAmount(value) {
  const num = Number(value || 0);
  if (Number.isNaN(num)) {
    return '0';
  }
  if (Math.abs(num - Math.round(num)) < 0.000001) {
    return String(Math.round(num));
  }
  return num.toFixed(2).replace(/\.?0+$/, '');
}

function phaseLabel(phase, t) {
  switch (phase) {
    case 'draw':
      return { text: t('抽奖日'), color: 'blue' };
    case 'waiting_consume':
      return { text: t('等待次日消费'), color: 'orange' };
    case 'consume':
      return { text: t('消费日'), color: 'green' };
    case 'expired':
      return { text: t('已过期'), color: 'grey' };
    default:
      return { text: t('待开启'), color: 'grey' };
  }
}

function rewardStatus(reward, t) {
  if (reward.can_consume) {
    return { text: t('当前可消费'), color: 'green' };
  }
  switch (reward.status) {
    case 'pending_activation':
      return { text: t('待激活'), color: 'orange' };
    case 'activated':
      return { text: t('已激活'), color: 'blue' };
    case 'consumed':
      return { text: t('已消费'), color: 'green' };
    case 'expired':
      return { text: t('已过期'), color: 'grey' };
    default:
      return { text: t('未知状态'), color: 'grey' };
  }
}

const Lottery = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const [backendState, setBackendState] = useState(null);
  const [stage, setStage] = useState('idle');
  const [result, setResult] = useState(null);
  const [drawSeed, setDrawSeed] = useState(0);
  const [stateLoading, setStateLoading] = useState(false);
  const [recentWins, setRecentWins] = useState([]);
  const [recentWinsLoading, setRecentWinsLoading] = useState(false);
  const [drawLoading, setDrawLoading] = useState(false);
  const [activateLoading, setActivateLoading] = useState(false);
  const [openLoading, setOpenLoading] = useState('');
  const [giftModalVisible, setGiftModalVisible] = useState(false);
  const [giftLoading, setGiftLoading] = useState(false);
  const [giftUsername, setGiftUsername] = useState('');
  const [giftReward, setGiftReward] = useState(null);
  const revealTimerRef = useRef(null);
  const fireworksStopTimerRef = useRef(null);

  const isAdmin = (userState?.user?.role || 0) >= 10;

  const rewardPool = useMemo(() => {
    const pool = backendState?.reward_pool?.length
      ? backendState.reward_pool
      : DEFAULT_POOL;
    return pool.map((item, index) => ({ ...VISUALS[index], ...item }));
  }, [backendState?.reward_pool]);

  const decorateReward = (reward) => {
    const visual =
      rewardPool.find((item) => item.name === reward?.tier_name) ||
      rewardPool[0];
    return {
      ...visual,
      ...reward,
      name: reward?.tier_name || visual?.name,
      copy: reward?.gifted_by_username
        ? t('来自 {{username}} 的赠送。', {
            username: reward.gifted_by_username,
          })
        : visual?.copy,
    };
  };

  const rewards = (backendState?.rewards || []).map(decorateReward);
  const decoratedRecentWins = recentWins.map(decorateReward);
  const summary = backendState?.reward_summary || {};
  const activity = backendState?.activity || null;
  const phase = phaseLabel(activity?.phase, t);
  const canDraw = Boolean(backendState?.can_participate) && stage !== 'drawing';
  const totalProbability = rewardPool.reduce(
    (sum, item) => sum + Number(item.probability || 0),
    0,
  );

  const clearTimers = () => {
    if (revealTimerRef.current) {
      window.clearTimeout(revealTimerRef.current);
      revealTimerRef.current = null;
    }
    if (fireworksStopTimerRef.current) {
      window.clearTimeout(fireworksStopTimerRef.current);
      fireworksStopTimerRef.current = null;
    }
  };

  useEffect(() => {
    return () => {
      clearTimers();
      fireworks.stop();
    };
  }, []);

  const loadState = async (silent = false) => {
    setStateLoading(true);
    try {
      const res = await API.get('/api/lottery/self/state');
      if (res.data?.success) {
        setBackendState(res.data.data || null);
      } else if (!silent) {
        showError(res.data?.message || t('大乐透状态加载失败'));
      }
    } catch (error) {
      if (!silent) {
        showError(t('大乐透状态加载失败'));
      }
    } finally {
      setStateLoading(false);
    }
  };

  const loadRecentWins = async (silent = false) => {
    setRecentWinsLoading(true);
    try {
      const res = await API.get('/api/lottery/recent-wins');
      if (res.data?.success) {
        setRecentWins(res.data.data || []);
      } else if (!silent) {
        showError(res.data?.message || t('中奖动态加载失败'));
      }
    } catch (error) {
      if (!silent) {
        showError(t('中奖动态加载失败'));
      }
    } finally {
      setRecentWinsLoading(false);
    }
  };

  useEffect(() => {
    const loadInitialData = async () => {
      await loadState();
      await loadRecentWins(true);
    };
    loadInitialData();
  }, []);

  useEffect(() => {
    clearTimers();
    setStage('idle');
    setResult(null);
  }, [activity?.id, activity?.phase, activity?.draw_date, activity?.scope]);

  const triggerMythicFireworks = () => {
    fireworks.init('root', {});
    fireworks.start();
    fireworksStopTimerRef.current = window.setTimeout(() => {
      fireworks.stop();
      fireworksStopTimerRef.current = null;
    }, 2400);
  };

  const handleDraw = async () => {
    if (!canDraw) {
      showError(t('当前不可抽奖'));
      return;
    }
    clearTimers();
    setDrawLoading(true);
    setStage('drawing');
    setResult(null);
    setDrawSeed((seed) => seed + 1);
    try {
      const res = await API.post('/api/lottery/self/draw');
      if (!res.data?.success || !res.data?.data?.reward) {
        setStage('idle');
        showError(res.data?.message || t('抽奖失败'));
        return;
      }
      const next = decorateReward(res.data.data.reward);
      revealTimerRef.current = window.setTimeout(() => {
        setResult(next);
        setStage('revealed');
        if (next.key === 'mythic') {
          triggerMythicFireworks();
        }
        revealTimerRef.current = null;
        loadState(true);
        loadRecentWins(true);
      }, DRAW_DURATION_MS);
    } catch (error) {
      setStage('idle');
      showError(t('抽奖失败'));
    } finally {
      setDrawLoading(false);
    }
  };

  const handleActivate = async () => {
    setActivateLoading(true);
    try {
      const res = await API.post('/api/lottery/self/activate');
      if (res.data?.success) {
        const count = res.data.data?.activated_count || 0;
        showSuccess(
          count > 0
            ? t('已激活 {{count}} 张乐透券', { count })
            : t('当前没有待激活的乐透券'),
        );
        await loadState(true);
      } else {
        showError(res.data?.message || t('激活失败'));
      }
    } catch (error) {
      showError(t('激活失败'));
    } finally {
      setActivateLoading(false);
    }
  };

  const handleOpen = async (scope) => {
    setOpenLoading(scope);
    try {
      const url =
        scope === 'test'
          ? '/api/lottery/admin/open/test'
          : '/api/lottery/admin/open/public';
      const res = await API.post(url);
      if (res.data?.success) {
        showSuccess(
          scope === 'test' ? t('管理员测试场已开启') : t('大乐透活动已开启'),
        );
        await loadState(true);
        await loadRecentWins(true);
      } else {
        showError(res.data?.message || t('大乐透活动开启失败'));
      }
    } catch (error) {
      showError(t('大乐透活动开启失败'));
    } finally {
      setOpenLoading('');
    }
  };

  const openGiftModal = (reward) => {
    setGiftReward(reward);
    setGiftUsername('');
    setGiftModalVisible(true);
  };

  const closeGiftModal = (forceClose = false) => {
    if (giftLoading && !forceClose) {
      return;
    }
    setGiftModalVisible(false);
    setGiftReward(null);
    setGiftUsername('');
  };

  const handleGift = async () => {
    if (!giftReward?.id) {
      showError(t('未找到可赠送的乐透券'));
      return;
    }
    if (!giftUsername.trim()) {
      showError(t('请输入目标用户名'));
      return;
    }
    setGiftLoading(true);
    try {
      const res = await API.post('/api/lottery/self/gift', {
        reward_id: giftReward.id,
        target_username: giftUsername.trim(),
      });
      if (res.data?.success) {
        showSuccess(
          t('已赠送给 {{username}}', {
            username: res.data.data?.target_username || giftUsername.trim(),
          }),
        );
        closeGiftModal(true);
        await loadState(true);
      } else {
        showError(res.data?.message || t('赠送失败'));
      }
    } catch (error) {
      showError(t('赠送失败'));
    } finally {
      setGiftLoading(false);
    }
  };

  const activeAccent = result?.accent || '#5fa8ff';
  const activeGlow = result?.glow || 'rgba(95,168,255,0.30)';
  const activeCard =
    result?.card ||
    'linear-gradient(135deg, rgba(95,168,255,0.20), rgba(255,255,255,0.95))';
  const ActiveIcon = result?.icon || Gift;

  return (
    <div className='mt-[60px] px-2 pb-10'>
      <div className='mx-auto w-full max-w-6xl'>
        <Card className='overflow-hidden !rounded-[28px] border-0 shadow-sm'>
          <div className='relative overflow-hidden rounded-[28px] bg-[linear-gradient(180deg,#f7fbff,#ffffff)] p-5 md:p-7'>
            <div className='pointer-events-none absolute -left-10 top-0 h-52 w-52 rounded-full bg-[rgba(255,176,73,0.18)] blur-3xl' />
            <div className='pointer-events-none absolute -right-8 bottom-0 h-56 w-56 rounded-full bg-[rgba(95,168,255,0.16)] blur-3xl' />

            <div className='relative z-[1] flex flex-wrap items-center justify-between gap-3'>
              <div className='flex flex-wrap items-center gap-3'>
                <Typography.Title heading={3} style={{ margin: 0 }}>
                  {t('大乐透')}
                </Typography.Title>
                <Tag color='blue'>{t('独立活动页')}</Tag>
                <Tag color={activity ? phase.color : 'grey'}>
                  {activity ? phase.text : t('当前无活动')}
                </Tag>
                {activity?.scope === 'admin_only' ? (
                  <Tag color='orange'>{t('管理员测试场')}</Tag>
                ) : null}
                {backendState?.admin_override ? (
                  <Tag color='green'>{t('管理员测试资格')}</Tag>
                ) : null}
              </div>
              <Button
                loading={stateLoading || recentWinsLoading}
                onClick={async () => {
                  await loadState(true);
                  await loadRecentWins(true);
                }}
              >
                {t('刷新状态')}
              </Button>
            </div>

            <div className='relative z-[1] mt-6 grid grid-cols-1 gap-5 xl:grid-cols-[1.02fr_0.98fr]'>
              <div className='space-y-5'>
                <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
                  {[
                    {
                      label: t('当前等级'),
                      value: backendState?.current_level ?? '--',
                    },
                    {
                      label: t('剩余抽数'),
                      value: backendState?.remaining_draw_count ?? '--',
                    },
                    {
                      label: t('待激活额度'),
                      value: `$${formatAmount(summary.pending_amount)}`,
                    },
                    {
                      label: t('当前可消费额度'),
                      value: `$${formatAmount(summary.consumable_amount)}`,
                    },
                  ].map((item) => (
                    <div
                      key={item.label}
                      className='rounded-[18px] border border-slate-200 bg-white/90 p-4 shadow-sm'
                    >
                      <div className='text-xs text-slate-500'>{item.label}</div>
                      <div className='mt-2 text-[22px] font-bold text-slate-900'>
                        {item.value}
                      </div>
                    </div>
                  ))}
                </div>

                <div className='rounded-[22px] border border-slate-200 bg-white/88 p-5 shadow-sm'>
                  <div className='flex flex-wrap items-center justify-between gap-3'>
                    <Typography.Text strong>{t('当前活动')}</Typography.Text>
                    {backendState?.panel_visible ? (
                      <Tag color='green'>{t('对你可见')}</Tag>
                    ) : (
                      <Tag color='grey'>{t('当前对你未开放')}</Tag>
                    )}
                  </div>
                  {activity ? (
                    <div className='mt-4 grid gap-2 text-sm text-slate-600'>
                      <div>
                        {t('抽奖日')}：{activity.draw_date || '--'}
                      </div>
                      <div>
                        {t('使用日')}：
                        {formatBeijingDate(activity.consume_starts_at)}
                      </div>
                    </div>
                  ) : (
                    <div className='mt-4 rounded-[18px] border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600'>
                      {backendState?.current_level >= 1
                        ? t('你已经具备参与资格，等待管理员开启活动即可。')
                        : t(
                            'LV1 起可参与大乐透；若别人赠送给你乐透券，这里也会直接显示。',
                          )}
                    </div>
                  )}

                  <div className='mt-4 flex flex-wrap gap-3'>
                    {summary.activatable_count > 0 ? (
                      <Button
                        type='primary'
                        theme='solid'
                        loading={activateLoading}
                        onClick={handleActivate}
                      >
                        {t('一键激活待激活乐透券')}
                      </Button>
                    ) : null}
                    {isAdmin ? (
                      <>
                        <Button
                          type='primary'
                          theme='light'
                          loading={openLoading === 'public'}
                          onClick={() => handleOpen('public')}
                        >
                          {t('立即开启一场大乐透')}
                        </Button>
                        <Button
                          theme='light'
                          loading={openLoading === 'test'}
                          onClick={() => handleOpen('test')}
                        >
                          {t('开启管理员测试场')}
                        </Button>
                      </>
                    ) : null}
                  </div>
                </div>

                <div className='flex flex-wrap items-center justify-between gap-3 px-1'>
                  <Typography.Text strong>
                    {activity ? t('本场奖池') : t('默认奖池')}
                  </Typography.Text>
                  <Typography.Text type='tertiary' size='small'>
                    {activity
                      ? t('当前活动使用开启时的奖池快照')
                      : t('当前显示的是系统设置中的默认奖池')}
                  </Typography.Text>
                </div>

                <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
                  {rewardPool.map((tier) => {
                    const TierIcon = tier.icon || Gift;
                    return (
                      <div
                        key={`${tier.key}-${tier.name}`}
                        className='rounded-[22px] border p-4 shadow-sm'
                        style={{
                          background: tier.card,
                          borderColor: `${tier.accent}40`,
                        }}
                      >
                        <div
                          className='mb-3 inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold'
                          style={{
                            color: tier.accent,
                            borderColor: `${tier.accent}4d`,
                          }}
                        >
                          <TierIcon size={14} />
                          <span>{tier.name}</span>
                        </div>
                        <div className='text-[30px] font-bold leading-none text-slate-900'>
                          {tier.amount}
                          <span className='ml-2 text-sm font-medium text-slate-500'>
                            {t('乐透券')}
                          </span>
                        </div>
                        <div className='mt-3 text-sm text-slate-500'>
                          {t('概率')} {tier.probability}%
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-white/86 p-4 shadow-[0_20px_44px_rgba(15,23,42,0.08)]'>
                <div
                  className={`lottery-machine ${stage === 'drawing' ? 'is-drawing' : ''}`}
                  style={{
                    background: `radial-gradient(circle at top, ${activeGlow}, transparent 48%), linear-gradient(180deg, rgba(255,255,255,0.96), rgba(248,250,255,0.96))`,
                  }}
                >
                  <div
                    className='lottery-ring lottery-ring-a'
                    style={{ borderColor: `${activeAccent}30` }}
                  />
                  <div
                    className='lottery-ring lottery-ring-b'
                    style={{ borderColor: `${activeAccent}22` }}
                  />
                  <div className='relative z-[1] flex min-h-[520px] flex-col items-center justify-center gap-5 px-5 py-8'>
                    <div
                      key={`draw-${drawSeed}-${result?.key || 'idle'}`}
                      className={`lottery-prize ${stage === 'drawing' ? 'is-drawing' : ''} ${stage === 'revealed' ? 'is-revealed' : ''}`}
                      style={{
                        borderColor: `${activeAccent}55`,
                        background: activeCard,
                        boxShadow: `0 24px 50px ${activeGlow}`,
                      }}
                    >
                      <div
                        className='lottery-shine'
                        style={{
                          background: `linear-gradient(120deg, transparent, ${activeAccent}35, transparent)`,
                        }}
                      />
                      <div
                        className='mb-4 inline-flex h-16 w-16 items-center justify-center rounded-full bg-white/80'
                        style={{ color: activeAccent }}
                      >
                        <ActiveIcon size={28} />
                      </div>
                      <div className='text-lg font-bold text-slate-900'>
                        {result ? result.name : t('等待开奖')}
                      </div>
                      <div className='mt-3 flex items-end gap-2 text-[58px] font-black leading-none text-slate-900'>
                        <span>{result ? result.amount : '?'}</span>
                        <span className='pb-1 text-sm font-semibold text-slate-500'>
                          {t('乐透券')}
                        </span>
                      </div>
                      <div className='mt-4 max-w-[240px] text-center text-sm leading-6 text-slate-600'>
                        {result
                          ? t(result.copy)
                          : backendState?.panel_visible
                            ? t('点击按钮，消耗一次本场抽奖次数。')
                            : t('活动开启后，开奖结果会显示在这里。')}
                      </div>
                    </div>

                    <div className='text-xs font-bold uppercase tracking-[0.18em] text-slate-500'>
                      {stage === 'drawing'
                        ? t('开奖中...')
                        : result
                          ? t('开奖完成')
                          : activity
                            ? phase.text
                            : t('等待开场')}
                    </div>

                    <Button
                      type='primary'
                      theme='solid'
                      size='large'
                      loading={drawLoading}
                      disabled={!canDraw}
                      onClick={handleDraw}
                      className='!h-12 !rounded-full !px-8 !font-bold'
                    >
                      {stage === 'drawing' ? t('正在开奖') : t('抽一发大乐透')}
                    </Button>

                    <div className='text-center text-sm leading-6 text-slate-500'>
                      {t(
                        '未激活的乐透券可赠送；次日 00:00 自动激活；第三天 00:00 失效。',
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className='relative z-[1] mt-6 rounded-[22px] border border-slate-200 bg-white/88 p-5 shadow-sm'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <div className='flex items-center gap-3'>
                  <Typography.Text strong>{t('中奖动态')}</Typography.Text>
                  <Tag color='orange'>{t('稀有以上')}</Tag>
                </div>
                <Typography.Text type='tertiary' size='small'>
                  {recentWinsLoading
                    ? t('刷新中')
                    : t('最近 {{count}} 条', {
                        count: decoratedRecentWins.length,
                      })}
                </Typography.Text>
              </div>

              {decoratedRecentWins.length === 0 ? (
                <div className='mt-4 rounded-[18px] border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600'>
                  {t('等待第一位幸运儿')}
                </div>
              ) : (
                <div className='mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4'>
                  {decoratedRecentWins.map((win) => {
                    const WinIcon = win.icon || Gift;
                    return (
                      <div
                        key={win.id}
                        className='rounded-[18px] border bg-white/96 p-4'
                        style={{
                          borderColor: `${win.accent}35`,
                          boxShadow: `0 12px 28px ${win.glow}`,
                        }}
                      >
                        <div className='flex items-start justify-between gap-3'>
                          <div className='min-w-0'>
                            <div className='truncate text-sm font-semibold text-slate-900'>
                              {win.source_username || t('匿名用户')}
                            </div>
                            <div className='mt-1 text-xs text-slate-500'>
                              {formatTime(win.created_at)}
                            </div>
                          </div>
                          <div
                            className='flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-white'
                            style={{ color: win.accent }}
                          >
                            <WinIcon size={18} />
                          </div>
                        </div>
                        <div
                          className='mt-4 inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold'
                          style={{
                            color: win.accent,
                            borderColor: `${win.accent}4d`,
                          }}
                        >
                          <span>{win.name}</span>
                        </div>
                        <div className='mt-3 text-[26px] font-bold leading-none text-slate-900'>
                          {win.amount}
                          <span className='ml-2 text-sm font-medium text-slate-500'>
                            {t('乐透券')}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            <div className='relative z-[1] mt-6 rounded-[22px] border border-slate-200 bg-white/88 p-5 shadow-sm'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <Typography.Text strong>{t('我的乐透券')}</Typography.Text>
                <Typography.Text type='tertiary'>
                  {t('概率合计')} {totalProbability}%
                </Typography.Text>
              </div>

              {rewards.length === 0 ? (
                <div className='mt-4 rounded-[18px] border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600'>
                  {t(
                    '当前还没有乐透券。抽奖后、收到赠送后，或者进入消费日后，这里会展示你的券。',
                  )}
                </div>
              ) : (
                <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
                  {rewards.map((reward) => {
                    const status = rewardStatus(reward, t);
                    const RewardIcon = reward.icon || Gift;
                    return (
                      <div
                        key={reward.id}
                        className='rounded-[22px] border bg-white/96 p-4'
                        style={{ borderColor: `${reward.accent}35` }}
                      >
                        <div className='flex items-start justify-between gap-3'>
                          <div>
                            <div
                              className='inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold'
                              style={{
                                color: reward.accent,
                                borderColor: `${reward.accent}4d`,
                              }}
                            >
                              <RewardIcon size={14} />
                              <span>{reward.name}</span>
                            </div>
                            <div className='mt-3 text-[30px] font-bold leading-none text-slate-900'>
                              {reward.amount}
                              <span className='ml-2 text-sm font-medium text-slate-500'>
                                {t('乐透券')}
                              </span>
                            </div>
                          </div>
                          <Tag color={status.color}>{status.text}</Tag>
                        </div>

                        <div className='mt-4 grid gap-2 text-sm text-slate-600'>
                          <div>
                            {t('剩余额度')} $
                            {formatAmount(reward.remaining_amount)}
                          </div>
                          <div>
                            {t('抽奖序号')} #{reward.source_draw_index}
                          </div>
                          <div>
                            {t('获得时间')} {formatTime(reward.created_at)}
                          </div>
                          {reward.activated_at ? (
                            <div>
                              {t('激活时间')} {formatTime(reward.activated_at)}
                            </div>
                          ) : null}
                          {reward.gifted_by_username ? (
                            <div>
                              {t('来自赠送')} {reward.gifted_by_username}
                            </div>
                          ) : null}
                          {reward.gifted_to_username ? (
                            <div>
                              {t('赠送给')} {reward.gifted_to_username}
                            </div>
                          ) : null}
                          <div>
                            {t('失效时间')} {formatTime(reward.expires_at)}
                          </div>
                        </div>

                        {reward.can_gift ? (
                          <div className='mt-4'>
                            <Button
                              theme='light'
                              type='primary'
                              onClick={() => openGiftModal(reward)}
                            >
                              {t('赠送给用户')}
                            </Button>
                          </div>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </Card>
      </div>

      <Modal
        title={t('赠送乐透券')}
        visible={giftModalVisible}
        onOk={handleGift}
        onCancel={closeGiftModal}
        confirmLoading={giftLoading}
        maskClosable={!giftLoading}
        centered
      >
        <div className='space-y-4'>
          <div className='rounded-[18px] border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600'>
            <div className='font-semibold text-slate-900'>
              {giftReward
                ? `${giftReward.name} · ${giftReward.amount}${t('乐透券')}`
                : t('未选择乐透券')}
            </div>
            <div className='mt-2'>
              {t('只能赠送给精确用户名，且这张券必须保持未激活状态。')}
            </div>
          </div>
          <div>
            <Typography.Text strong className='mb-2 block'>
              {t('目标用户名')}
            </Typography.Text>
            <Input
              value={giftUsername}
              onChange={setGiftUsername}
              placeholder={t('请输入精确用户名')}
              autoFocus
            />
          </div>
        </div>
      </Modal>

      <style>{`
        .lottery-machine {
          position: relative;
          overflow: hidden;
          min-height: 520px;
          border-radius: 24px;
          border: 1px solid rgba(255,255,255,0.72);
        }

        .lottery-machine.is-drawing {
          animation: lotteryMachineShake 0.9s ease-in-out infinite;
        }

        .lottery-ring {
          position: absolute;
          border-radius: 999px;
          border: 1px solid;
        }

        .lottery-ring-a {
          width: 260px;
          height: 260px;
          top: 54px;
          left: calc(50% - 130px);
          animation: lotteryOrbit 7s linear infinite;
        }

        .lottery-ring-b {
          width: 340px;
          height: 340px;
          top: 14px;
          left: calc(50% - 170px);
          animation: lotteryOrbitReverse 12s linear infinite;
        }

        .lottery-prize {
          position: relative;
          overflow: hidden;
          width: min(100%, 340px);
          min-height: 250px;
          border-radius: 28px;
          border: 1px solid;
          padding: 26px 24px 22px;
          text-align: center;
          transform-origin: center;
        }

        .lottery-prize.is-drawing {
          animation: lotteryPrizeFlip 0.8s ease-in-out infinite;
        }

        .lottery-prize.is-revealed {
          animation: lotteryPrizePop 0.58s cubic-bezier(.2,.9,.2,1);
        }

        .lottery-shine {
          position: absolute;
          inset: -40%;
          transform: rotate(22deg);
          opacity: 0;
        }

        .lottery-prize.is-drawing .lottery-shine,
        .lottery-prize.is-revealed .lottery-shine {
          animation: lotteryShine 1.3s ease-in-out infinite;
        }

        @keyframes lotteryMachineShake {
          0% { transform: translate3d(0,0,0) rotate(0deg); }
          25% { transform: translate3d(-2px,0,0) rotate(-1deg); }
          50% { transform: translate3d(3px,0,0) rotate(1deg); }
          75% { transform: translate3d(-2px,0,0) rotate(-1deg); }
          100% { transform: translate3d(0,0,0) rotate(0deg); }
        }

        @keyframes lotteryPrizeFlip {
          0% { transform: rotateY(0deg) scale(1); }
          50% { transform: rotateY(180deg) scale(0.98); }
          100% { transform: rotateY(360deg) scale(1); }
        }

        @keyframes lotteryPrizePop {
          0% { transform: scale(0.88) translateY(14px); opacity: 0; }
          70% { transform: scale(1.04) translateY(-6px); opacity: 1; }
          100% { transform: scale(1) translateY(0); opacity: 1; }
        }

        @keyframes lotteryShine {
          0% { opacity: 0; transform: translateX(-28%) rotate(22deg); }
          30% { opacity: 0.7; }
          100% { opacity: 0; transform: translateX(28%) rotate(22deg); }
        }

        @keyframes lotteryOrbit {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }

        @keyframes lotteryOrbitReverse {
          from { transform: rotate(360deg); }
          to { transform: rotate(0deg); }
        }
      `}</style>
    </div>
  );
};

export default Lottery;
