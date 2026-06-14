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
import { Card, Row, Col, Avatar } from '@douyinfe/semi-ui';
import { Activity, Zap } from 'lucide-react';
import { renderNumber } from '../../helpers';
import { CARD_PROPS } from '../../constants/dashboard.constants';

const RealtimeCards = ({ realtime, t }) => {
  const items = [
    {
      key: 'rpm',
      title: t('实时 RPM'),
      value: realtime?.rpm || 0,
      icon: <Activity size={18} />,
      avatarColor: 'blue',
      desc: t('每分钟请求数'),
    },
    {
      key: 'tpm',
      title: t('实时 TPM'),
      value: realtime?.tpm || 0,
      icon: <Zap size={18} />,
      avatarColor: 'orange',
      desc: t('每分钟 Token 数'),
    },
  ];

  return (
    <Row gutter={16} className='mb-4'>
      {items.map((item) => (
        <Col xs={24} sm={12} key={item.key}>
          <Card {...CARD_PROPS} className='!rounded-2xl'>
            <div className='flex items-center'>
              <Avatar className='mr-3' size='default' color={item.avatarColor}>
                {item.icon}
              </Avatar>
              <div>
                <div className='text-xs text-gray-500'>{item.title}</div>
                <div className='text-2xl font-semibold'>
                  {renderNumber(item.value)}
                </div>
                <div className='text-xs text-gray-400'>{item.desc}</div>
              </div>
            </div>
          </Card>
        </Col>
      ))}
    </Row>
  );
};

export default RealtimeCards;
