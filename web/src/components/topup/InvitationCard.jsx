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
  Typography,
  Card,
  Button,
  Input,
  Badge,
  Space,
} from '@douyinfe/semi-ui';
import { Copy, Users, BarChart2, TrendingUp, Gift, Zap } from 'lucide-react';

const { Text } = Typography;

const formatInviteUsd = (value) => {
  const amount = Number(value || 0);
  return `$${amount.toLocaleString(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  })}`;
};

const formatInviteLevel = (level) => `LV ${Number(level || 0)}`;

const formatInviteTime = (timestamp) => {
  if (!timestamp) {
    return '-';
  }
  return new Date(timestamp * 1000).toLocaleString();
};

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  inviteMode,
  showInviteActions,
  affLink,
  handleAffLinkClick,
  registrationInviteInfo,
  registrationInviteLoading,
  onGenerateRegistrationInviteCode,
  onCopyRegistrationInviteCode,
}) => {
  if (inviteMode === 'one_time') {
    const isAdminUnlimited = !!registrationInviteInfo?.is_admin_unlimited;
    const hasCode = !!registrationInviteInfo?.code;
    const canInvite = !!registrationInviteInfo?.can_invite;
    const canGenerate = !!registrationInviteInfo?.can_generate;
    const cycleUsageText = isAdminUnlimited
      ? t('0/不限')
      : `${registrationInviteInfo?.used_slots || 0}/${registrationInviteInfo?.total_slots || 0}`;
    const currentLevelText = isAdminUnlimited
      ? t('管理员')
      : formatInviteLevel(registrationInviteInfo?.invite_level);

    const renderOneTimeInviteActions = () => {
      if (!showInviteActions) {
        return (
          <Text type='tertiary' className='text-sm'>
            {t('当前角色不显示邀请码操作')}
          </Text>
        );
      }

      if (hasCode) {
        return (
          <Space vertical style={{ width: '100%' }}>
            <Input
              value={registrationInviteInfo.code}
              readOnly
              className='!rounded-lg'
              suffix={
                <Button
                  type='primary'
                  theme='solid'
                  onClick={onCopyRegistrationInviteCode}
                  icon={<Copy size={14} />}
                  className='!rounded-lg'
                >
                  {t('复制')}
                </Button>
              }
            />
            {canGenerate && (
              <Button
                type='primary'
                theme='solid'
                loading={registrationInviteLoading}
                onClick={onGenerateRegistrationInviteCode}
                className='!rounded-lg'
              >
                {t('生成邀请码')}
              </Button>
            )}
          </Space>
        );
      }

      if (canGenerate) {
        return (
          <Button
            type='primary'
            theme='solid'
            loading={registrationInviteLoading}
            onClick={onGenerateRegistrationInviteCode}
            className='!rounded-lg'
          >
            {t('生成邀请码')}
          </Button>
        );
      }

      if (canInvite) {
        return (
          <Text type='tertiary' className='text-sm'>
            {t('当前周期邀请名额已用完')}
          </Text>
        );
      }

      return (
        <Text type='tertiary' className='text-sm'>
          {t('当前等级未达到邀请条件')}
        </Text>
      );
    };

    return (
      <Card className='!rounded-2xl shadow-sm border-0'>
        <Space vertical style={{ width: '100%' }} size='large'>
          <div>
            <Typography.Text className='text-lg font-medium'>
              {t('用户邀请等级')}
            </Typography.Text>
          </div>

          <Card className='!rounded-xl w-full'>
            <div className='space-y-3'>
              <div className='flex items-center justify-between gap-3'>
                <Text type='tertiary'>{t('当前等级')}</Text>
                <Text strong>{currentLevelText}</Text>
              </div>
              {!isAdminUnlimited && (
                <>
                  <div className='flex items-center justify-between gap-3'>
                    <Text type='tertiary'>{t('累计消耗金额')}</Text>
                    <Text strong>
                      {formatInviteUsd(registrationInviteInfo?.consumed_amount_usd)}
                    </Text>
                  </div>
                  <div className='flex items-center justify-between gap-3'>
                    <Text type='tertiary'>{t('下一等级')}</Text>
                    <Text>
                      {formatInviteLevel(registrationInviteInfo?.next_level)} ·{' '}
                      {t('还差')}{' '}
                      {formatInviteUsd(
                        registrationInviteInfo?.amount_to_next_level_usd,
                      )}
                    </Text>
                  </div>
                </>
              )}
              <div className='flex items-center justify-between gap-3'>
                <Text type='tertiary'>
                  {t('当前被邀请注册新用户初始额度')}
                </Text>
                <Text strong>
                  {formatInviteUsd(
                    registrationInviteInfo?.current_new_user_quota_usd,
                  )}
                </Text>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <Text type='tertiary'>{t('本周期使用情况')}</Text>
                <Text>{cycleUsageText}</Text>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <Text type='tertiary'>{t('当前周期')}</Text>
                <Text>
                  {formatInviteTime(registrationInviteInfo?.cycle_started_at)}{' '}
                  ~ {formatInviteTime(registrationInviteInfo?.cycle_ends_at)}
                </Text>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <Text type='tertiary'>{t('下次刷新')}</Text>
                <Text>{formatInviteTime(registrationInviteInfo?.next_refresh_at)}</Text>
              </div>
            </div>
          </Card>

          <Card className='!rounded-xl w-full' title={t('生成邀请码')}>
            <Space vertical style={{ width: '100%' }}>
              <Input
                value={registrationInviteInfo?.code || t('当前暂无邀请码')}
                readOnly
                className='!rounded-lg'
              />
              {renderOneTimeInviteActions()}
            </Space>
          </Card>

          <Card className='!rounded-xl w-full' title={<Text type='tertiary'>{t('邀请规则')}</Text>}>
            <div className='space-y-3'>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('LV3 以上可邀请用户')}
                </Text>
              </div>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('LV3 每个周期 1 个邀请名额，每升一级多 1 个')}
                </Text>
              </div>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('邀请名额按自然月周期统一刷新')}
                </Text>
              </div>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('管理员当前设置为每')} {registrationInviteInfo?.cycle_months || 1}{' '}
                  {t('个月刷新一次')}
                </Text>
              </div>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('每个邀请码仅可使用 1 次')}
                </Text>
              </div>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('邀请码到当前周期结束时自动失效')}
                </Text>
              </div>
            </div>
          </Card>
        </Space>
      </Card>
    );
  }

  const renderInviteActions = () => {
    if (!showInviteActions) {
      return (
        <Text type='tertiary' className='text-sm'>
          {t('当前角色不显示邀请操作入口')}
        </Text>
      );
    }

    return (
      <Input
        value={affLink}
        readonly
        className='!rounded-lg'
        prefix={t('邀请链接')}
        suffix={
          <Button
            type='primary'
            theme='solid'
            onClick={handleAffLinkClick}
            icon={<Copy size={14} />}
            className='!rounded-lg'
          >
            {t('复制')}
          </Button>
        }
      />
    );
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='green' className='mr-3 shadow-md'>
          <Gift size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('邀请奖励')}
          </Typography.Text>
          <div className='text-xs'>{t('邀请好友获得额外奖励')}</div>
        </div>
      </div>

      {/* 收益展示区域 */}
      <Space vertical style={{ width: '100%' }}>
        {/* 统计数据统一卡片 */}
        <Card
          className='!rounded-xl w-full'
          cover={
            <div
              className='relative h-30'
              style={{
                '--palette-primary-darkerChannel': '0 75 80',
                backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
                backgroundSize: 'cover',
                backgroundPosition: 'center',
                backgroundRepeat: 'no-repeat',
              }}
            >
              {/* 标题和按钮 */}
              <div className='relative z-10 h-full flex flex-col justify-between p-4'>
                <div className='flex justify-between items-center'>
                  <Text strong style={{ color: 'white', fontSize: '16px' }}>
                    {t('收益统计')}
                  </Text>
                  <Button
                    type='primary'
                    theme='solid'
                    size='small'
                    disabled={
                      !userState?.user?.aff_quota ||
                      userState?.user?.aff_quota <= 0
                    }
                    onClick={() => setOpenTransfer(true)}
                    className='!rounded-lg'
                  >
                    <Zap size={12} className='mr-1' />
                    {t('划转到余额')}
                  </Button>
                </div>

                {/* 统计数据 */}
                <div className='grid grid-cols-3 gap-6 mt-4'>
                  {/* 待使用收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.aff_quota || 0)}
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
                        {t('待使用收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 总收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.aff_history_quota || 0)}
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
                        {t('总收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 邀请人数 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {userState?.user?.aff_count || 0}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <Users
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
                        {t('邀请人数')}
                      </Text>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          }
        >
          {renderInviteActions()}
        </Card>

        {/* 奖励说明 */}
        <Card
          className='!rounded-xl w-full'
          title={<Text type='tertiary'>{t('奖励说明')}</Text>}
        >
          <div className='space-y-3'>
            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {inviteMode === 'one_time'
                  ? t('每个周期可生成一个一次性邀请码，每个邀请码只能使用一次')
                  : t('邀请好友注册，好友充值后您可获得相应奖励')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('通过划转功能将奖励额度转入到您的账户余额中')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请的好友越多，获得的奖励越多')}
              </Text>
            </div>
          </div>
        </Card>
      </Space>
    </Card>
  );
};

export default InvitationCard;
