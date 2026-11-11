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

// AWS「测试可用区域 / 批量生成可用区域密钥」的多凭证纯逻辑（不依赖 React，便于单测）。
//
// 测试结果结构 resultsByPrefix: { [凭证主体]: { error?: string, items?: [{ region, ok, status_code, latency_ms, message }] } }
//   - error：该凭证整体请求失败（后端 success=false 或网络异常）
//   - items：后端逐区域探测结果

// 多组凭证测试时凭证之间的并发上限（每组凭证在后端还会以 8 并发探测各区域）
export const AWS_CRED_TEST_CONCURRENCY = 3;

// 凭证展示用打码：保留首 4 尾 4，绝不完整展示
export const maskAwsCredential = (value) => {
  const s = String(value || '');
  if (s.length <= 8) return `${s.slice(0, 2)}****`;
  return `${s.slice(0, 4)}****${s.slice(-4)}`;
};

// 解析密钥框：每行一组凭证（忽略行内已写的 region），按凭证主体去重，保留首次出现的行号（1 起，含空行计数，便于在文本框里定位）。
// ak_sk：AK|SK[|region]；api_key：APIKey[|region]
export const parseAwsCredentials = (text, keyType) => {
  const credentials = [];
  const invalidLines = [];
  const seen = new Set();
  String(text || '')
    .split('\n')
    .forEach((raw, idx) => {
      const line = raw.trim();
      if (!line) return;
      const lineNo = idx + 1;
      const parts = line.split('|').map((p) => p.trim());
      let cred;
      if (keyType === 'api_key') {
        if (!parts[0]) {
          invalidLines.push(lineNo);
          return;
        }
        cred = {
          lineNo,
          prefix: parts[0],
          apiKey: parts[0],
          label: maskAwsCredential(parts[0]),
        };
      } else {
        if (!parts[0] || !parts[1]) {
          invalidLines.push(lineNo);
          return;
        }
        cred = {
          lineNo,
          prefix: `${parts[0]}|${parts[1]}`,
          accessKey: parts[0],
          secretKey: parts[1],
          label: maskAwsCredential(parts[0]),
        };
      }
      if (seen.has(cred.prefix)) return;
      seen.add(cred.prefix);
      credentials.push(cred);
    });
  return { credentials, invalidLines };
};

export const getAvailableRegions = (result) =>
  ((result && result.items) || [])
    .filter((item) => item.ok)
    .map((item) => item.region);

// 不可用原因：整体请求错误优先，否则取失败区域里出现最多的错误信息
export const describeUnavailableReason = (result) => {
  if (!result) return '';
  if (result.error) return result.error;
  const counts = new Map();
  (result.items || [])
    .filter((item) => !item.ok)
    .forEach((item) => {
      const msg =
        item.message || (item.status_code ? String(item.status_code) : '');
      if (!msg) return;
      counts.set(msg, (counts.get(msg) || 0) + 1);
    });
  let best = '';
  let bestCount = 0;
  counts.forEach((count, msg) => {
    if (count > bestCount) {
      best = msg;
      bestCount = count;
    }
  });
  return best;
};

// 汇总测试结果：可用凭证（附可用区域）与不可用凭证（附原因）
export const summarizeAwsCredentialResults = (credentials, resultsByPrefix) => {
  const available = [];
  const unavailable = [];
  (credentials || []).forEach((cred) => {
    const result = resultsByPrefix && resultsByPrefix[cred.prefix];
    if (!result) return;
    const regions = getAvailableRegions(result);
    if (regions.length > 0) {
      available.push({ cred, regions });
    } else {
      unavailable.push({ cred, reason: describeUnavailableReason(result) });
    }
  });
  return { available, unavailable };
};

// 测试后默认勾选：所有可用凭证可用区域的并集，按 allRegions 顺序
export const unionAvailableRegions = (
  allRegions,
  credentials,
  resultsByPrefix,
) => {
  const union = new Set();
  (credentials || []).forEach((cred) => {
    getAvailableRegions(
      resultsByPrefix && resultsByPrefix[cred.prefix],
    ).forEach((region) => union.add(region));
  });
  return (allRegions || []).filter((region) => union.has(region));
};

// 失败区域按原因分组：[{ errorCode, message, regions, regionNotEnabled }]，次数多的在前（同数按首次出现），区域保持探测顺序。
// regionNotEnabled：凭证在其他区域可用、仅该区域报 UnrecognizedClientException（AWS 原文为「token 无效」），
// 实为账号未开通该 opt-in 区域；凭证全区不可用时则是凭证本身无效，不做此判断。
export const groupFailedRegions = (result) => {
  const items = (result && result.items) || [];
  const hasAvailable = items.some((item) => item.ok);
  const groups = new Map();
  items
    .filter((item) => !item.ok)
    .forEach((item) => {
      const message =
        item.message || (item.status_code ? String(item.status_code) : '');
      if (!groups.has(message)) {
        const errorCode = item.error_code || '';
        groups.set(message, {
          errorCode,
          message,
          regions: [],
          regionNotEnabled:
            hasAvailable && errorCode === 'UnrecognizedClientException',
        });
      }
      groups.get(message).regions.push(item.region);
    });
  return Array.from(groups.values()).sort(
    (a, b) => b.regions.length - a.regions.length,
  );
};

