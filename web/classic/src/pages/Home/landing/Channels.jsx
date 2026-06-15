import React from 'react';
import { useTranslation } from 'react-i18next';

const Channels = () => {
  const { t } = useTranslation();
  const list = ['Claude (Anthropic)', 'AWS Bedrock', 'OpenRouter', 'OpenAI', t('更多官方渠道持续接入')];
  return (
    <section className='landing-section landing-section--alt'>
      <div className='landing-container'>
        <span className='landing-eyebrow'>{t('支持的官方渠道')}</span>
        <h2 className='landing-h2'>{t('主流官方渠道,一处托管统一接单')}</h2>
        <div className='landing-badges'>
          {list.map((c) => (
            <div className='landing-badge' key={c}>{c}</div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Channels;
