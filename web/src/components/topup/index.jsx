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

import React, { useEffect, useState, useContext } from 'react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  renderQuota,
  renderQuotaWithAmount,
  copy,
  getQuotaPerUnit,
} from '../../helpers';
import { Modal, Toast } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import RechargeCard from './RechargeCard';
import InvitationCard from './InvitationCard';
import TransferModal from './modals/TransferModal';
import PaymentConfirmModal from './modals/PaymentConfirmModal';
import TopupHistoryModal from './modals/TopupHistoryModal';

const emptyRegistrationInviteInfo = {
  code: '',
  expires_at: 0,
  used_at: 0,
  revoked_at: 0,
  can_invite: false,
  can_generate: false,
  is_admin_unlimited: false,
  invite_level: 0,
  consumed_amount_usd: 0,
  next_level: 1,
  next_level_threshold_usd: 100,
  amount_to_next_level_usd: 100,
  used_slots: 0,
  total_slots: 0,
  cycle_months: 1,
  cycle_started_at: 0,
  cycle_ends_at: 0,
  next_refresh_at: 0,
  current_new_user_quota: 0,
  current_new_user_quota_usd: 0,
  next_available_at: 0,
};
const registrationInviteRefreshIntervalMs = 30000;

const TopUp = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const showInvitationCard = statusState?.status?.show_invitation_card ?? true;

  const [redemptionCode, setRedemptionCode] = useState('');
  const [amount, setAmount] = useState(0.0);
  const [minTopUp, setMinTopUp] = useState(statusState?.status?.min_topup || 1);
  const [topUpCount, setTopUpCount] = useState(
    statusState?.status?.min_topup || 1,
  );
  const [topUpLink, setTopUpLink] = useState(
    statusState?.status?.top_up_link || '',
  );
  const showTopupStore = (statusState?.status?.show_topup_store ?? false) && !!topUpLink;
  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(
    statusState?.status?.enable_online_topup || false,
  );
  const [priceRatio, setPriceRatio] = useState(statusState?.status?.price || 1);

  const [enableStripeTopUp, setEnableStripeTopUp] = useState(
    statusState?.status?.enable_stripe_topup || false,
  );
  const [statusLoading, setStatusLoading] = useState(true);

  // Creem 相关状态
  const [creemProducts, setCreemProducts] = useState([]);
  const [enableCreemTopUp, setEnableCreemTopUp] = useState(false);
  const [creemOpen, setCreemOpen] = useState(false);
  const [selectedCreemProduct, setSelectedCreemProduct] = useState(null);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [open, setOpen] = useState(false);
  const [payWay, setPayWay] = useState('');
  const [amountLoading, setAmountLoading] = useState(false);
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [payMethods, setPayMethods] = useState([]);

  // 邀请相关状态
  const [affLink, setAffLink] = useState('');
  const [registrationInviteInfo, setRegistrationInviteInfo] = useState(
    emptyRegistrationInviteInfo,
  );
  const [registrationInviteLoading, setRegistrationInviteLoading] =
    useState(false);
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);

  // 账单Modal状态
  const [openHistory, setOpenHistory] = useState(false);

  // 订阅相关
  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [subscriptionLoading, setSubscriptionLoading] = useState(true);
  const [billingPreference, setBillingPreference] =
    useState('subscription_first');
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);

  // 预设充值额度选项
  const [presetAmounts, setPresetAmounts] = useState([]);
  const [selectedPreset, setSelectedPreset] = useState(null);

  // 充值配置信息
  const [topupInfo, setTopupInfo] = useState({
    amount_options: [],
    discount: {},
  });

  const topUp = async () => {
    if (redemptionCode === '') {
      showInfo(t('请输入兑换码！'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('兑换成功！'));
        if (data?.redeem_type === 'subscription') {
          Modal.success({
            title: t('兑换成功！'),
            content:
              t('成功兑换订阅：') +
              (data.subscription_plan_title || t('订阅套餐')) +
              '，' +
              t('有效期至：') +
              new Date((data.subscription_end_time || 0) * 1000).toLocaleString(),
            centered: true,
          });
          await getSubscriptionSelf();
        } else {
          const quota = typeof data === 'number' ? data : Number(data?.quota || 0);
          Modal.success({
            title: t('兑换成功！'),
            content: t('成功兑换额度：') + renderQuota(quota),
            centered: true,
          });
          if (userState.user) {
            const updatedUser = {
              ...userState.user,
              quota: userState.user.quota + quota,
            };
            userDispatch({ type: 'login', payload: updatedUser });
          }
        }
        setRedemptionCode('');
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('请求失败'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('超级管理员未设置充值链接！'));
      return;
    }
    window.open(topUpLink, '_blank');
  };

  const preTopUp = async (payment) => {
    if (payment === 'stripe') {
      if (!enableStripeTopUp) {
        showError(t('管理员未开启Stripe充值！'));
        return;
      }
    } else {
      if (!enableOnlineTopUp) {
        showError(t('管理员未开启在线充值！'));
        return;
      }
    }

    setPayWay(payment);
    setPaymentLoading(true);
    try {
      if (payment === 'stripe') {
        await getStripeAmount();
      } else {
        await getAmount();
      }

      if (topUpCount < minTopUp) {
        showError(t('充值数量不能小于') + minTopUp);
        return;
      }
      setOpen(true);
    } catch (error) {
      showError(t('获取金额失败'));
    } finally {
      setPaymentLoading(false);
    }
  };

  const onlineTopUp = async () => {
    if (payWay === 'stripe') {
      // Stripe 支付处理
      if (amount === 0) {
        await getStripeAmount();
      }
    } else {
      // 普通支付处理
      if (amount === 0) {
        await getAmount();
      }
    }

    if (topUpCount < minTopUp) {
      showError('充值数量不能小于' + minTopUp);
      return;
    }
    setConfirmLoading(true);
    try {
      let res;
      if (payWay === 'stripe') {
        // Stripe 支付请求
        res = await API.post('/api/user/stripe/pay', {
          amount: parseInt(topUpCount),
          payment_method: 'stripe',
        });
      } else {
        // 普通支付请求
        res = await API.post('/api/user/pay', {
          amount: parseInt(topUpCount),
          payment_method: payWay,
        });
      }

      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          if (payWay === 'stripe') {
            // Stripe 支付回调处理
            window.open(data.pay_link, '_blank');
          } else {
            // 普通支付表单提交
            let params = data;
            let url = res.data.url;
            let form = document.createElement('form');
            form.action = url;
            form.method = 'POST';
            let isSafari =
              navigator.userAgent.indexOf('Safari') > -1 &&
              navigator.userAgent.indexOf('Chrome') < 1;
            if (!isSafari) {
              form.target = '_blank';
            }
            for (let key in params) {
              let input = document.createElement('input');
              input.type = 'hidden';
              input.name = key;
              input.value = params[key];
              form.appendChild(input);
            }
            document.body.appendChild(form);
            form.submit();
            document.body.removeChild(form);
          }
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('支付失败');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
      showError(t('支付请求失败'));
    } finally {
      setOpen(false);
      setConfirmLoading(false);
    }
  };

  const creemPreTopUp = async (product) => {
    if (!enableCreemTopUp) {
      showError(t('管理员未开启 Creem 充值！'));
      return;
    }
    setSelectedCreemProduct(product);
    setCreemOpen(true);
  };

  const onlineCreemTopUp = async () => {
    if (!selectedCreemProduct) {
      showError(t('请选择产品'));
      return;
    }
    // Validate product has required fields
    if (!selectedCreemProduct.productId) {
      showError(t('产品配置错误，请联系管理员'));
      return;
    }
    setConfirmLoading(true);
    try {
      const res = await API.post('/api/user/creem/pay', {
        product_id: selectedCreemProduct.productId,
        payment_method: 'creem',
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          processCreemCallback(data);
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('支付失败');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
      showError(t('支付请求失败'));
    } finally {
      setCreemOpen(false);
      setConfirmLoading(false);
    }
  };

  const processCreemCallback = (data) => {
    // 与 Stripe 保持一致的实现方式
    window.open(data.checkout_url, '_blank');
  };

  const getUserQuota = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getSubscriptionPlans = async () => {
    setSubscriptionLoading(true);
    try {
      const res = await API.get('/api/subscription/plans');
      if (res.data?.success) {
        setSubscriptionPlans(res.data.data || []);
      }
    } catch (e) {
      setSubscriptionPlans([]);
    } finally {
      setSubscriptionLoading(false);
    }
  };

  const getSubscriptionSelf = async () => {
    try {
      const res = await API.get('/api/subscription/self');
      if (res.data?.success) {
        setBillingPreference(
          res.data.data?.billing_preference || 'subscription_first',
        );
        // Active subscriptions
        const activeSubs = res.data.data?.subscriptions || [];
        setActiveSubscriptions(activeSubs);
        // All subscriptions (including expired)
        const allSubs = res.data.data?.all_subscriptions || [];
        setAllSubscriptions(allSubs);
      }
    } catch (e) {
      // ignore
    }
  };

  const updateBillingPreference = async (pref) => {
    const previousPref = billingPreference;
    setBillingPreference(pref);
    try {
      const res = await API.put('/api/subscription/self/preference', {
        billing_preference: pref,
      });
      if (res.data?.success) {
        showSuccess(t('更新成功'));
        const normalizedPref =
          res.data?.data?.billing_preference || pref || previousPref;
        setBillingPreference(normalizedPref);
      } else {
        showError(res.data?.message || t('更新失败'));
        setBillingPreference(previousPref);
      }
    } catch (e) {
      showError(t('请求失败'));
      setBillingPreference(previousPref);
    }
  };

  // 获取充值配置信息
  const getTopupInfo = async () => {
    try {
      const res = await API.get('/api/user/topup/info');
      const { message, data, success } = res.data;
      if (success) {
        setTopupInfo({
          amount_options: data.amount_options || [],
          discount: data.discount || {},
        });

        // 处理支付方式
        let payMethods = data.pay_methods || [];
        try {
          if (typeof payMethods === 'string') {
            payMethods = JSON.parse(payMethods);
          }
          if (payMethods && payMethods.length > 0) {
            // 检查name和type是否为空
            payMethods = payMethods.filter((method) => {
              return method.name && method.type;
            });
            // 如果没有color，则设置默认颜色
            payMethods = payMethods.map((method) => {
              // 规范化最小充值数
              const normalizedMinTopup = Number(method.min_topup);
              method.min_topup = Number.isFinite(normalizedMinTopup)
                ? normalizedMinTopup
                : 0;

              // Stripe 的最小充值从后端字段回填
              if (
                method.type === 'stripe' &&
                (!method.min_topup || method.min_topup <= 0)
              ) {
                const stripeMin = Number(data.stripe_min_topup);
                if (Number.isFinite(stripeMin)) {
                  method.min_topup = stripeMin;
                }
              }

              if (!method.color) {
                if (method.type === 'alipay') {
                  method.color = 'rgba(var(--semi-blue-5), 1)';
                } else if (method.type === 'wxpay') {
                  method.color = 'rgba(var(--semi-green-5), 1)';
                } else if (method.type === 'stripe') {
                  method.color = 'rgba(var(--semi-purple-5), 1)';
                } else {
                  method.color = 'rgba(var(--semi-primary-5), 1)';
                }
              }
              return method;
            });
          } else {
            payMethods = [];
          }

          // 如果启用了 Stripe 支付，添加到支付方法列表
          // 这个逻辑现在由后端处理，如果 Stripe 启用，后端会在 pay_methods 中包含它

          setPayMethods(payMethods);
          const enableStripeTopUp = data.enable_stripe_topup || false;
          const enableOnlineTopUp = data.enable_online_topup || false;
          const enableCreemTopUp = data.enable_creem_topup || false;
          const minTopUpValue = enableOnlineTopUp
            ? data.min_topup
            : enableStripeTopUp
              ? data.stripe_min_topup
              : 1;
          setEnableOnlineTopUp(enableOnlineTopUp);
          setEnableStripeTopUp(enableStripeTopUp);
          setEnableCreemTopUp(enableCreemTopUp);
          setMinTopUp(minTopUpValue);
          setTopUpCount(minTopUpValue);

          // 设置 Creem 产品
          try {
            console.log(' data is ?', data);
            console.log(' creem products is ?', data.creem_products);
            const products = JSON.parse(data.creem_products || '[]');
            setCreemProducts(products);
          } catch (e) {
            setCreemProducts([]);
          }

          // 如果没有自定义充值数量选项，根据最小充值金额生成预设充值额度选项
          if (topupInfo.amount_options.length === 0) {
            setPresetAmounts(generatePresetAmounts(minTopUpValue));
          }

          // 初始化显示实付金额
          getAmount(minTopUpValue);
        } catch (e) {
          console.log('解析支付方式失败:', e);
          setPayMethods([]);
        }

        // 如果有自定义充值数量选项，使用它们替换默认的预设选项
        if (data.amount_options && data.amount_options.length > 0) {
          const customPresets = data.amount_options.map((amount) => ({
            value: amount,
            discount: data.discount[amount] || 1.0,
          }));
          setPresetAmounts(customPresets);
        }
      } else {
        console.error('获取充值配置失败:', data);
      }
    } catch (error) {
      console.error('获取充值配置异常:', error);
    }
  };

  // 获取邀请链接
  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      let link = `${window.location.origin}/register?aff=${data}`;
      setAffLink(link);
    } else {
      showError(message);
    }
  };

  const getRegistrationInviteCode = async ({ silent = false } = {}) => {
    if (!silent) {
      setRegistrationInviteLoading(true);
    }
    try {
      const res = await API.get('/api/user/registration-invite-code');
      const { success, message, data } = res.data;
      if (success) {
        setRegistrationInviteInfo(data || emptyRegistrationInviteInfo);
      } else {
        setRegistrationInviteInfo(emptyRegistrationInviteInfo);
        if (!silent) {
          showError(message);
        }
      }
    } catch (error) {
      setRegistrationInviteInfo(emptyRegistrationInviteInfo);
      if (!silent) {
        showError(t('邀请码状态获取失败'));
      }
    } finally {
      if (!silent) {
        setRegistrationInviteLoading(false);
      }
    }
  };

  const generateRegistrationInviteCode = async () => {
    setRegistrationInviteLoading(true);
    try {
      const res = await API.post('/api/user/registration-invite-code');
      const { success, message, data } = res.data;
      if (success) {
        setRegistrationInviteInfo(data || emptyRegistrationInviteInfo);
        showSuccess(t('邀请码已生成'));
      } else {
        if (data) {
          setRegistrationInviteInfo({
            ...emptyRegistrationInviteInfo,
            ...data,
          });
        }
        showError(message);
      }
    } catch (error) {
      showError(t('邀请码生成失败，请重试'));
    } finally {
      setRegistrationInviteLoading(false);
    }
  };

  // 划转邀请额度
  const transfer = async () => {
    if (transferAmount < getQuotaPerUnit()) {
      showError(t('划转金额最低为') + ' ' + renderQuota(getQuotaPerUnit()));
      return;
    }
    const res = await API.post(`/api/user/aff_transfer`, {
      quota: transferAmount,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      getUserQuota().then();
    } else {
      showError(message);
    }
  };

  // 复制邀请链接
  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  const oneTimeInviteEnabled =
    !!statusState?.status?.password_register_one_time_invite_code_enabled;
  const currentUserRole = userState?.user?.role ?? 0;
  const showInviteActions = oneTimeInviteEnabled
    ? currentUserRole > 0
    : currentUserRole === 1;

  const handleRegistrationInviteCodeCopy = async () => {
    if (!registrationInviteInfo.code) {
      return;
    }
    await copy(registrationInviteInfo.code);
    showSuccess(t('邀请码已复制到剪切板'));
  };

  useEffect(() => {
    // 始终获取最新用户数据，确保余额等统计信息准确
    getUserQuota().then();
    setTransferAmount(getQuotaPerUnit());
  }, []);

  useEffect(() => {
    const status = statusState?.status;
    const role = userState?.user?.role;
    if (!status || role === undefined) {
      return;
    }

    const oneTimeInviteEnabled =
      !!status.password_register_one_time_invite_code_enabled;

    if (oneTimeInviteEnabled) {
      setAffLink('');
      if (role > 0) {
        const refreshRegistrationInviteInfo = () => {
          getRegistrationInviteCode({ silent: true }).then();
        };
        refreshRegistrationInviteInfo();

        const intervalId = window.setInterval(
          refreshRegistrationInviteInfo,
          registrationInviteRefreshIntervalMs,
        );
        const handleWindowFocus = () => {
          refreshRegistrationInviteInfo();
        };
        const handleVisibilityChange = () => {
          if (document.visibilityState === 'visible') {
            refreshRegistrationInviteInfo();
          }
        };

        window.addEventListener('focus', handleWindowFocus);
        document.addEventListener('visibilitychange', handleVisibilityChange);

        return () => {
          window.clearInterval(intervalId);
          window.removeEventListener('focus', handleWindowFocus);
          document.removeEventListener(
            'visibilitychange',
            handleVisibilityChange,
          );
        };
      } else {
        setRegistrationInviteInfo(emptyRegistrationInviteInfo);
      }
      return;
    }

    setRegistrationInviteInfo(emptyRegistrationInviteInfo);
    if (role === 1) {
      getAffLink().then();
    } else {
      setAffLink('');
    }
  }, [statusState?.status, userState?.user?.role]);

  // 在 statusState 可用时获取充值信息
  useEffect(() => {
    getTopupInfo().then();
    getSubscriptionPlans().then();
    getSubscriptionSelf().then();
  }, []);

  useEffect(() => {
    if (statusState?.status) {
      // const minTopUpValue = statusState.status.min_topup || 1;
      // setMinTopUp(minTopUpValue);
      // setTopUpCount(minTopUpValue);
      setTopUpLink(statusState.status.top_up_link || '');
      setPriceRatio(statusState.status.price || 1);

      setStatusLoading(false);
    }
  }, [statusState?.status]);

  const renderAmount = () => {
    return amount + ' ' + t('元');
  };

  const getAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
    }
    setAmountLoading(false);
  };

  const getStripeAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/stripe/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: '错误：' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
    } finally {
      setAmountLoading(false);
    }
  };

  const handleCancel = () => {
    setOpen(false);
  };

  const handleTransferCancel = () => {
    setOpenTransfer(false);
  };

  const handleOpenHistory = () => {
    setOpenHistory(true);
  };

  const handleHistoryCancel = () => {
    setOpenHistory(false);
  };

  const handleCreemCancel = () => {
    setCreemOpen(false);
    setSelectedCreemProduct(null);
  };

  // 选择预设充值额度
  const selectPresetAmount = (preset) => {
    setTopUpCount(preset.value);
    setSelectedPreset(preset.value);

    // 计算实际支付金额，考虑折扣
    const discount = preset.discount || topupInfo.discount[preset.value] || 1.0;
    const discountedAmount = preset.value * priceRatio * discount;
    setAmount(discountedAmount);
  };

  // 格式化大数字显示
  const formatLargeNumber = (num) => {
    return num.toString();
  };

  // 根据最小充值金额生成预设充值额度选项
  const generatePresetAmounts = (minAmount) => {
    const multipliers = [1, 5, 10, 30, 50, 100, 300, 500];
    return multipliers.map((multiplier) => ({
      value: minAmount * multiplier,
    }));
  };

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2'>
      {/* 划转模态框 */}
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={handleTransferCancel}
        userState={userState}
        renderQuota={renderQuota}
        getQuotaPerUnit={getQuotaPerUnit}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
      />

      {/* 充值确认模态框 */}
      <PaymentConfirmModal
        t={t}
        open={open}
        onlineTopUp={onlineTopUp}
        handleCancel={handleCancel}
        confirmLoading={confirmLoading}
        topUpCount={topUpCount}
        renderQuotaWithAmount={renderQuotaWithAmount}
        amountLoading={amountLoading}
        renderAmount={renderAmount}
        payWay={payWay}
        payMethods={payMethods}
        amountNumber={amount}
        discountRate={topupInfo?.discount?.[topUpCount] || 1.0}
      />

      {/* 充值账单模态框 */}
      <TopupHistoryModal
        visible={openHistory}
        onCancel={handleHistoryCancel}
        t={t}
      />

      {/* Creem 充值确认模态框 */}
      <Modal
        title={t('确定要充值 $')}
        visible={creemOpen}
        onOk={onlineCreemTopUp}
        onCancel={handleCreemCancel}
        maskClosable={false}
        size='small'
        centered
        confirmLoading={confirmLoading}
      >
        {selectedCreemProduct && (
          <>
            <p>
              {t('产品名称')}：{selectedCreemProduct.name}
            </p>
            <p>
              {t('价格')}：{selectedCreemProduct.currency === 'EUR' ? '€' : '$'}
              {selectedCreemProduct.price}
            </p>
            <p>
              {t('充值额度')}：{selectedCreemProduct.quota}
            </p>
            <p>{t('是否确认充值？')}</p>
          </>
        )}
      </Modal>

      {/* 主布局区域 */}
      <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
        <RechargeCard
          t={t}
          enableOnlineTopUp={enableOnlineTopUp}
          enableStripeTopUp={enableStripeTopUp}
          enableCreemTopUp={enableCreemTopUp}
          creemProducts={creemProducts}
          creemPreTopUp={creemPreTopUp}
          presetAmounts={presetAmounts}
          selectedPreset={selectedPreset}
          selectPresetAmount={selectPresetAmount}
          formatLargeNumber={formatLargeNumber}
          priceRatio={priceRatio}
          topUpCount={topUpCount}
          minTopUp={minTopUp}
          renderQuotaWithAmount={renderQuotaWithAmount}
          getAmount={getAmount}
          setTopUpCount={setTopUpCount}
          setSelectedPreset={setSelectedPreset}
          renderAmount={renderAmount}
          amountLoading={amountLoading}
          payMethods={payMethods}
          preTopUp={preTopUp}
          paymentLoading={paymentLoading}
          payWay={payWay}
          redemptionCode={redemptionCode}
          setRedemptionCode={setRedemptionCode}
          topUp={topUp}
          isSubmitting={isSubmitting}
          topUpLink={topUpLink}
          showTopupStore={showTopupStore}
          openTopUpLink={openTopUpLink}
          userState={userState}
          renderQuota={renderQuota}
          statusLoading={statusLoading}
          topupInfo={topupInfo}
          onOpenHistory={handleOpenHistory}
          subscriptionLoading={subscriptionLoading}
          subscriptionPlans={subscriptionPlans}
          billingPreference={billingPreference}
          onChangeBillingPreference={updateBillingPreference}
          activeSubscriptions={activeSubscriptions}
          allSubscriptions={allSubscriptions}
          reloadSubscriptionSelf={getSubscriptionSelf}
        />
        {showInvitationCard && (
          <InvitationCard
            t={t}
            userState={userState}
            renderQuota={renderQuota}
            setOpenTransfer={setOpenTransfer}
            inviteMode={
              statusState?.status?.password_register_one_time_invite_code_enabled
                ? 'one_time'
                : 'legacy_aff'
            }
            showInviteActions={showInviteActions}
            affLink={affLink}
            handleAffLinkClick={handleAffLinkClick}
            registrationInviteInfo={registrationInviteInfo}
            registrationInviteLoading={registrationInviteLoading}
            onGenerateRegistrationInviteCode={generateRegistrationInviteCode}
            onCopyRegistrationInviteCode={handleRegistrationInviteCodeCopy}
          />
        )}
      </div>
    </div>
  );
};

export default TopUp;
