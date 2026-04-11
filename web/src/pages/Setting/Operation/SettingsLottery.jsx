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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const DEFAULT_TIERS = [
  { name: '普通', amount: 3, probability: 82 },
  { name: '稀有', amount: 8, probability: 13 },
  { name: '史诗', amount: 20, probability: 4 },
  { name: '传说', amount: 50, probability: 0.8 },
  { name: '神话', amount: 200, probability: 0.2 },
];

function normalizeTiers(rawTiers) {
  if (!Array.isArray(rawTiers) || rawTiers.length !== DEFAULT_TIERS.length) {
    return DEFAULT_TIERS;
  }
  return DEFAULT_TIERS.map((tier, index) => {
    const raw = rawTiers[index] || {};
    const amount = Number(raw.amount);
    const probability = Number(raw.probability);
    return {
      name: tier.name,
      amount: Number.isFinite(amount) && amount > 0 ? amount : tier.amount,
      probability:
        Number.isFinite(probability) && probability >= 0
          ? probability
          : tier.probability,
    };
  });
}

function createInputsFromTiers(tiers) {
  const values = {};
  tiers.forEach((tier, index) => {
    values[`lottery_setting.tier_${index}_amount`] = tier.amount;
    values[`lottery_setting.tier_${index}_probability`] = tier.probability;
  });
  return values;
}

function buildTiersFromInputs(inputs) {
  return DEFAULT_TIERS.map((tier, index) => ({
    name: tier.name,
    amount: Number(inputs[`lottery_setting.tier_${index}_amount`] || 0),
    probability: Number(
      inputs[`lottery_setting.tier_${index}_probability`] || 0,
    ),
  }));
}

