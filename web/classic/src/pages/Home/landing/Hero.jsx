import React, { useContext } from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../context/Status';

const Hero = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);

  const fallbackStats = [
    { value: '', label: t('数百家'), caption: t('企业客户') },
    { value: '', label: t('持续'), caption: t('充足订单') },
    { value: '', label: t('多币种'), caption: t('快速结算') },
    { value: '', label: t('端到端'), caption: t('加密隔离') },
  ];
  let stats = fallbackStats;
  const raw = statusState?.status?.HomeSupplierStats;
  if (raw) {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) {
        stats = parsed.slice(0, 4).map((s) => ({
          value: s.value || '',
          label: s.label || '',
          caption: s.caption || '',
        }));
      }
    } catch (e) {
      /* 解析失败保留定性默认 */
    }
  }

  const sampleRows = [
    { name: 'Claude (Anthropic)', masked: 'sk-ant-••••', amount: '¥ 12,480' },
    { name: 'AWS Bedrock', masked: 'AKIA••••', amount: '¥ 8,920' },
    { name: 'OpenAI', masked: 'sk-••••', amount: '¥ 6,310' },
  ];

  return (
    <section className='landing-hero'>
      <div className='landing-hero__bg' aria-hidden='true' />
      <div className='landing-container landing-hero__grid'>
        <div className='landing-hero__copy'>
          <span className='landing-eyebrow'>{t('面向供应商 · 官方额度变现')}</span>
          <h1 className='landing-hero__title'>
            {t('托管你的官方 Key,接入全球订单')}
          </h1>
          <p className='landing-hero__sub'>
            {t(
              '我们为全球数百家企业稳定供应 AI 大模型 token,订单量充足。把你闲置的 Claude / AWS / OpenAI 官方额度托管给我们——安全隔离、实时计量、多币种快速结算。',
            )}
          </p>
          <div className='landing-hero__cta'>
            <Link to='/register'>
              <Button theme='solid' type='primary' size='large' className='!rounded-xl px-7'>
                {t('成为供应商')}
              </Button>
            </Link>
            <Link to='/login'>
              <Button size='large' className='!rounded-xl px-7'>
                {t('登录控制台')}
              </Button>
            </Link>
          </div>
          <div className='landing-stats'>
            {stats.map((s, i) => (
              <div className='landing-stats__item' key={i}>
                <div className='landing-stats__value'>{s.value || s.label}</div>
                <div className='landing-stats__label'>{s.value ? s.label : s.caption}</div>
              </div>
            ))}
          </div>
        </div>
        <div className='landing-panel' role='img' aria-label={t('收益示意')}>
          <div className='landing-panel__head'>
            <span>{t('我的托管渠道')}</span>
            <span className='landing-panel__badge'>{t('示意')}</span>
          </div>
          {sampleRows.map((r) => (
            <div className='landing-panel__row' key={r.name}>
              <div className='landing-panel__name'>
                <div>{r.name}</div>
                <div className='landing-panel__mask'>{r.masked} · {t('已加密')}</div>
              </div>
              <div className='landing-panel__amt'>{r.amount}</div>
            </div>
          ))}
          <div className='landing-panel__total'>
            <span>{t('本周期累计收益(示意)')}</span>
            <span className='landing-panel__total-amt'>¥ 27,710</span>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Hero;