// 「测试可用区域」测试模型的下拉候选：默认模型第一，当前选中的模型第二（手动输入的也保留，否则下拉里看不到当前值），
// 其次渠道已选模型（保持原顺序），再次 AWS 适配器支持的模型（按名称倒序：后端 map 遍历顺序随机，倒序让新模型靠前、Claude 3.x 靠后）。
// 渠道 / 适配器列表只保留 Claude（探测请求固定用 Anthropic 格式）；非数组忽略；去首尾空白、去重、忽略空值。
export const buildAwsTestModelOptions = (
  defaultModel,
  channelModels,
  adaptorModels,
  currentModel,
) => {
  const seen = new Set();
  const options = [];
  const add = (model) => {
    const name = String(model ?? '').trim();
    if (!name || seen.has(name)) return;
    seen.add(name);
    options.push(name);
  };
  const isClaude = (model) =>
    String(model ?? '')
      .toLowerCase()
      .includes('claude');
  const byNameDesc = (a, b) => (a < b ? 1 : a > b ? -1 : 0);
  add(defaultModel);
  add(currentModel);
  const asList = (value) => (Array.isArray(value) ? value : []);
  asList(channelModels).filter(isClaude).forEach(add);
  asList(adaptorModels)
    .filter(isClaude)
    .map((model) => String(model).trim())
    .sort(byNameDesc)
    .forEach(add);
  return options;
};

// 测试模型在浏览器本地记住上次的选择
export const AWS_TEST_MODEL_STORAGE_KEY = 'aws_region_test_model';

// 实际使用的测试模型：去首尾空白，为空时回退默认模型
export const resolveAwsTestModel = (value, defaultModel) =>
  String(value ?? '').trim() || defaultModel;

// 区域下拉框后缀：单凭证沿用「✓ 延迟 / ✗ 错误」，多凭证显示可用凭证数「✓ k/n」
export const regionStatusSuffix = (region, credentials, resultsByPrefix) => {
  const tested = (credentials || [])
    .map((cred) => resultsByPrefix && resultsByPrefix[cred.prefix])
    .filter(Boolean);
  if (tested.length === 0) return '';
  if (tested.length === 1) {
    const item = (tested[0].items || []).find((it) => it.region === region);
    if (!item) return '';
    // 选项里只放短错误码（完整原因在按钮下方按原因分组展示）；非 AWS API 错误无错误码，回退到原因
    return item.ok
      ? ` ✓ ${item.latency_ms}ms`
      : ` ✗ ${item.error_code || item.message || item.status_code}`;
  }
  const okCount = tested.filter((result) =>
    (result.items || []).some((it) => it.region === region && it.ok),
  ).length;
  return okCount > 0
    ? ` ✓ ${okCount}/${tested.length}`
    : ` ✗ 0/${tested.length}`;
};

// 生成密钥行：每组可用凭证 × (该凭证可用区域 ∩ 已选区域)，区域按已选顺序。
// - 全部凭证都没测过：手动模式，每组凭证 × 全部已选区域（与原单凭证行为一致）
// - 部分凭证没测过（测试后改了密钥框）：返回 untested，由调用方提示重新测试，不生成
// - 测过但无可用区域：剔除，记入 unavailable
// - 有可用区域但都没被勾选：记入 noSelected
export const buildAwsRegionKeyLines = (
  credentials,
  resultsByPrefix,
  selectedRegions,
) => {
  const creds = credentials || [];
  const results = resultsByPrefix || {};
  const selected = selectedRegions || [];
  const lines = [];
  const used = [];
  const unavailable = [];
  const noSelected = [];

  const manual = creds.every((cred) => !results[cred.prefix]);
  const untested = manual ? [] : creds.filter((cred) => !results[cred.prefix]);
  if (untested.length > 0) {
    return { lines, used, unavailable, noSelected, untested };
  }

  creds.forEach((cred) => {
    let regions;
    if (manual) {
      regions = selected;
    } else {
      const result = results[cred.prefix];
      const availableSet = new Set(getAvailableRegions(result));
      if (availableSet.size === 0) {
        unavailable.push({ cred, reason: describeUnavailableReason(result) });
        return;
      }
      regions = selected.filter((region) => availableSet.has(region));
      if (regions.length === 0) {
        noSelected.push(cred);
        return;
      }
    }
    regions.forEach((region) => lines.push(`${cred.prefix}|${region}`));
    used.push(cred);
  });

  return {
    lines: Array.from(new Set(lines)),
    used,
    unavailable,
    noSelected,
    untested,
  };
};

// 「第 2 行 AKIA****WXYZ」
export const formatAwsCredentialRef = (cred, t) =>
  t('第 {{line}} 行 {{label}}', { line: cred.lineNo, label: cred.label });

// 有限并发执行异步任务，结果按输入顺序返回
export const runWithConcurrency = async (items, limit, worker) => {
  const results = new Array(items.length);
  let next = 0;
  const runners = Array.from(
    { length: Math.max(1, Math.min(limit, items.length)) },
    async () => {
      while (next < items.length) {
        const idx = next++;
        results[idx] = await worker(items[idx], idx);
      }
    },
  );
  await Promise.all(runners);
  return results;
};
