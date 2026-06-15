import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconTick } from '@douyinfe/semi-icons';

const Security = () => {
  const { t } = useTranslation();
  const points = [
    t('端到端加密存储,供应商之间数据严格隔离、互不可见'),
    t('防套现结算:成交价逐笔快照,事后改价不影响已结算金额'),
    t('原子幂等结算 + 资金账本,杜绝重复打款、全程可审计对账'),
    t('账号级登录防暴破、可吊销会话、SSRF 校验等纵深防护'),
  ];
  return (
    <section className='landing-section'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('安全保障')}</span>
        <h2 className='landing-h2'>{t('把安全做到供应商敢托付的程度')}</h2>
        <ul className='landing-seclist'>
          {points.map((p, i) => (
            <li className='landing-seclist__item' key={i}>
              <IconTick className='landing-seclist__tick' />
              <span>{p}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
};

export default Security;
