import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconPulse, IconLayers, IconShield, IconCreditCard } from '@douyinfe/semi-icons';

const Advantages = () => {
  const { t } = useTranslation();
  const items = [
    { icon: <IconPulse />, title: t('订单充沛,额度不再闲置'),
      desc: t('平台汇聚全球数百家企业的真实算力需求,你的官方 Key 持续接单、稳定运转,把沉睡的额度变成实打实的收益。') },
    { icon: <IconLayers />, title: t('全渠道托管,自助秒级接入'),
      desc: t('一处托管 Claude(Anthropic)、AWS Bedrock、OpenRouter、OpenAI 等主流官方渠道,自助上传、即刻生效,统一接单、集中管理。') },
    { icon: <IconShield />, title: t('数据加密隔离,安全有保障'),
      desc: t('官方 Key 全程密文存储,供应商之间严格隔离、互不可见,遵循最小可见原则,把每一份凭证都护在保险箱里。') },
    { icon: <IconCreditCard />, title: t('透明计费,多币种实时结算'),
      desc: t('按官方口径逐笔计量,成交价即时快照、明细可对账;支持多币种实时结算,收益看得见、回款不用等。') },
  ];
  return (
    <section className='landing-section'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('核心优势')}</span>
        <h2 className='landing-h2'>{t('为什么把官方 Key 交给我们托管')}</h2>
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
