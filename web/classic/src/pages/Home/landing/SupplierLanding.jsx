import React from 'react';
import './landing.css';
import Hero from './Hero';
import Advantages from './Advantages';
import Process from './Process';
import Security from './Security';
import Channels from './Channels';
import CtaBand from './CtaBand';

const SupplierLanding = () => (
  <div className='supplier-landing w-full overflow-x-hidden bg-semi-color-bg-0'>
    <Hero />
    <Advantages />
    <Process />
    <Security />
    <Channels />
    <CtaBand />
  </div>
);

export default SupplierLanding;
