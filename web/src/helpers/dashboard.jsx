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
import { Divider, Empty } from '@douyinfe/semi-ui';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import {
  timestamp2string,
  timestamp2string1,
  isDataCrossYear,
  copy,
  showSuccess,
} from './utils';
import {
  STORAGE_KEYS,
  DEFAULT_TIME_INTERVALS,
  DEFAULTS,
  ILLUSTRATION_SIZE,
  UPTIME_HISTORY_BLOCKS,
  UPTIME_HISTORY_PLACEHOLDER_COLOR,
} from '../constants/dashboard.constants';

// ========== 时间相关工具函数 ==========
export const getDefaultTime = () => {
  return localStorage.getItem(STORAGE_KEYS.DATA_EXPORT_DEFAULT_TIME) || 'hour';
};

export const getTimeInterval = (timeType, isSeconds = false) => {
  const intervals =
    DEFAULT_TIME_INTERVALS[timeType] || DEFAULT_TIME_INTERVALS.hour;
  return isSeconds ? intervals.seconds : intervals.minutes;
};

export const getInitialTimestamp = () => {
  const defaultTime = getDefaultTime();
  const now = new Date().getTime() / 1000;

  switch (defaultTime) {
    case 'hour':
      return timestamp2string(now - 86400);
    case 'week':
      return timestamp2string(now - 86400 * 30);
    default:
      return timestamp2string(now - 86400 * 7);
  }
};

// ========== 数据处理工具函数 ==========
export const updateMapValue = (map, key, value) => {
  if (!map.has(key)) {
    map.set(key, 0);
  }
  map.set(key, map.get(key) + value);
};

export const initializeMaps = (key, ...maps) => {
  maps.forEach((map) => {
    if (!map.has(key)) {
      map.set(key, 0);
    }
  });
};

// ========== 图表相关工具函数 ==========
export const updateChartSpec = (
  setterFunc,
  newData,
  subtitle,
  newColors,
  dataId,
) => {
  setterFunc((prev) => ({
    ...prev,
    data: [{ id: dataId, values: newData }],
    title: {
      ...prev.title,
      subtext: subtitle,
    },
    color: {
      specified: newColors,
    },
  }));
};

export const getTrendSpec = (data, color) => ({
  type: 'line',
  data: [{ id: 'trend', values: data.map((val, idx) => ({ x: idx, y: val })) }],
  xField: 'x',
  yField: 'y',
  height: 40,
  width: 100,
  axes: [
    {
      orient: 'bottom',
      visible: false,
    },
    {
      orient: 'left',
      visible: false,
    },
  ],
  padding: 0,
  autoFit: false,
  legends: { visible: false },
  tooltip: { visible: false },
  crosshair: { visible: false },
  line: {
    style: {
      stroke: color,
      lineWidth: 2,
    },
  },
  point: {
    visible: false,
  },
  background: {
    fill: 'transparent',
  },
});

// ========== UI 工具函数 ==========
export const createSectionTitle = (Icon, text) => (
  <div className='flex items-center gap-2'>
    <Icon size={16} />
    {text}
  </div>
);

export const createFormField = (Component, props, FORM_FIELD_PROPS) => (
  <Component {...FORM_FIELD_PROPS} {...props} />
);

// ========== 操作处理函数 ==========
export const handleCopyUrl = async (url, t) => {
  if (await copy(url)) {
    showSuccess(t('复制成功'));
  }
};

export const handleSpeedTest = (apiUrl) => {
  const encodedUrl = encodeURIComponent(apiUrl);
  const speedTestUrl = `https://www.tcptest.cn/http/${encodedUrl}`;
  window.open(speedTestUrl, '_blank', 'noopener,noreferrer');
};

// ========== 状态映射函数 ==========
export const getUptimeStatusColor = (status, uptimeStatusMap) =>
  uptimeStatusMap[status]?.color || '#8b9aa7';

export const getUptimeStatusText = (status, uptimeStatusMap, t) =>
  t(uptimeStatusMap[status]?.text || '未知');

// ========== 监控列表渲染函数 ==========
export const renderMonitorList = (
  monitors,
  getUptimeStatusColor,
  getUptimeStatusText,
  t,
) => {
  if (!monitors || monitors.length === 0) {
    return (
      <div className='flex justify-center items-center py-4'>
        <Empty
          image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
          darkModeImage={
            <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
          }
          title={t('暂无监控数据')}
        />
      </div>
    );
  }

  const grouped = {};
  monitors.forEach((m) => {
    const g = m.group || '';
    if (!grouped[g]) grouped[g] = [];
    grouped[g].push(m);
  });

  const normalizeHeartbeatHistory = (heartbeats) => {
    const values = Array.isArray(heartbeats)
      ? heartbeats.slice(-UPTIME_HISTORY_BLOCKS)
      : [];
    const missingCount = Math.max(0, UPTIME_HISTORY_BLOCKS - values.length);
    return [
      ...Array.from({ length: missingCount }, () => null),
      ...values.map((status) => (Number.isInteger(status) ? status : null)),
    ];
  };

  const getStatusBadgeStyle = (status) => {
    const color = getUptimeStatusColor(status);
    return {
      color,
      backgroundColor: `${color}1A`,
    };
  };

  const renderItem = (monitor, idx) => (
    <div
      key={idx}
      className='p-3 hover:bg-white rounded-xl transition-colors border border-gray-100'
    >
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2 min-w-0'>
          <div
            className='w-2.5 h-2.5 rounded-full flex-shrink-0'
            style={{ backgroundColor: getUptimeStatusColor(monitor.status) }}
          />
          <span className='text-sm font-medium text-gray-900 truncate'>
            {monitor.name}
          </span>
        </div>
        <span
          className='text-xs font-medium px-2 py-1 rounded-full whitespace-nowrap'
          style={getStatusBadgeStyle(monitor.status)}
        >
          {getUptimeStatusText(monitor.status)}
        </span>
      </div>

      <div className='mt-3'>
        <div
          className='grid gap-1'
          style={{
            gridTemplateColumns: `repeat(${UPTIME_HISTORY_BLOCKS}, minmax(0, 1fr))`,
          }}
        >
          {normalizeHeartbeatHistory(monitor.heartbeats).map((status, index) => (
            <div
              key={`${monitor.name}-${index}`}
              className='h-4 rounded-[3px]'
              style={{
                backgroundColor:
                  status === null
                    ? UPTIME_HISTORY_PLACEHOLDER_COLOR
                    : getUptimeStatusColor(status),
                opacity: status === null ? 0.45 : 1,
              }}
            />
          ))}
        </div>
        <div className='mt-2 flex items-center justify-between text-[11px] text-gray-500'>
          <span>{t('最近50次检测，最新在右')}</span>
          <span>{`24h ${((monitor.uptime || 0) * 100).toFixed(1)}%`}</span>
        </div>
      </div>
    </div>
  );

  return Object.entries(grouped).map(([gname, list]) => (
    <div key={gname || 'default'} className='mb-2'>
      {gname && (
        <>
          <div className='text-md font-semibold text-gray-500 px-2 py-1'>
            {gname}
          </div>
          <Divider />
        </>
      )}
      {list.map(renderItem)}
    </div>
  ));
};