export default function SettingsLottery(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const refForm = useRef();
  const [inputs, setInputs] = useState({
    'lottery_setting.weekly_day': 0,
    'lottery_setting.myth_broadcast_enabled': true,
    ...createInputsFromTiers(DEFAULT_TIERS),
  });
  const [inputsRow, setInputsRow] = useState(inputs);
  const [effectiveWeeklyDay, setEffectiveWeeklyDay] = useState(0);

  const weeklyDayOptions = useMemo(
    () => [
      { label: t('关闭自动开场'), value: 0 },
      { label: t('周一'), value: 1 },
      { label: t('周二'), value: 2 },
      { label: t('周三'), value: 3 },
      { label: t('周四'), value: 4 },
      { label: t('周五'), value: 5 },
      { label: t('周六'), value: 6 },
      { label: t('周日'), value: 7 },
      { label: t('周一到周五随机一天'), value: 8 },
    ],
    [t],
  );

  const effectiveWeeklyDayLabel = useMemo(() => {
    if (Number(effectiveWeeklyDay) <= 0) {
      return t('未定');
    }
    const matched = weeklyDayOptions.find(
      (option) => Number(option.value) === Number(effectiveWeeklyDay),
    );
    return matched?.label || t('未定');
  }, [effectiveWeeklyDay, t, weeklyDayOptions]);

  const totalProbability = useMemo(
    () =>
      buildTiersFromInputs(inputs).reduce(
        (sum, tier) => sum + Number(tier.probability || 0),
        0,
      ),
    [inputs],
  );

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((current) => ({ ...current, [fieldName]: value }));
    };
  }

  async function onSubmit() {
    const nextTiers = buildTiersFromInputs(inputs);
    const previousTiers = buildTiersFromInputs(inputsRow);

    if (
      nextTiers.some(
        (tier) => !Number.isFinite(tier.amount) || Number(tier.amount) <= 0,
      )
    ) {
      return showWarning(t('五档金额都必须大于 0'));
    }
    if (
      nextTiers.some(
        (tier) =>
          !Number.isFinite(tier.probability) || Number(tier.probability) < 0,
      )
    ) {
      return showWarning(t('五档概率都必须大于或等于 0'));
    }
    if (Math.abs(totalProbability - 100) > 0.0001) {
      return showWarning(t('五档概率合计必须为 100%'));
    }

    const changes = [];
    if (
      Number(inputs['lottery_setting.weekly_day']) !==
      Number(inputsRow['lottery_setting.weekly_day'])
    ) {
      changes.push({
        label: t('每周自动开场日'),
        key: 'lottery_setting.weekly_day',
        value: String(Number(inputs['lottery_setting.weekly_day']) || 0),
      });
    }
    if (
      Boolean(inputs['lottery_setting.myth_broadcast_enabled']) !==
      Boolean(inputsRow['lottery_setting.myth_broadcast_enabled'])
    ) {
      changes.push({
        label: t('神话档位广播'),
        key: 'lottery_setting.myth_broadcast_enabled',
        value: String(
          Boolean(inputs['lottery_setting.myth_broadcast_enabled']),
        ),
      });
    }
    if (JSON.stringify(nextTiers) !== JSON.stringify(previousTiers)) {
      changes.push({
        label: t('五档奖池'),
        key: 'lottery_setting.tiers',
        value: JSON.stringify(nextTiers),
      });
    }

    if (!changes.length) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      for (const change of changes) {
        const res = await API.put('/api/option/', {
          key: change.key,
          value: change.value,
        });
        if (!res.data?.success) {
          throw new Error(
            res.data?.message || `${change.label}${t('保存失败，请重试')}`,
          );
        }
      }
      showSuccess(t('保存成功；若当前已有进行中的活动，新奖池会从下一场生效'));
      await props.refresh();
    } catch (error) {
      showError(error?.message || t('保存失败，当前值已刷新，请确认后重试'));
      await props.refresh();
    } finally {
      setLoading(false);
    }
  }

  async function loadLotteryAdminState() {
    try {
      const res = await API.get('/api/lottery/admin/active');
      if (res.data?.success) {
        setEffectiveWeeklyDay(Number(res.data.data?.effective_weekly_day) || 0);
      }
    } catch (error) {
      setEffectiveWeeklyDay(0);
    }
  }

  useEffect(() => {
    let parsedTiers = DEFAULT_TIERS;
    const rawTiers = props.options?.['lottery_setting.tiers'];
    if (typeof rawTiers === 'string' && rawTiers) {
      try {
        parsedTiers = normalizeTiers(JSON.parse(rawTiers));
      } catch (error) {
        parsedTiers = DEFAULT_TIERS;
      }
    }

    const currentInputs = {
      'lottery_setting.weekly_day': Number(
        props.options?.['lottery_setting.weekly_day'] || 0,
      ),
      'lottery_setting.myth_broadcast_enabled':
        props.options?.['lottery_setting.myth_broadcast_enabled'] ?? true,
      ...createInputsFromTiers(parsedTiers),
    };

    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current?.setValues(currentInputs);
  }, [props.options]);

  useEffect(() => {
    loadLotteryAdminState();
  }, [props.options]);

  return (
    <Spin spinning={loading}>
      <Form
        values={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('大乐透设置')}>
          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 16, display: 'block' }}
          >
            {t(
              '公开场按设定周几懒触发开启；也支持每周在周一到周五之间随机一天开场，且同一周内结果固定。管理员也可随时在活动页立即开启公开场或管理员测试场。已开启活动会沿用开启时的奖池快照，新的金额和概率从下一场生效。保存时会按顺序提交并在失败后自动回刷当前值。',
            )}
          </Typography.Text>

          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Select
                field={'lottery_setting.weekly_day'}
                label={t('每周自动开场日')}
                optionList={weeklyDayOptions}
                onChange={handleFieldChange('lottery_setting.weekly_day')}
              />
              {Number(inputs['lottery_setting.weekly_day']) === 8 ? (
                <Typography.Text
                  type='tertiary'
                  size='small'
                  style={{ display: 'block', marginTop: 8 }}
                >
                  {t('本周实际随机开场日')}：{effectiveWeeklyDayLabel}
                </Typography.Text>
              ) : null}
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field={'lottery_setting.myth_broadcast_enabled'}
                label={t('神话档位广播')}
                extraText={t('控制神话开出时是否允许后续展示广播文案')}
                size='default'
                checkedText='｜'
                uncheckedText='〇'
                onChange={handleFieldChange(
                  'lottery_setting.myth_broadcast_enabled',
                )}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <div
                style={{
                  paddingTop: 30,
                  fontSize: 14,
                  color:
                    Math.abs(totalProbability - 100) <= 0.0001
                      ? 'var(--semi-color-text-1)'
                      : 'var(--semi-color-danger)',
                }}
              >
                {t('概率合计')} {totalProbability}%
              </div>
            </Col>
          </Row>

          <div style={{ marginTop: 8, marginBottom: 12, fontWeight: 600 }}>
            {t('五档奖池')}
          </div>
          <Row gutter={[16, 16]}>
            {DEFAULT_TIERS.map((tier, index) => (
              <Col key={tier.name} xs={24} sm={12} md={12} lg={12} xl={12}>
                <div
                  style={{
                    padding: 16,
                    borderRadius: 10,
                    border: '1px solid var(--semi-color-border)',
                    background: 'var(--semi-color-bg-1)',
                  }}
                >
                  <div
                    style={{
                      marginBottom: 12,
                      fontSize: 15,
                      fontWeight: 600,
                    }}
                  >
                    {tier.name}
                  </div>
                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.InputNumber
                        field={`lottery_setting.tier_${index}_amount`}
                        label={t('金额')}
                        min={1}
                        step={1}
                        precision={0}
                        onChange={handleFieldChange(
                          `lottery_setting.tier_${index}_amount`,
                        )}
                      />
                    </Col>
                    <Col span={12}>
                      <Form.InputNumber
                        field={`lottery_setting.tier_${index}_probability`}
                        label={t('概率 %')}
                        min={0}
                        step={0.1}
                        precision={3}
                        onChange={handleFieldChange(
                          `lottery_setting.tier_${index}_probability`,
                        )}
                      />
                    </Col>
                  </Row>
                </div>
              </Col>
            ))}
          </Row>

          <Row style={{ marginTop: 16 }}>
            <Button size='default' onClick={onSubmit}>
              {t('保存大乐透设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
