/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useId } from 'react';
import './FacetLogo.css';

// 六块三角碎片（固化坐标，来自设计交付 token-ki-facet）。
// dx/dy/rot/delay 为入场汇聚动效参数：碎片从中心向外、带旋转缩放飞入归位。
const SHARDS = [
  { d: 'M100 100 L 100 26 L 164.09 63 Z', fill: '#bae6fd', dx: 29, dy: -50.23, rot: -26, delay: 0 },
  { d: 'M100 100 L 164.09 63 L 164.09 137 Z', fill: '#7dd3fc', dx: 58, dy: 0, rot: 26, delay: 0.07 },
  { d: 'M100 100 L 164.09 137 L 100 174 Z', fill: '#38bdf8', dx: 29, dy: 50.23, rot: -26, delay: 0.14 },
  { d: 'M100 100 L 100 174 L 35.91 137 Z', fill: '#3b82f6', dx: -29, dy: 50.23, rot: 26, delay: 0.21 },
  { d: 'M100 100 L 35.91 137 L 35.91 63 Z', fill: '#60a5fa', dx: -58, dy: 0, rot: -26, delay: 0.28 },
  { d: 'M100 100 L 35.91 63 L 100 26 Z', fill: '#93c5fd', dx: -29, dy: -50.23, rot: 26, delay: 0.35 },
];

// animate: 'static' | 'entrance' | 'pulse' | 'hover' | 'auto'
const MODE_CLASS = {
  static: '',
  entrance: 'is-entrance',
  pulse: 'is-pulse',
  hover: 'is-hover',
  auto: 'is-entrance is-pulse is-hover',
};

const FacetLogo = ({ size = 64, animate = 'static', className = '', style = {}, title = 'Token Ki' }) => {
  const uid = useId().replace(/:/g, '');
  const bgId = `facetBg-${uid}`;
  const clipId = `facetClip-${uid}`;
  const glowId = `facetGlow-${uid}`;
  const modeCls = MODE_CLASS[animate] ?? '';

  return (
    <span
      className={`facet-tile ${modeCls} ${className}`.trim()}
      style={{ width: size, height: size, ...style }}
    >
      <svg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg' role='img' aria-label={title}>
        <defs>
          <radialGradient id={bgId} cx='30%' cy='22%' r='120%'>
            <stop offset='0%' stopColor='#143672' />
            <stop offset='38%' stopColor='#0b2350' />
            <stop offset='100%' stopColor='#06112e' />
          </radialGradient>
          <radialGradient id={glowId}>
            <stop offset='0%' stopColor='#bae6fd' stopOpacity='0.9' />
            <stop offset='100%' stopColor='#bae6fd' stopOpacity='0' />
          </radialGradient>
          <clipPath id={clipId}>
            <rect width='200' height='200' rx='44' />
          </clipPath>
        </defs>
        <g clipPath={`url(#${clipId})`}>
          <rect width='200' height='200' fill={`url(#${bgId})`} />
          {SHARDS.map((s, i) => (
            <path
              key={i}
              className='facet-shard'
              d={s.d}
              fill={s.fill}
              stroke='#06112e'
              strokeWidth='2.4'
              strokeLinejoin='round'
              style={{
                '--dx': `${s.dx}px`,
                '--dy': `${s.dy}px`,
                '--rot': `${s.rot}deg`,
                '--d': `${s.delay}s`,
              }}
            />
          ))}
          {/* 核心光晕（呼吸脉动用）*/}
          <circle className='facet-halo' cx='100' cy='100' r='30' fill={`url(#${glowId})`} />
          {/* 核心圆 */}
          <circle className='facet-core' cx='100' cy='100' r='7.5' fill='#eaf6ff' />
        </g>
      </svg>
    </span>
  );
};

export default FacetLogo;
