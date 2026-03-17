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

import React from 'react';
import {
  Avatar,
  Banner,
  Button,
  Card,
  Form,
  Space,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { BarChart2, CreditCard, TrendingUp, Wallet } from 'lucide-react';
import { IconGift } from '@douyinfe/semi-icons';
import SubscriptionPlansCard from './SubscriptionPlansCard';

const { Text } = Typography;

const SHOP_URL = 'https://pay.ldxp.cn/shop/5J1Y8A0I';

const RechargeCard = ({
  t,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  userState,
  renderQuota,
  statusLoading,
  subscriptionLoading = false,
  subscriptionPlans = [],
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
}) => {
  const unifiedHint = statusLoading ? (
    <div className='py-4 flex justify-center'>
      <Spin size='large' />
    </div>
  ) : (
    <Banner
      type='info'
      closeIcon={null}
      className='!rounded-xl'
      description={
        <div className='text-sm leading-6'>
          <div>{t('订阅和额度均通过兑换码完成。')}</div>
          <div>{t('请先在店铺购买兑换码，再回到本页完成充值或开通订阅。')}</div>
        </div>
      }
    />
  );

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex items-center justify-between mb-4'>
        <div className='flex items-center'>
          <Avatar size='small' color='blue' className='mr-3 shadow-md'>
            <CreditCard size={16} />
          </Avatar>
          <div>
            <Typography.Text className='text-lg font-medium'>
              {t('账户充值')}
            </Typography.Text>
            <div className='text-xs'>
              {t('订阅和额度均通过兑换码完成')}
            </div>
          </div>
        </div>
      </div>

      <Space vertical style={{ width: '100%' }} spacing={12}>
        <Card
          className='!rounded-xl w-full'
          cover={
            <div
              className='relative h-30'
              style={{
                '--palette-primary-darkerChannel': '37 99 235',
                backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
                backgroundSize: 'cover',
                backgroundPosition: 'center',
                backgroundRepeat: 'no-repeat',
              }}
            >
              <div className='relative z-10 h-full flex flex-col justify-between p-4'>
                <div className='flex justify-between items-center'>
                  <Text strong style={{ color: 'white', fontSize: '16px' }}>
                    {t('账户统计')}
                  </Text>
                </div>

                <div className='grid grid-cols-3 gap-6 mt-4'>
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.quota)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <Wallet
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('当前余额')}
                      </Text>
                    </div>
                  </div>

                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.used_quota)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <TrendingUp
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('历史消耗')}
                      </Text>
                    </div>
                  </div>

                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {userState?.user?.request_count || 0}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <BarChart2
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('请求次数')}
                      </Text>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          }
        >
          {unifiedHint}
        </Card>

        <Card
          className='!rounded-xl w-full'
          title={
            <Text type='tertiary' strong>
              {t('兑换码充值')}
            </Text>
          }
        >
          <Form initValues={{ redemptionCode }}>
            <Form.Input
              field='redemptionCode'
              noLabel
              placeholder={t('请输入兑换码')}
              value={redemptionCode}
              onChange={(value) => setRedemptionCode(value)}
              prefix={<IconGift />}
              suffix={
                <div className='flex items-center gap-2'>
                  <Button
                    type='primary'
                    theme='solid'
                    onClick={topUp}
                    loading={isSubmitting}
                  >
                    {t('立即兑换')}
                  </Button>
                </div>
              }
              showClear
              style={{ width: '100%' }}
              extraText={
                <Text type='tertiary'>
                  {t('还没有兑换码？')}
                  <a
                    href={SHOP_URL}
                    target='_blank'
                    rel='noreferrer'
                    className='ml-1 underline'
                  >
                    {t('前往店铺购买')}
                  </a>
                </Text>
              }
            />
          </Form>
        </Card>

        <SubscriptionPlansCard
          t={t}
          loading={subscriptionLoading}
          plans={subscriptionPlans}
          billingPreference={billingPreference}
          onChangeBillingPreference={onChangeBillingPreference}
          activeSubscriptions={activeSubscriptions}
          allSubscriptions={allSubscriptions}
          reloadSubscriptionSelf={reloadSubscriptionSelf}
          withCard={false}
        />
      </Space>
    </Card>
  );
};

export default RechargeCard;
