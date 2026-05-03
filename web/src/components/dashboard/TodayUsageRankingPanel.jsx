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
import { Avatar, Card, Empty, Spin, Tag } from '@douyinfe/semi-ui';
import { Trophy } from 'lucide-react';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import ScrollableContainer from '../common/ui/ScrollableContainer';
import { formatTokenMillions } from '../../helpers/usageRanking';

const rankColors = ['amber', 'grey', 'orange'];

const TodayUsageRankingPanel = ({
  rankingData,
  rankingLoading,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
  t,
}) => {
  const top = rankingData?.top || [];

  return (
    <Card
      {...CARD_PROPS}
      className='bg-gray-50 border-0 !rounded-2xl'
      title={
        <div className='flex items-center justify-between w-full gap-2'>
          <div className={FLEX_CENTER_GAP2}>
            <Trophy size={16} />
            {t('今日Token榜')}
          </div>
          <Tag color='white' shape='circle'>
            Top {rankingData?.limit || 10}
          </Tag>
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <Spin spinning={rankingLoading}>
        <ScrollableContainer maxHeight='24rem'>
          {top.length > 0 ? (
            <div className='p-2 space-y-2'>
              {top.map((item) => (
                <div
                  key={item.user_id}
                  className={`flex items-center gap-3 rounded-lg px-2 py-2 transition-colors ${
                    item.is_me
                      ? 'bg-blue-50 ring-1 ring-blue-100'
                      : 'hover:bg-white'
                  }`}
                >
                  <Avatar
                    size='extra-small'
                    color={rankColors[item.rank - 1] || 'blue'}
                  >
                    {item.rank}
                  </Avatar>
                  <div className='min-w-0 flex-1'>
                    <div className='flex items-center justify-between gap-2'>
                      <span className='truncate text-sm font-medium text-gray-900'>
                        {item.username || `User ${item.user_id}`}
                      </span>
                      {item.is_me && (
                        <Tag color='blue' size='small' shape='circle'>
                          {t('我')}
                        </Tag>
                      )}
                    </div>
                    <div className='mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500'>
                      <span>
                        {t('Token消耗')} {formatTokenMillions(item.token_count)}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className='flex justify-center items-center min-h-[20rem] w-full'>
              <Empty
                image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
                darkModeImage={
                  <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
                }
                title={t('今日暂无使用记录')}
              />
            </div>
          )}
        </ScrollableContainer>
      </Spin>
    </Card>
  );
};

export default TodayUsageRankingPanel;
