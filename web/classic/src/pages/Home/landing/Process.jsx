import React from 'react';
import { useTranslation } from 'react-i18next';

const Process = () => {
  const { t } = useTranslation();
  const steps = [
    { n: '1', title: t('接入官方 Key'), desc: t('提交你的 Claude / AWS / OpenAI 官方凭证,加密入库。') },
    { n: '2', title: t('平台智能分发'), desc: t('全球企业订单按规则分发到你的渠道,真实流量、稳定调用。') },
    { n: '3', title: t('实时计量计费'), desc: t('每笔调用按官方口径计量,成交价即时快照,账目清晰可对账。') },
    { n: '4', title: t('多币种结算'), desc: t('按周期生成结算单,核对无误后快速打款,支持多币种。') },
  ];
  return (
    <section className='landing-section landing-section--alt'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('托管流程')}</span>
        <h2 className='landing-h2'>{t('四步开始,简单透明')}</h2>
        <div className='landing-grid-4'>
          {steps.map((s) => (
            <div className='landing-step' key={s.n}>
              <div className='landing-step__num'>{s.n}</div>
              <h3 className='landing-card__title'>{s.title}</h3>
              <p className='landing-card__desc'>{s.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Process;
