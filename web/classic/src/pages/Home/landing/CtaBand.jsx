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
          {t('现在就开始托管,让你的官方额度产生稳定收益')}
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
