import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconPulse, IconLayers, IconShield, IconCreditCard } from '@douyinfe/semi-icons';

const Advantages = () => {
  const { t } = useTranslation();
  const items = [
    { icon: <IconPulse />, title: t('订单量充足,Key 不闲置'),
      desc: t('平台聚合全球数百家企业的真实需求,你的官方额度持续接单,稳定产生收益。') },
    { icon: <IconLayers />, title: t('多种官方 Key 托管'),
      desc: t('支持 Claude(Anthropic)、AWS Bedrock、OpenAI 等主流官方渠道,一处托管、统一接单。') },
    { icon: <IconShield />, title: t('关键数据隔离加密'),
      desc: t('官方 Key 加密存储,供应商之间严格隔离、互不可见;遵循最小可见原则。') },
    { icon: <IconCreditCard />, title: t('多币种快速结算'),
      desc: t('按实际消耗实时计量,成交价逐笔冻结(不受事后改价影响),支持多币种、快速回款。') },
  ];
  return (
    <section className='landing-section'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('核心优势')}</span>
        <h2 className='landing-h2'>{t('为什么把官方 Key 托管给我们')}</h2>
        <div className='landing-grid-4'>
          {items.map((it, i) => (
            <div className='landing-card' key={i}>
              <div className='landing-card__icon'>{it.icon}</div>
              <h3 className='landing-card__title'>{it.title}</h3>
              <p className='landing-card__desc'>{it.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Advantages;
