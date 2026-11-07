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

import { describe, test, expect } from 'bun:test';
import {
  maskAwsCredential,
  parseAwsCredentials,
  summarizeAwsCredentialResults,
  unionAvailableRegions,
  regionStatusSuffix,
  buildAwsRegionKeyLines,
  runWithConcurrency,
} from './awsRegionCredentials';

const AK1 = 'AKIAAAAAAAAAAAAA1111';
const AK2 = 'AKIABBBBBBBBBBBB2222';
const AK3 = 'AKIACCCCCCCCCCCC3333';

const ok = (region, latency = 100) => ({
  region,
  ok: true,
  status_code: 200,
  latency_ms: latency,
  message: '',
});
const bad = (region, message = 'AccessDeniedException') => ({
  region,
  ok: false,
  status_code: 403,
  latency_ms: 50,
  message,
});

describe('parseAwsCredentials', () => {
  test('多行 AK|SK 全部解析，忽略行内 region，按凭证去重并保留首个行号', () => {
    const text = [
      `${AK1}|sk1`,
      '',
      `${AK2}|sk2|us-east-1`,
      `${AK1}|sk1|us-west-2`, // 与第 1 行同一凭证
      `  ${AK3} | sk3  `,
    ].join('\n');
    const { credentials, invalidLines } = parseAwsCredentials(text, 'ak_sk');
    expect(credentials.map((c) => c.prefix)).toEqual([
      `${AK1}|sk1`,
      `${AK2}|sk2`,
      `${AK3}|sk3`,
    ]);
    expect(credentials.map((c) => c.lineNo)).toEqual([1, 3, 5]);
    expect(invalidLines).toEqual([]);
  });

  test('缺 SK 的行记为格式错误', () => {
    const { credentials, invalidLines } = parseAwsCredentials(
      `${AK1}|sk1\n${AK2}`,
      'ak_sk',
    );
    expect(credentials).toHaveLength(1);
    expect(invalidLines).toEqual([2]);
  });

  test('api_key 模式取第 1 段', () => {
    const { credentials } = parseAwsCredentials(
      'key-aaa|us-east-1\nkey-bbb',
      'api_key',
    );
    expect(credentials.map((c) => c.prefix)).toEqual(['key-aaa', 'key-bbb']);
  });

  test('展示标签打码，不含完整凭证', () => {
    const { credentials } = parseAwsCredentials(`${AK1}|secret-key`, 'ak_sk');
    expect(credentials[0].label).toBe('AKIA****1111');
    expect(maskAwsCredential('short')).toBe('sh****');
  });
});

