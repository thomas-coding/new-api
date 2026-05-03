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
import { Card, Spin, Tag } from '@douyinfe/semi-ui';
import { Medal } from 'lucide-react';
import { formatTokenMillions } from '../../helpers/usageRanking';

const statClass =
  'rounded-lg bg-gray-50 px-3 py-2 min-h-[4.25rem] flex flex-col justify-center';

const MyUsageRankingPanel = ({
  rankingData,
  rankingLoading,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  t,
}) => {
  const me = rankingData?.me || {};
  const hasRank = Number(me.rank || 0) > 0;

  return (
    <Card
      {...CARD_PROPS}
      className='shadow-sm !rounded-2xl lg:col-span-1'
      title={
        <div className='flex items-center justify-between w-full gap-2'>
          <div className={FLEX_CENTER_GAP2}>
            <Medal size={16} />
            {t('我的今日排名')}
          </div>
          <Tag color='white' shape='circle'>
            UTC+8
          </Tag>
        </div>
      }
    >
      <Spin spinning={rankingLoading}>
        <div className='space-y-3'>
          <div className='flex items-baseline justify-between gap-3'>
            <div className='min-w-0'>
              <div className='truncate text-sm text-gray-500'>
                {me.username || t('暂无用户')}
              </div>
              <div className='mt-1 text-3xl font-semibold text-gray-900'>
                {hasRank ? `#${me.rank}` : t('暂无排名')}
              </div>
            </div>
            <Tag color={hasRank ? 'blue' : 'grey'} shape='circle'>
              {t('北京时间今日')}
            </Tag>
          </div>

          <div className='grid grid-cols-1 gap-2'>
            <div className={statClass}>
              <span className='text-xs text-gray-500'>
                {t('今日Token消耗')}
              </span>
              <span className='mt-1 text-base font-semibold text-gray-900'>
                {formatTokenMillions(me.token_count)}
              </span>
            </div>
          </div>
        </div>
      </Spin>
    </Card>
  );
};

export default MyUsageRankingPanel;
