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

import React, { useMemo, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  Divider,
  Select,
  Skeleton,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { renderQuota } from '../../helpers';
import { formatSubscriptionQuotaLabel } from '../../helpers/subscriptionFormat';

const { Text } = Typography;


const SubscriptionPlansCard = ({
  t,
  loading = false,
  plans = [],
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
  showTopupStore = false,
  topUpLink = '',
  withCard = true,
}) => {
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await reloadSubscriptionSelf?.();
    } finally {
      setRefreshing(false);
    }
  };

  const hasActiveSubscription = activeSubscriptions.length > 0;
  const hasAnySubscription = allSubscriptions.length > 0;
  const disableSubscriptionPreference = !hasActiveSubscription;
  const isSubscriptionPreference =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only';
  const displayBillingPreference =
    disableSubscriptionPreference && isSubscriptionPreference
      ? 'wallet_first'
      : billingPreference;
  const subscriptionPreferenceLabel =
    billingPreference === 'subscription_only' ? t('仅用订阅') : t('优先订阅');

  const planTitleMap = useMemo(() => {
    const map = new Map();
    (plans || []).forEach((item) => {
      const plan = item?.plan;
      if (!plan?.id) return;
      map.set(plan.id, plan.title || '');
    });
    return map;
  }, [plans]);

  const getRemainingDays = (sub) => {
    if (!sub?.subscription?.end_time) return 0;
    const now = Date.now() / 1000;
    const remaining = sub.subscription.end_time - now;
    return Math.max(0, Math.ceil(remaining / 86400));
  };

  const getUsagePercent = (sub) => {
    const total = Number(sub?.subscription?.amount_total || 0);
    const used = Number(sub?.subscription?.amount_used || 0);
    if (total <= 0) return 0;
    return Math.round((used / total) * 100);
  };

  const cardContent = loading ? (
    <div className='space-y-4'>
      <Card className='!rounded-xl w-full' bodyStyle={{ padding: '12px' }}>
        <div className='flex items-center justify-between mb-3'>
          <Skeleton.Title active style={{ width: 100, height: 20 }} />
          <Skeleton.Button active style={{ width: 24, height: 24 }} />
        </div>
        <div className='space-y-2'>
          <Skeleton.Paragraph active rows={2} />
        </div>
      </Card>
      <Card className='!rounded-xl w-full' bodyStyle={{ padding: '16px' }}>
        <Skeleton.Paragraph active rows={4} />
      </Card>
    </div>
  ) : (
    <Space vertical style={{ width: '100%' }} spacing={8}>
      <Card className='!rounded-xl w-full' bodyStyle={{ padding: '12px' }}>
        <div className='flex items-center justify-between mb-2 gap-3'>
          <div className='flex items-center gap-2 flex-1 min-w-0'>
            <Text strong>{t('我的订阅')}</Text>
            {hasActiveSubscription ? (
              <Tag
                color='white'
                size='small'
                shape='circle'
                prefixIcon={<Badge dot type='success' />}
              >
                {activeSubscriptions.length} {t('个生效中')}
              </Tag>
            ) : (
              <Tag color='white' size='small' shape='circle'>
                {t('无生效')}
              </Tag>
            )}
            {allSubscriptions.length > activeSubscriptions.length && (
              <Tag color='white' size='small' shape='circle'>
                {allSubscriptions.length - activeSubscriptions.length} {t('个已过期')}
              </Tag>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Select
              value={displayBillingPreference}
              onChange={onChangeBillingPreference}
              size='small'
              optionList={[
                {
                  value: 'subscription_first',
                  label: disableSubscriptionPreference
                    ? `${t('优先订阅')} (${t('无生效')})`
                    : t('优先订阅'),
                  disabled: disableSubscriptionPreference,
                },
                { value: 'wallet_first', label: t('优先钱包') },
                {
                  value: 'subscription_only',
                  label: disableSubscriptionPreference
                    ? `${t('仅用订阅')} (${t('无生效')})`
                    : t('仅用订阅'),
                  disabled: disableSubscriptionPreference,
                },
                { value: 'wallet_only', label: t('仅用钱包') },
              ]}
            />
            <Button
              size='small'
              theme='light'
              type='tertiary'
              icon={
                <RefreshCw size={12} className={refreshing ? 'animate-spin' : ''} />
              }
              onClick={handleRefresh}
              loading={refreshing}
            />
          </div>
        </div>

        {disableSubscriptionPreference && isSubscriptionPreference && (
          <Text type='tertiary' size='small'>
            {t('已保存偏好为')}
            {subscriptionPreferenceLabel}
            {t('，当前无生效订阅，将自动使用钱包')}
          </Text>
        )}

        {hasAnySubscription ? (
          <>
            <Divider margin={8} />
            <div className='max-h-64 overflow-y-auto pr-1 semi-table-body'>
              {allSubscriptions.map((sub, subIndex) => {
                const isLast = subIndex === allSubscriptions.length - 1;
                const subscription = sub.subscription;
                const totalAmount = Number(subscription?.amount_total || 0);
                const usedAmount = Number(subscription?.amount_used || 0);
                const remainAmount =
                  totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0;
                const planTitle = planTitleMap.get(subscription?.plan_id) || '';
                const remainDays = getRemainingDays(sub);
                const usagePercent = getUsagePercent(sub);
                const now = Date.now() / 1000;
                const isExpired = (subscription?.end_time || 0) < now;
                const isCancelled = subscription?.status === 'cancelled';
                const isActive = subscription?.status === 'active' && !isExpired;

                return (
                  <div key={subscription?.id || subIndex}>
                    <div className='flex items-center justify-between text-xs mb-2'>
                      <div className='flex items-center gap-2'>
                        <span className='font-medium'>
                          {planTitle
                            ? `${planTitle} · ${t('订阅')} #${subscription?.id}`
                            : `${t('订阅')} #${subscription?.id}`}
                        </span>
                        {isActive ? (
                          <Tag
                            color='white'
                            size='small'
                            shape='circle'
                            prefixIcon={<Badge dot type='success' />}
                          >
                            {t('生效')}
                          </Tag>
                        ) : isCancelled ? (
                          <Tag color='white' size='small' shape='circle'>
                            {t('已作废')}
                          </Tag>
                        ) : (
                          <Tag color='white' size='small' shape='circle'>
                            {t('已过期')}
                          </Tag>
                        )}
                      </div>
                      {isActive && (
                        <span className='text-gray-500'>
                          {t('剩余')} {remainDays} {t('天')}
                        </span>
                      )}
                    </div>
                    <div className='text-xs text-gray-500 mb-2'>
                      {isActive
                        ? t('至')
                        : isCancelled
                          ? t('作废于')
                          : t('过期于')}{' '}
                      {new Date((subscription?.end_time || 0) * 1000).toLocaleString()}
                    </div>
                    <div className='text-xs text-gray-500 mb-2'>
                      {formatSubscriptionQuotaLabel(subscription, t, true)}:{' '}
                      {totalAmount > 0 ? (
                        <Tooltip
                          content={`${t('原生额度')}：${usedAmount}/${totalAmount} · ${t('剩余')} ${remainAmount}`}
                        >
                          <span>
                            {renderQuota(usedAmount)}/{renderQuota(totalAmount)} · {t('剩余')}{' '}
                            {renderQuota(remainAmount)}
                          </span>
                        </Tooltip>
                      ) : (
                        t('不限')
                      )}
                      {totalAmount > 0 && (
                        <span className='ml-2'>
                          {t('已用')} {usagePercent}%
                        </span>
                      )}
                    </div>
                    {!isLast && <Divider margin={12} />}
                  </div>
                );
              })}
            </div>
          </>
        ) : (
          <div className='text-xs text-gray-500 mt-2'>
            {showTopupStore
              ? t('当前暂无订阅，购买兑换码后可在上方完成兑换。')
              : t('当前暂无订阅，获取兑换码后可在上方完成兑换。')}
          </div>
        )}
      </Card>

      {showTopupStore && topUpLink ? (
        <Card className='!rounded-xl w-full' bodyStyle={{ padding: '16px' }}>
          <div className='flex items-center justify-between gap-3 mb-4'>
            <Text strong>{t('购买方式')}</Text>
            <Button
              size='small'
              theme='solid'
              type='primary'
              onClick={() => window.open(topUpLink, '_blank', 'noopener,noreferrer')}
            >
              {t('打开店铺')}
            </Button>
          </div>

          <div className='grid grid-cols-1 md:grid-cols-[minmax(0,1.4fr)_220px] gap-5 items-start'>
            <div className='space-y-4'>
              <div>
                <Text type='tertiary' size='small'>
                  {t('店铺地址')}
                </Text>
                <div className='mt-2 break-all'>
                  <a
                    href={topUpLink}
                    target='_blank'
                    rel='noreferrer'
                    className='text-blue-600 underline'
                  >
                    {topUpLink}
                  </a>
                </div>
              </div>

              <div className='text-sm text-gray-500 leading-7'>
                <div>{t('请先在店铺购买兑换码。')}</div>
                <div>{t('购买后复制兑换码，回到本页完成充值或开通订阅。')}</div>
              </div>
            </div>

            <div className='flex flex-col items-start md:items-center gap-2'>
              <Text type='tertiary' size='small'>
                {t('二维码')}
              </Text>
              <div className='rounded-xl border border-gray-200 bg-white p-3 shadow-sm'>
                <QRCodeSVG value={topUpLink} size={180} includeMargin />
              </div>
            </div>
          </div>
        </Card>
      ) : null}
    </Space>
  );

  return withCard ? (
    <Card className='!rounded-2xl shadow-sm border-0'>{cardContent}</Card>
  ) : (
    <div className='space-y-3'>{cardContent}</div>
  );
};

export default SubscriptionPlansCard;