describe('buildAwsRegionKeyLines', () => {
  const { credentials } = parseAwsCredentials(
    [`${AK1}|sk1`, `${AK2}|sk2`, `${AK3}|sk3`].join('\n'),
    'ak_sk',
  );
  const results = {
    [`${AK1}|sk1`]: {
      items: [ok('us-east-1'), ok('us-west-2'), ok('eu-west-1')],
    },
    [`${AK2}|sk2`]: {
      items: [ok('us-east-1'), bad('us-west-2'), bad('eu-west-1')],
    },
    [`${AK3}|sk3`]: {
      items: [
        bad('us-east-1', 'UnrecognizedClientException'),
        bad('us-west-2', 'UnrecognizedClientException'),
        bad('eu-west-1'),
      ],
    },
  };

  test('每组凭证按各自可用区域生成（非交集），不可用凭证被剔除并带原因', () => {
    const r = buildAwsRegionKeyLines(credentials, results, [
      'us-east-1',
      'us-west-2',
      'eu-west-1',
    ]);
    expect(r.lines).toEqual([
      `${AK1}|sk1|us-east-1`,
      `${AK1}|sk1|us-west-2`,
      `${AK1}|sk1|eu-west-1`,
      `${AK2}|sk2|us-east-1`,
    ]);
    expect(r.used.map((c) => c.prefix)).toEqual([`${AK1}|sk1`, `${AK2}|sk2`]);
    expect(r.unavailable).toHaveLength(1);
    expect(r.unavailable[0].cred.lineNo).toBe(3);
    expect(r.unavailable[0].reason).toBe('UnrecognizedClientException');
    expect(r.untested).toEqual([]);
  });

  test('已选区域作为过滤器：取消勾选的区域不生成；可用区域全被取消的凭证记入 noSelected', () => {
    const r = buildAwsRegionKeyLines(credentials, results, ['eu-west-1']);
    expect(r.lines).toEqual([`${AK1}|sk1|eu-west-1`]);
    expect(r.noSelected.map((c) => c.prefix)).toEqual([`${AK2}|sk2`]);
  });

  test('整体请求失败的凭证视为不可用', () => {
    const r = buildAwsRegionKeyLines(
      credentials.slice(0, 2),
      {
        [`${AK1}|sk1`]: results[`${AK1}|sk1`],
        [`${AK2}|sk2`]: { error: '请求失败' },
      },
      ['us-east-1'],
    );
    expect(r.lines).toEqual([`${AK1}|sk1|us-east-1`]);
    expect(r.unavailable[0].reason).toBe('请求失败');
  });

  test('从未测试（手动勾选区域）：每组凭证 × 全部已选区域', () => {
    const r = buildAwsRegionKeyLines(credentials.slice(0, 2), {}, [
      'us-east-1',
      'ap-northeast-1',
    ]);
    expect(r.lines).toEqual([
      `${AK1}|sk1|us-east-1`,
      `${AK1}|sk1|ap-northeast-1`,
      `${AK2}|sk2|us-east-1`,
      `${AK2}|sk2|ap-northeast-1`,
    ]);
  });

  test('测试后新增了凭证：返回 untested，不生成任何行', () => {
    const { credentials: withNew } = parseAwsCredentials(
      [`${AK1}|sk1`, `${AK2}|sk2`, 'AKIANEWNEWNEWNEW9999|sk9'].join('\n'),
      'ak_sk',
    );
    const r = buildAwsRegionKeyLines(
      withNew,
      {
        [`${AK1}|sk1`]: results[`${AK1}|sk1`],
        [`${AK2}|sk2`]: results[`${AK2}|sk2`],
      },
      ['us-east-1'],
    );
    expect(r.lines).toEqual([]);
    expect(r.untested.map((c) => c.lineNo)).toEqual([3]);
  });

  test('生成后的密钥框再次生成结果不变（幂等）', () => {
    const first = buildAwsRegionKeyLines(credentials, results, [
      'us-east-1',
      'us-west-2',
      'eu-west-1',
    ]);
    const { credentials: again } = parseAwsCredentials(
      first.lines.join('\n'),
      'ak_sk',
    );
    const second = buildAwsRegionKeyLines(again, results, [
      'us-east-1',
      'us-west-2',
      'eu-west-1',
    ]);
    expect(second.lines).toEqual(first.lines);
  });
});

describe('summarize / union / suffix', () => {
  const { credentials } = parseAwsCredentials(
    `${AK1}|sk1\n${AK2}|sk2`,
    'ak_sk',
  );
  const results = {
    [`${AK1}|sk1`]: { items: [ok('us-east-1', 120), bad('eu-west-1')] },
    [`${AK2}|sk2`]: { items: [bad('us-east-1'), bad('eu-west-1')] },
  };

  test('summarize 区分可用与不可用', () => {
    const s = summarizeAwsCredentialResults(credentials, results);
    expect(s.available.map((a) => a.regions)).toEqual([['us-east-1']]);
    expect(s.unavailable.map((u) => u.cred.lineNo)).toEqual([2]);
  });

  test('默认勾选为可用区域并集，按全量区域顺序', () => {
    const r = unionAvailableRegions(
      ['eu-west-1', 'us-east-1', 'us-west-2'],
      credentials,
      {
        ...results,
        [`${AK2}|sk2`]: { items: [ok('us-west-2'), ok('eu-west-1')] },
      },
    );
    expect(r).toEqual(['eu-west-1', 'us-east-1', 'us-west-2']);
  });

  test('多凭证显示 k/n，单凭证沿用延迟/错误码', () => {
    expect(regionStatusSuffix('us-east-1', credentials, results)).toBe(
      ' ✓ 1/2',
    );
    expect(regionStatusSuffix('eu-west-1', credentials, results)).toBe(
      ' ✗ 0/2',
    );
    expect(
      regionStatusSuffix('us-east-1', credentials.slice(0, 1), results),
    ).toBe(' ✓ 120ms');
    expect(
      regionStatusSuffix('eu-west-1', credentials.slice(0, 1), results),
    ).toBe(' ✗ AccessDeniedException');
    expect(regionStatusSuffix('us-east-1', credentials, {})).toBe('');
  });
});

describe('runWithConcurrency', () => {
  test('按输入顺序返回结果且并发不超过上限', async () => {
    let running = 0;
    let peak = 0;
    const out = await runWithConcurrency([1, 2, 3, 4, 5], 2, async (n) => {
      running++;
      peak = Math.max(peak, running);
      await new Promise((r) => setTimeout(r, 5 * (6 - n)));
      running--;
      return n * 10;
    });
    expect(out).toEqual([10, 20, 30, 40, 50]);
    expect(peak).toBeLessThanOrEqual(2);
  });
});
