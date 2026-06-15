import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const CtaBand = () => {
  const { t } = useTranslation();
  return (
    <section className='landing-ctaband'>
      <div className='landing-container landing-ctaband__inner'>
        <h2 className='landing-ctaband__title'>
          {t('立即托管你的官方 Key,让闲置额度转化为稳定收入')}
        </h2>
        <Link to='/register'>
          <Button theme='solid' type='primary' size='large' className='!rounded-xl px-8'>
            {t('成为供应商')}
          </Button>
        </Link>
      </div>
    </section>
  );
};

export default CtaBand;
