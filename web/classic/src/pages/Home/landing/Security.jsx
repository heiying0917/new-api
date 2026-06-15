import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconTick } from '@douyinfe/semi-icons';

const Security = () => {
  const { t } = useTranslation();
  const points = [
    t('端到端加密存储,官方 Key 全程密文落库,供应商之间严格隔离、互不可见'),
    t('透明计费防套现:每笔调用按官方口径计量,成交价逐笔快照,事后改价绝不影响已结算金额'),
    t('原子幂等结算 + 资金账本,杜绝重复打款,每一分收益全程可审计、可对账'),
    t('额度自主可控:自助上传、随时启停托管,你对官方 Key 始终拥有完全掌控权'),
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
