import React, { useContext, useEffect, useRef, useState } from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../context/Status';
import FacetLogo from '../../../components/common/logo/FacetLogo';

// 页面加载后数字滚动动画（0 → 目标值，easeOutCubic），支持错峰延迟
function useCountUp(target, { duration = 1600, delay = 0 } = {}) {
  const [val, setVal] = useState(0);
  useEffect(() => {
    let raf = null;
    let timer = null;
    let startTs = null;
    const ease = (t) => 1 - Math.pow(1 - t, 3);
    const tick = (ts) => {
      if (startTs === null) startTs = ts;
      const p = Math.min((ts - startTs) / duration, 1);
      setVal(Math.round(target * ease(p)));
      if (p < 1) raf = requestAnimationFrame(tick);
    };
    timer = setTimeout(() => {
      raf = requestAnimationFrame(tick);
    }, delay);
    return () => {
      if (timer) clearTimeout(timer);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [target, duration, delay]);
  return val;
}

const fmtUSD = (n) => '$' + Math.round(n).toLocaleString('en-US');

const AnimatedAmount = ({ value, delay = 0 }) => {
  const v = useCountUp(value, { delay });
  return <>{fmtUSD(v)}</>;
};

const Hero = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);

  const fallbackStats = [
    { value: '', label: t('数百家'), caption: t('全球企业客户') },
    { value: '', label: t('全天候'), caption: t('订单不间断') },
    { value: '', label: t('多币种'), caption: t('实时结算') },
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

  // 正在托管的 Key（按官 Key 类型展示供应商数量 + 累计托管金额，金额以美元计依次降低）
  const sampleRows = [
    { name: 'Claude (Anthropic)', suppliers: 18, amount: 8247360 },
    { name: 'AWS Bedrock', suppliers: 26, amount: 4612980 },
    { name: 'OpenRouter', suppliers: 8, amount: 1358420 },
    { name: 'OpenAI', suppliers: 6, amount: 842170 },
  ];
  const total = sampleRows.reduce((sum, r) => sum + r.amount, 0);

  return (
    <section className='landing-hero'>
      <div className='landing-hero__bg' aria-hidden='true' />
      <div className='landing-container landing-hero__grid'>
        <div className='landing-hero__copy'>
          <div
            className='landing-hero__brand'
            style={{ display: 'flex', alignItems: 'center', gap: '14px', marginBottom: '20px' }}
          >
            <FacetLogo size={44} animate='auto' />
            <span className='landing-eyebrow' style={{ marginBottom: 0 }}>
              {t('专业加密 · 安全托管')}
            </span>
          </div>
          <h1 className='landing-hero__title'>
            {t('企业级官 Key 托管平台,一键接入全球 AI 算力市场')}
          </h1>
          <p className='landing-hero__sub'>
            {t(
              'TokenKi 平台为全球数百家企业提供稳定 AI 算力,支持 Claude、AWS、OpenRouter、OpenAI 等官 Key 托管。一键上传,加密存储,透明计费,多币种实时结算,让每一位供应商都能获得高额收益。',
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
        <div className='landing-panel' role='img' aria-label={t('已托管的官 Key')}>
          <div className='landing-panel__head'>
            <span>{t('已托管的官 Key')}</span>
          </div>
          {sampleRows.map((r, i) => (
            <div className='landing-panel__row' key={r.name}>
              <div className='landing-panel__name'>
                <div>{r.name}</div>
                <div className='landing-panel__mask'>{t('供应商数量')}：{r.suppliers} {t('个')}</div>
              </div>
              <div className='landing-panel__amt'>
                <AnimatedAmount value={r.amount} delay={i * 140} />
              </div>
            </div>
          ))}
          <div className='landing-panel__total'>
            <span>{t('累计托管金额')}</span>
            <span className='landing-panel__total-amt'>
              <AnimatedAmount value={total} delay={sampleRows.length * 140} />
            </span>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Hero;
