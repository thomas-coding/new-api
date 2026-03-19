import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import CardPro from '../../components/common/ui/CardPro';
import {
  API,
  copy,
  downloadTextAsFile,
  isRoot,
  showError,
  showSuccess,
} from '../../helpers';
import { displayAmountToQuota } from '../../helpers/quota';
import {
  Banner,
  Button,
  Card,
  DatePicker,
  Divider,
  Input,
  InputNumber,
  Modal,
  Radio,
  RadioGroup,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { REDEMPTION_TYPES } from '../../constants/redemption.constants';

const { Title, Text } = Typography;

const defaultUsersForm = {
  count: 10,
  username_prefix: 'demo',
  start_number: 1,
  number_width: 4,
  initial_quota_display: 10,
  group: 'default',
  status: 1,
  password_mode: 'random',
  fixed_password: '',
  random_password_length: 12,
  export_format: 'txt',
  file_name: 'users-batch',
};

const defaultRedemptionsForm = {
  redeem_type: REDEMPTION_TYPES.SUBSCRIPTION,
  plan_id: 0,
  quota_display: 10,
  count: 20,
  code_length: 12,
  prefix: 'WEEK-',
  expired_time: null,
  status: 1,
  name: '',
  export_format: 'txt',
  file_name: 'redeem-codes',
};

const defaultQuotaForm = {
  scope_type: 'all',
  group: 'default',
  include_admins: false,
  quota_delta_display: 10,
  reason: '',
};

const csvHeaders = {
  users: 'username,password,token,group,initial_quota,status\n',
  redemptions: 'code,redeem_type,target,expires_at,batch_id\n',
  quota: 'username,result,reason\n',
};

function BatchTools() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('redemptions');
  const [groups, setGroups] = useState([]);
  const [plans, setPlans] = useState([]);
  const [jobs, setJobs] = useState([]);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [jobDetail, setJobDetail] = useState(null);
  const [jobDetailVisible, setJobDetailVisible] = useState(false);
  const [jobDetailLoading, setJobDetailLoading] = useState(false);

  const [usersForm, setUsersForm] = useState(defaultUsersForm);
  const [usersPreview, setUsersPreview] = useState(null);
  const [usersResult, setUsersResult] = useState(null);
  const [usersLoading, setUsersLoading] = useState(false);

  const [redemptionsForm, setRedemptionsForm] = useState(defaultRedemptionsForm);
  const [redemptionsPreview, setRedemptionsPreview] = useState(null);
  const [redemptionsResult, setRedemptionsResult] = useState(null);
  const [redemptionsLoading, setRedemptionsLoading] = useState(false);

  const [quotaForm, setQuotaForm] = useState(defaultQuotaForm);
  const [quotaPreview, setQuotaPreview] = useState(null);
  const [quotaResult, setQuotaResult] = useState(null);
  const [quotaLoading, setQuotaLoading] = useState(false);

  const groupOptions = useMemo(
    () => groups.map((item) => ({ label: item, value: item })),
    [groups],
  );
  const planOptions = useMemo(
    () =>
      plans.map((item) => ({
        label: `${item.plan?.title || t('未命名套餐')} (#${item.plan?.id || 0})`,
        value: item.plan?.id,
      })),
    [plans, t],
  );
  const statusOptions = useMemo(
    () => [
      { label: t('启用'), value: 1 },
      { label: t('禁用'), value: 2 },
    ],
    [t],
  );
  const redemptionTypeOptions = useMemo(
    () => [
      { label: '订阅套餐', value: REDEMPTION_TYPES.SUBSCRIPTION },
      { label: '额度', value: REDEMPTION_TYPES.QUOTA },
    ],
    [],
  );

  useEffect(() => {
    if (!isRoot()) return;
    const loadInitialData = async () => {
      setJobsLoading(true);
      try {
        const [groupRes, planRes, jobsRes] = await Promise.all([
          API.get('/api/group/'),
          API.get('/api/subscription/admin/plans'),
          API.get('/api/admin/batch/jobs'),
        ]);
        const groupList = groupRes.data?.success && Array.isArray(groupRes.data.data)
          ? groupRes.data.data
          : [];
        const planList = planRes.data?.success && Array.isArray(planRes.data.data)
          ? planRes.data.data
          : [];
        const jobList = jobsRes.data?.success
          ? jobsRes.data.data?.items || []
          : [];
        setGroups(groupList);
        setPlans(planList);
        setJobs(jobList);
        if (groupList[0]) {
          setUsersForm((prev) => ({ ...prev, group: prev.group || groupList[0] }));
          setQuotaForm((prev) => ({ ...prev, group: prev.group || groupList[0] }));
        }
        if (planList[0]?.plan?.id) {
          setRedemptionsForm((prev) => ({
            ...prev,
            plan_id: prev.plan_id || planList[0].plan.id,
            name: prev.name || planList[0].plan.title || '',
          }));
        }
      } catch {
        showError(t('批量工具初始化加载失败'));
      } finally {
        setJobsLoading(false);
      }
    };
    void loadInitialData();
  }, [t]);

  const loadGroups = async () => {
    try {
      const res = await API.get('/api/group/');
      if (res.data.success) {
        const list = Array.isArray(res.data.data) ? res.data.data : [];
        setGroups(list);
        setUsersForm((prev) => ({ ...prev, group: prev.group || list[0] || 'default' }));
        setQuotaForm((prev) => ({ ...prev, group: prev.group || list[0] || 'default' }));
      }
    } catch {
      showError(t('加载分组失败'));
    }
  };

  const loadPlans = async () => {
    try {
      const res = await API.get('/api/subscription/admin/plans');
      if (res.data.success) {
        const list = Array.isArray(res.data.data) ? res.data.data : [];
        setPlans(list);
        if (list[0]?.plan?.id) {
          setRedemptionsForm((prev) => ({
            ...prev,
            plan_id: prev.plan_id || list[0].plan.id,
            name: prev.name || list[0].plan.title || '',
          }));
        }
      }
    } catch {
      showError(t('加载套餐失败'));
    }
  };

  const loadJobs = async () => {
    setJobsLoading(true);
    try {
      const res = await API.get('/api/admin/batch/jobs');
      if (res.data.success) {
        setJobs(res.data.data?.items || []);
      }
    } catch {
      showError(t('加载最近批次失败'));
    } finally {
      setJobsLoading(false);
    }
  };

  const openJobDetail = async (batchId) => {
    setJobDetailVisible(true);
    setJobDetailLoading(true);
    try {
      const res = await API.get(`/api/admin/batch/jobs/${batchId}`);
      if (res.data.success) {
        setJobDetail(res.data.data);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('加载批次详情失败'));
    } finally {
      setJobDetailLoading(false);
    }
  };

  const updateUsersForm = (patch) => {
    setUsersForm((prev) => ({ ...prev, ...patch }));
    setUsersPreview(null);
    setUsersResult(null);
  };
  const updateRedemptionsForm = (patch) => {
    setRedemptionsForm((prev) => ({ ...prev, ...patch }));
    setRedemptionsPreview(null);
    setRedemptionsResult(null);
  };
  const updateQuotaForm = (patch) => {
    setQuotaForm((prev) => ({ ...prev, ...patch }));
    setQuotaPreview(null);
    setQuotaResult(null);
  };

  const usersPayload = () => ({
    ...usersForm,
    initial_quota: displayAmountToQuota(usersForm.initial_quota_display),
  });
  const redemptionsPayload = () => ({
    ...redemptionsForm,
    plan_id:
      redemptionsForm.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION
        ? Number(redemptionsForm.plan_id || 0)
        : 0,
    quota:
      redemptionsForm.redeem_type === REDEMPTION_TYPES.QUOTA
        ? displayAmountToQuota(Number(redemptionsForm.quota_display || 0))
        : 0,
    expired_time: redemptionsForm.expired_time
      ? Math.floor(new Date(redemptionsForm.expired_time).getTime() / 1000)
      : 0,
  });
  const quotaPayload = () => ({
    ...quotaForm,
    quota_delta: displayAmountToQuota(quotaForm.quota_delta_display),
  });

  const runPreview = async (url, payload, setLoading, setPreview, message) => {
    setLoading(true);
    try {
      const res = await API.post(url, payload);
      if (res.data.success) {
        setPreview(res.data.data);
        showSuccess(message);
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(message);
    } finally {
      setLoading(false);
    }
  };

  const runExecute = (title, content, setLoading, executor) => {
    Modal.confirm({
      title,
      content,
      onOk: async () => {
        setLoading(true);
        try {
          await executor();
        } finally {
          setLoading(false);
        }
      },
    });
  };

  const previewUsers = async () => {
    await runPreview(
      '/api/admin/batch/users/preview',
      usersPayload(),
      setUsersLoading,
      setUsersPreview,
      t('批量用户预览已生成'),
    );
  };

  const executeUsers = () => {
    if (!usersPreview?.preview_token) {
      showError(t('请先预览再执行'));
      return;
    }
    runExecute(
      t('确认批量创建用户'),
      t('明文密码仅在当前执行结果中显示一次，请执行后立即下载保存。'),
      setUsersLoading,
      async () => {
        const res = await API.post('/api/admin/batch/users/execute', {
          ...usersPayload(),
          preview_token: usersPreview.preview_token,
        });
        if (res.data.success) {
          setUsersResult(res.data.data);
          showSuccess(t('批量创建完成'));
          await loadJobs();
          return;
        }
        showError(res.data.message);
      },
    );
  };

  const previewRedemptions = async () => {
    await runPreview(
      '/api/admin/batch/redemptions/preview',
      redemptionsPayload(),
      setRedemptionsLoading,
      setRedemptionsPreview,
      t('批量兑换码预览已生成'),
    );
  };

  const executeRedemptions = () => {
    if (!redemptionsPreview?.preview_token) {
      showError(t('请先预览再执行'));
      return;
    }
    runExecute(
      t('确认批量生成兑换码'),
      t('完整兑换码仅会在当前结果中返回一次。'),
      setRedemptionsLoading,
      async () => {
        const res = await API.post('/api/admin/batch/redemptions/execute', {
          ...redemptionsPayload(),
          preview_token: redemptionsPreview.preview_token,
        });
        if (res.data.success) {
          setRedemptionsResult(res.data.data);
          showSuccess(t('批量兑换码生成完成'));
          await loadJobs();
          return;
        }
        showError(res.data.message);
      },
    );
  };

  const previewQuota = async () => {
    await runPreview(
      '/api/admin/batch/quota/preview',
      quotaPayload(),
      setQuotaLoading,
      setQuotaPreview,
      t('批量增额预览已生成'),
    );
  };

  const executeQuota = () => {
    if (!quotaPreview?.preview_token) {
      showError(t('请先预览再执行'));
      return;
    }
    runExecute(
      t('确认批量增额'),
      t('该操作会立即生效，且不提供一键回滚。'),
      setQuotaLoading,
      async () => {
        const res = await API.post('/api/admin/batch/quota/execute', {
          ...quotaPayload(),
          preview_token: quotaPreview.preview_token,
        });
        if (res.data.success) {
          setQuotaResult(res.data.data);
          showSuccess(t('批量增额执行完成'));
          await loadJobs();
          return;
        }
        showError(res.data.message);
      },
    );
  };

  const handleDownload = (result, kind) => {
    if (!result?.export?.available) {
      showError(t('当前没有可下载内容'));
      return;
    }
    let content = result.export.content || '';
    if (result.export.format === 'csv' && content && !content.startsWith(csvHeaders[kind])) {
      content = `${csvHeaders[kind]}${content}`;
    }
    downloadTextAsFile(content, result.export.filename);
  };

  const handleCopy = async (result) => {
    if (!result?.export?.content) {
      showError(t('没有可复制内容'));
      return;
    }
    const ok = await copy(result.export.content);
    if (ok) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const renderWarnings = (preview) => {
    if (!preview?.warnings?.length) return null;
    return preview.warnings.map((warning, idx) => (
      <Banner
        key={`${preview.batch_type || 'preview'}-${warning}-${preview.preview_expires_at || idx}`}
        type={preview.risk_level === 'high' ? 'warning' : 'info'}
        description={warning}
        className='mb-2'
      />
    ));
  };

  const renderResultPanel = (preview, result, kind) => (
    <div className='space-y-4'>
      <Card>
        <Title heading={6}>{t('预览 / 风险')}</Title>
        {preview ? (
          <>
            {renderWarnings(preview)}
            <pre className='text-xs whitespace-pre-wrap break-all bg-[var(--semi-color-fill-0)] p-3 rounded-lg overflow-auto'>
              {JSON.stringify(preview.summary, null, 2)}
            </pre>
            {Array.isArray(preview.samples) && preview.samples.length > 0 && (
              <div className='mt-3 flex flex-wrap gap-2'>
                {preview.samples.map((item) => (
                  <Tag key={item} color='blue' shape='circle'>
                    {item}
                  </Tag>
                ))}
              </div>
            )}
          </>
        ) : (
          <Text type='tertiary'>{t('填写参数后点击预览，这里会展示摘要和风险提示。')}</Text>
        )}
      </Card>
      <Card>
        <Title heading={6}>{t('执行结果')}</Title>
        {result ? (
          <>
            <div className='mb-3 flex flex-wrap gap-2'>
              <Tag color='green' shape='circle'>
                {t('批次号')}: {result.batch_id}
              </Tag>
              <Tag color={result.status === 'completed' ? 'green' : 'orange'} shape='circle'>
                {result.status}
              </Tag>
            </div>
            <pre className='text-xs whitespace-pre-wrap break-all bg-[var(--semi-color-fill-0)] p-3 rounded-lg overflow-auto'>
              {JSON.stringify(result.summary, null, 2)}
            </pre>
            {result.failed_items?.length > 0 && (
              <pre className='text-xs whitespace-pre-wrap break-all bg-[var(--semi-color-fill-0)] p-3 rounded-lg overflow-auto mt-3'>
                {JSON.stringify(result.failed_items, null, 2)}
              </pre>
            )}
            <div className='mt-3 flex flex-wrap gap-2'>
              <Button theme='solid' disabled={!result.export?.available} onClick={() => handleDownload(result, kind)}>
                {t('下载结果')}
              </Button>
              <Button theme='light' disabled={!result.export?.available} onClick={() => handleCopy(result)}>
                {t('复制内容')}
              </Button>
            </div>
          </>
        ) : (
          <Text type='tertiary'>{t('执行后这里会展示结果摘要和失败清单。')}</Text>
        )}
      </Card>
    </div>
  );

  const jobsColumns = [
    { title: t('批次号'), dataIndex: 'batch_id' },
    { title: t('类型'), dataIndex: 'batch_type' },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={value === 'completed' ? 'green' : 'orange'} shape='circle'>
          {value}
        </Tag>
      ),
    },
    { title: t('操作者'), dataIndex: 'operator_username' },
    {
      title: t('成功/失败'),
      render: (_, record) => `${record.success_count}/${record.failed_count}`,
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: (value) => (value ? new Date(value * 1000).toLocaleString() : '-'),
    },
    {
      title: t('操作'),
      render: (_, record) => (
        <Button theme='light' size='small' onClick={() => openJobDetail(record.batch_id)}>
          {t('查看摘要')}
        </Button>
      ),
    },
  ];

  const pageDescription = (
    <div>
      <Title heading={4} className='!mb-1'>
        {t('批量工具')}
      </Title>
      <Text type='secondary'>
        {t('统一处理批量新建用户、批量生成兑换码、批量增额；敏感导出内容仅当前结果可见。')}
      </Text>
      <div className='mt-3'>
        <Banner
          type='warning'
          description={t('所有批量操作都必须先预览，再执行；敏感导出内容只会在执行成功后的当前结果中返回一次。')}
        />
      </div>
    </div>
  );

  if (!isRoot()) {
    return (
      <div className='mt-[60px] px-2'>
        <Banner type='danger' description={t('批量工具仅对 root 开放。')} />
      </div>
    );
  }

  return (
    <div className='mt-[60px] px-2 space-y-4'>
      <CardPro type='type1' descriptionArea={pageDescription}>
        <Tabs type='card' activeKey={activeTab} onChange={setActiveTab}>
          <Tabs.TabPane tab={t('批量生成兑换码')} itemKey='redemptions'>
            <div className='grid grid-cols-1 xl:grid-cols-3 gap-4'>
              <div className='xl:col-span-2'>
                <Card>
                  <Title heading={6}>{t('参数配置')}</Title>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                    <div>
                      <Text>{t('兑换类型')}</Text>
                      <Select
                        className='mt-1'
                        optionList={redemptionTypeOptions}
                        value={redemptionsForm.redeem_type}
                        onChange={(value) =>
                          updateRedemptionsForm({
                            redeem_type: value,
                            plan_id:
                              value === REDEMPTION_TYPES.SUBSCRIPTION
                                ? redemptionsForm.plan_id || planOptions[0]?.value || 0
                                : redemptionsForm.plan_id,
                          })
                        }
                      />
                    </div>
                    {redemptionsForm.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION ? (
                      <div>
                        <Text>{t('目标套餐')}</Text>
                        <Select
                          className='mt-1'
                          optionList={planOptions}
                          value={redemptionsForm.plan_id}
                          onChange={(value) => updateRedemptionsForm({ plan_id: value })}
                        />
                      </div>
                    ) : (
                      <div>
                        <Text>{t('额度（显示单位）')}</Text>
                        <InputNumber
                          className='mt-1 w-full'
                          min={0}
                          value={redemptionsForm.quota_display}
                          onChange={(value) =>
                            updateRedemptionsForm({
                              quota_display: Number(value || 0),
                            })
                          }
                        />
                      </div>
                    )}
                    <div>
                      <Text>{t('数量')}</Text>
                      <InputNumber className='mt-1 w-full' min={1} max={500} value={redemptionsForm.count} onChange={(value) => updateRedemptionsForm({ count: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('随机长度')}</Text>
                      <InputNumber className='mt-1 w-full' min={6} max={24} value={redemptionsForm.code_length} onChange={(value) => updateRedemptionsForm({ code_length: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('前缀')}</Text>
                      <Input className='mt-1' value={redemptionsForm.prefix} onChange={(value) => updateRedemptionsForm({ prefix: value })} />
                    </div>
                    <div>
                      <Text>{t('名称')}</Text>
                      <Input className='mt-1' value={redemptionsForm.name} onChange={(value) => updateRedemptionsForm({ name: value })} />
                    </div>
                    <div>
                      <Text>{t('状态')}</Text>
                      <Select className='mt-1' optionList={statusOptions} value={redemptionsForm.status} onChange={(value) => updateRedemptionsForm({ status: value })} />
                    </div>
                    <div>
                      <Text>{t('导出格式')}</Text>
                      <Select className='mt-1' optionList={[{ label: 'TXT', value: 'txt' }, { label: 'CSV', value: 'csv' }]} value={redemptionsForm.export_format} onChange={(value) => updateRedemptionsForm({ export_format: value })} />
                    </div>
                    <div>
                      <Text>{t('文件名')}</Text>
                      <Input className='mt-1' value={redemptionsForm.file_name} onChange={(value) => updateRedemptionsForm({ file_name: value })} />
                    </div>
                    <div className='md:col-span-2'>
                      <Text>{t('到期时间')}</Text>
                      <DatePicker className='mt-1 w-full' type='dateTime' value={redemptionsForm.expired_time} onChange={(value) => updateRedemptionsForm({ expired_time: value || null })} />
                      <Text type='tertiary' size='small' className='mt-1 block'>
                        {t('留空表示长期有效')}
                      </Text>
                    </div>
                  </div>
                  <Divider />
                  <Space>
                    <Button theme='solid' loading={redemptionsLoading} onClick={previewRedemptions}>{t('预览')}</Button>
                    <Button theme='light' disabled={!redemptionsPreview} loading={redemptionsLoading} onClick={executeRedemptions}>{t('执行')}</Button>
                  </Space>
                </Card>
              </div>
              {renderResultPanel(redemptionsPreview, redemptionsResult, 'redemptions')}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane tab={t('批量新建用户')} itemKey='users'>
            <div className='grid grid-cols-1 xl:grid-cols-3 gap-4'>
              <div className='xl:col-span-2'>
                <Card>
                  <Title heading={6}>{t('参数配置')}</Title>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                    <div>
                      <Text>{t('数量')}</Text>
                      <InputNumber className='mt-1 w-full' min={1} max={200} value={usersForm.count} onChange={(value) => updateUsersForm({ count: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('用户名前缀')}</Text>
                      <Input className='mt-1' value={usersForm.username_prefix} onChange={(value) => updateUsersForm({ username_prefix: value })} />
                    </div>
                    <div>
                      <Text>{t('起始序号')}</Text>
                      <InputNumber className='mt-1 w-full' min={1} value={usersForm.start_number} onChange={(value) => updateUsersForm({ start_number: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('序号位数')}</Text>
                      <InputNumber className='mt-1 w-full' min={1} max={8} value={usersForm.number_width} onChange={(value) => updateUsersForm({ number_width: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('初始额度（显示单位）')}</Text>
                      <InputNumber className='mt-1 w-full' min={0} value={usersForm.initial_quota_display} onChange={(value) => updateUsersForm({ initial_quota_display: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('分组')}</Text>
                      <Select className='mt-1' optionList={groupOptions} value={usersForm.group} onChange={(value) => updateUsersForm({ group: value })} />
                    </div>
                    <div>
                      <Text>{t('状态')}</Text>
                      <Select className='mt-1' optionList={statusOptions} value={usersForm.status} onChange={(value) => updateUsersForm({ status: value })} />
                    </div>
                    <div>
                      <Text>{t('密码模式')}</Text>
                      <RadioGroup className='mt-2' type='button' value={usersForm.password_mode} onChange={(event) => updateUsersForm({ password_mode: event.target.value })}>
                        <Radio value='random'>{t('随机密码')}</Radio>
                        <Radio value='fixed'>{t('固定密码')}</Radio>
                      </RadioGroup>
                    </div>
                    {usersForm.password_mode === 'fixed' ? (
                      <div className='md:col-span-2'>
                        <Text>{t('固定密码')}</Text>
                        <Input className='mt-1' value={usersForm.fixed_password} onChange={(value) => updateUsersForm({ fixed_password: value })} />
                      </div>
                    ) : (
                      <div>
                        <Text>{t('随机密码长度')}</Text>
                        <InputNumber className='mt-1 w-full' min={8} max={20} value={usersForm.random_password_length} onChange={(value) => updateUsersForm({ random_password_length: Number(value || 0) })} />
                      </div>
                    )}
                    <div>
                      <Text>{t('导出格式')}</Text>
                      <Select className='mt-1' optionList={[{ label: 'TXT', value: 'txt' }, { label: 'CSV', value: 'csv' }]} value={usersForm.export_format} onChange={(value) => updateUsersForm({ export_format: value })} />
                      <Text type='tertiary' size='small' className='mt-1 block'>
                        {t('TXT 每行固定导出为 用户名,密码')}
                      </Text>
                    </div>
                    <div>
                      <Text>{t('文件名')}</Text>
                      <Input className='mt-1' value={usersForm.file_name} onChange={(value) => updateUsersForm({ file_name: value })} />
                    </div>
                  </div>
                  <Divider />
                  <Space>
                    <Button theme='solid' loading={usersLoading} onClick={previewUsers}>{t('预览')}</Button>
                    <Button theme='light' disabled={!usersPreview} loading={usersLoading} onClick={executeUsers}>{t('执行')}</Button>
                  </Space>
                </Card>
              </div>
              {renderResultPanel(usersPreview, usersResult, 'users')}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane tab={t('批量增额')} itemKey='quota'>
            <div className='grid grid-cols-1 xl:grid-cols-3 gap-4'>
              <div className='xl:col-span-2'>
                <Card>
                  <Title heading={6}>{t('参数配置')}</Title>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                    <div className='md:col-span-2'>
                      <Text>{t('作用范围')}</Text>
                      <RadioGroup className='mt-2' type='button' value={quotaForm.scope_type} onChange={(event) => updateQuotaForm({ scope_type: event.target.value })}>
                        <Radio value='all'>{t('全部用户')}</Radio>
                        <Radio value='group'>{t('按分组')}</Radio>
                      </RadioGroup>
                    </div>
                    {quotaForm.scope_type === 'group' && (
                      <div>
                        <Text>{t('目标分组')}</Text>
                        <Select className='mt-1' optionList={groupOptions} value={quotaForm.group} onChange={(value) => updateQuotaForm({ group: value })} />
                      </div>
                    )}
                    <div>
                      <Text>{t('每人增加额度（显示单位）')}</Text>
                      <InputNumber className='mt-1 w-full' min={0} value={quotaForm.quota_delta_display} onChange={(value) => updateQuotaForm({ quota_delta_display: Number(value || 0) })} />
                    </div>
                    <div>
                      <Text>{t('包含管理员')}</Text>
                      <div className='mt-2'>
                        <Switch checked={quotaForm.include_admins} onChange={(checked) => updateQuotaForm({ include_admins: checked })} />
                      </div>
                    </div>
                    <div className='md:col-span-2'>
                      <Text>{t('变更原因')}</Text>
                      <Input className='mt-1' value={quotaForm.reason} onChange={(value) => updateQuotaForm({ reason: value })} />
                    </div>
                  </div>
                  <Divider />
                  <Space>
                    <Button theme='solid' loading={quotaLoading} onClick={previewQuota}>{t('预览')}</Button>
                    <Button theme='light' type='danger' disabled={!quotaPreview} loading={quotaLoading} onClick={executeQuota}>{t('执行')}</Button>
                  </Space>
                </Card>
              </div>
              {renderResultPanel(quotaPreview, quotaResult, 'quota')}
            </div>
          </Tabs.TabPane>
        </Tabs>
      </CardPro>

      <CardPro
        type='type1'
        descriptionArea={
          <div>
            <Title heading={5} className='!mb-1'>
              {t('最近批次')}
            </Title>
            <Text type='secondary'>
              {t('这里只展示非敏感摘要；明文密码和完整兑换码不会再次出现。')}
            </Text>
          </div>
        }
      >
        <Table columns={jobsColumns} dataSource={jobs} pagination={false} loading={jobsLoading} rowKey='batch_id' />
      </CardPro>

      <Modal
        title={t('批次摘要')}
        visible={jobDetailVisible}
        footer={null}
        onCancel={() => setJobDetailVisible(false)}
        width={900}
      >
        {jobDetailLoading ? (
          <Text>{t('加载中...')}</Text>
        ) : (
          <pre className='text-xs whitespace-pre-wrap break-all bg-[var(--semi-color-fill-0)] p-3 rounded-lg overflow-auto max-h-[70vh]'>
            {JSON.stringify(jobDetail, null, 2)}
          </pre>
        )}
      </Modal>
    </div>
  );
}

export default BatchTools;