// ========== 数据处理函数 ==========
export const processRawData = (
  data,
  dataExportDefaultTime,
  initializeMaps,
  updateMapValue,
) => {
  const result = {
    totalQuota: 0,
    totalTimes: 0,
    totalTokens: 0,
    uniqueModels: new Set(),
    timePoints: [],
    timeQuotaMap: new Map(),
    timeTokensMap: new Map(),
    timeCountMap: new Map(),
  };

  // 检查数据是否跨年
  const showYear = isDataCrossYear(data.map((item) => item.created_at));

  data.forEach((item) => {
    result.uniqueModels.add(item.model_name);
    result.totalTokens += item.token_used;
    result.totalQuota += item.quota;
    result.totalTimes += item.count;

    const timeKey = timestamp2string1(
      item.created_at,
      dataExportDefaultTime,
      showYear,
    );
    if (!result.timePoints.includes(timeKey)) {
      result.timePoints.push(timeKey);
    }

    initializeMaps(
      timeKey,
      result.timeQuotaMap,
      result.timeTokensMap,
      result.timeCountMap,
    );
    updateMapValue(result.timeQuotaMap, timeKey, item.quota);
    updateMapValue(result.timeTokensMap, timeKey, item.token_used);
    updateMapValue(result.timeCountMap, timeKey, item.count);
  });

  result.timePoints.sort();
  return result;
};

export const calculateTrendData = (
  timePoints,
  timeQuotaMap,
  timeTokensMap,
  timeCountMap,
  dataExportDefaultTime,
) => {
  const quotaTrend = timePoints.map((time) => timeQuotaMap.get(time) || 0);
  const tokensTrend = timePoints.map((time) => timeTokensMap.get(time) || 0);
  const countTrend = timePoints.map((time) => timeCountMap.get(time) || 0);

  const rpmTrend = [];
  const tpmTrend = [];

  if (timePoints.length >= 2) {
    const interval = getTimeInterval(dataExportDefaultTime);

    for (let i = 0; i < timePoints.length; i++) {
      rpmTrend.push(timeCountMap.get(timePoints[i]) / interval);
      tpmTrend.push(timeTokensMap.get(timePoints[i]) / interval);
    }
  }

  return {
    balance: [],
    usedQuota: [],
    requestCount: [],
    times: countTrend,
    consumeQuota: quotaTrend,
    tokens: tokensTrend,
    rpm: rpmTrend,
    tpm: tpmTrend,
  };
};

export const aggregateDataByTimeAndModel = (data, dataExportDefaultTime) => {
  const aggregatedData = new Map();

  // 检查数据是否跨年
  const showYear = isDataCrossYear(data.map((item) => item.created_at));

  data.forEach((item) => {
    const timeKey = timestamp2string1(
      item.created_at,
      dataExportDefaultTime,
      showYear,
    );
    const modelKey = item.model_name;
    const key = `${timeKey}-${modelKey}`;

    if (!aggregatedData.has(key)) {
      aggregatedData.set(key, {
        time: timeKey,
        model: modelKey,
        quota: 0,
        count: 0,
      });
    }

    const existing = aggregatedData.get(key);
    existing.quota += item.quota;
    existing.count += item.count;
  });

  return aggregatedData;
};

export const generateChartTimePoints = (
  aggregatedData,
  data,
  dataExportDefaultTime,
) => {
  let chartTimePoints = Array.from(
    new Set([...aggregatedData.values()].map((d) => d.time)),
  );

  if (chartTimePoints.length < DEFAULTS.MAX_TREND_POINTS) {
    const lastTime = Math.max(...data.map((item) => item.created_at));
    const interval = getTimeInterval(dataExportDefaultTime, true);

    // 生成时间点数组，用于检查是否跨年
    const generatedTimestamps = Array.from(
      { length: DEFAULTS.MAX_TREND_POINTS },
      (_, i) => lastTime - (6 - i) * interval,
    );
    const showYear = isDataCrossYear(generatedTimestamps);

    chartTimePoints = generatedTimestamps.map((ts) =>
      timestamp2string1(ts, dataExportDefaultTime, showYear),
    );
  }

  return chartTimePoints;
};
