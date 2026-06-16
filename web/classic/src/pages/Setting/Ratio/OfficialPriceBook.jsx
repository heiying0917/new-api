/*
Copyright (C) 2023-2026 QuantumNous

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
import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Table,
  Tag,
  Typography,
  Select,
  TextArea,
  Space,
  Spin,
} from '@douyinfe/semi-ui';
import { RefreshCcw, CheckSquare } from 'lucide-react';
import { API, showError, showInfo, showSuccess, showWarning } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

// 1 ratio unit = $0.002 / 1K tokens = $2 / 1M tokens, so input $/1M = ratio * 2.
const RATIO_TO_USD_PER_1M = 2;

function fmtNum(v) {
  return Number(Number(v).toFixed(4));
}

function formatInputUsd(modelRatio) {
  if (modelRatio === undefined || modelRatio === null) return '-';
  return `$${fmtNum(modelRatio * RATIO_TO_USD_PER_1M)}/1M`;
}

function formatRatioSet(rs, t) {
  if (!rs || rs.model_ratio === undefined || rs.model_ratio === null)
    return t('未设置价格');
  const parts = [`${t('输入')} ${formatInputUsd(rs.model_ratio)}`];
  if (rs.completion_ratio !== undefined && rs.completion_ratio !== null)
    parts.push(`${t('输出')} ${fmtNum(rs.completion_ratio)}×`);
  if (rs.cache_ratio !== undefined && rs.cache_ratio !== null)
    parts.push(`${t('缓存')} ${fmtNum(rs.cache_ratio)}×`);
  if (rs.create_cache_ratio !== undefined && rs.create_cache_ratio !== null)
    parts.push(`${t('缓存写入')} ${fmtNum(rs.create_cache_ratio)}×`);
  return parts.join(' · ');
}

function parseManualModels(raw) {
  return raw
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

export default function OfficialPriceBook(props) {
  const { t } = useTranslation();
  const refreshParent = props.refresh;

  const [meta, setMeta] = useState(null);
  const [metaLoading, setMetaLoading] = useState(false);
  const [scopeKind, setScopeKind] = useState('all_missing');
  const [manualModels, setManualModels] = useState('');
  const [mode, setMode] = useState('missing_only');
  const [preview, setPreview] = useState(null);
  const [selected, setSelected] = useState([]);
  const [refreshing, setRefreshing] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [applying, setApplying] = useState(false);

  const loadMeta = async () => {
    setMetaLoading(true);
    try {
      const res = await API.get('/api/pricing/official_book/');
      if (res.data.success) setMeta(res.data.data);
    } catch (e) {
      // silent: meta is best-effort
    } finally {
      setMetaLoading(false);
    }
  };

  useEffect(() => {
    loadMeta();
  }, []);

  const isBusy = refreshing || previewing || applying;

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const res = await API.post('/api/pricing/official_book/refresh');
      if (!res.data.success) {
        showError(res.data.message || t('刷新官方价手册失败'));
        return;
      }
      setMeta(res.data.data);
      showSuccess(
        t('官方价手册已刷新：{{count}} 个模型').replace(
          '{{count}}',
          res.data.data.model_count,
        ),
      );
    } catch (e) {
      showError(t('刷新官方价手册失败') + '：' + e.message);
    } finally {
      setRefreshing(false);
    }
  };

  const handlePreview = async () => {
    let scope;
    if (scopeKind === 'models') {
      const models = parseManualModels(manualModels);
      if (models.length === 0) {
        showWarning(t('请至少输入一个模型名'));
        return;
      }
      scope = { kind: 'models', models };
    } else {
      scope = { kind: 'all_missing' };
    }
    setPreviewing(true);
    try {
      const res = await API.post('/api/pricing/official_book/preview', {
        scope,
        mode,
      });
      if (!res.data.success) {
        showError(res.data.message || t('预览官方价格失败'));
        return;
      }
      const data = res.data.data;
      setPreview(data);
      const next = (data.rows || [])
        .filter((r) => r.action !== 'skip')
        .map((r) => r.model);
      setSelected(next);
      if ((data.rows || []).length === 0 && (data.unmatched || []).length === 0) {
        showInfo(t('没有需要填充的模型'));
      }
    } catch (e) {
      showError(t('预览官方价格失败') + '：' + e.message);
    } finally {
      setPreviewing(false);
    }
  };

  const handleApply = async () => {
    if (selected.length === 0) {
      showWarning(t('请至少选择一个模型'));
      return;
    }
    setApplying(true);
    try {
      const res = await API.post('/api/pricing/official_book/apply', {
        models: selected,
      });
      if (!res.data.success) {
        showError(res.data.message || t('应用官方价格失败'));
        return;
      }
      showSuccess(
        t('已为 {{count}} 个模型应用官方价格').replace(
          '{{count}}',
          res.data.data.applied_count,
        ),
      );
      setPreview(null);
      setSelected([]);
      if (typeof refreshParent === 'function') refreshParent();
    } catch (e) {
      showError(t('应用官方价格失败') + '：' + e.message);
    } finally {
      setApplying(false);
    }
  };

  const actionableRows = useMemo(
    () => (preview?.rows || []).filter((r) => r.action !== 'skip'),
    [preview],
  );

  const columns = [
    { title: t('模型'), dataIndex: 'model' },
    {
      title: t('来源'),
      dataIndex: 'provider',
      render: (provider, record) => (
        <Space spacing={4}>
          <span>{provider}</span>
          {record.first_party ? (
            <Tag color='green' size='small'>
              {t('官方')}
            </Tag>
          ) : (
            <Tag color='grey' size='small'>
              {t('非官方')}
            </Tag>
          )}
          {record.match_type === 'normalized' && (
            <Tag color='blue' size='small'>
              {t('模糊匹配')}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('官方价格'),
      dataIndex: 'official',
      render: (official) => (
        <Text size='small'>{formatRatioSet(official, t)}</Text>
      ),
    },
    {
      title: t('当前价格'),
      dataIndex: 'current',
      render: (current) => (
        <Text size='small' type='tertiary'>
          {formatRatioSet(current, t)}
        </Text>
      ),
    },
    {
      title: t('动作'),
      dataIndex: 'action',
      render: (action) => {
        if (action === 'add')
          return <Tag color='green' size='small'>{t('添加')}</Tag>;
        if (action === 'update')
          return <Tag color='orange' size='small'>{t('更新')}</Tag>;
        return <Tag color='grey' size='small'>{t('无变化')}</Tag>;
      },
    },
  ];

  const rowSelection = {
    selectedRowKeys: selected,
    onChange: (keys) => setSelected(keys),
    getCheckboxProps: (record) => ({ disabled: record.action === 'skip' }),
  };

  const fetchedLabel =
    meta && meta.fetched_at
      ? new Date(meta.fetched_at * 1000).toLocaleString()
      : t('从未刷新');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 状态行 */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 12,
          alignItems: 'center',
          justifyContent: 'space-between',
          border: '1px solid var(--semi-color-border)',
          borderRadius: 8,
          padding: 12,
        }}
      >
        <Space vertical align='start' spacing={2}>
          <Text type='tertiary'>
            {t('来源')}: <Text strong>models.dev</Text> · {t('收录模型')}:{' '}
            <Text strong>{metaLoading ? '…' : meta?.model_count ?? 0}</Text> ·{' '}
            {t('第一方')}:{' '}
            <Text strong>{metaLoading ? '…' : meta?.first_party_count ?? 0}</Text>
          </Text>
          <Text type='tertiary'>
            {t('最后刷新')}: <Text strong>{fetchedLabel}</Text>
          </Text>
        </Space>
        <Button
          theme='light'
          type='primary'
          icon={<RefreshCcw size={16} />}
          loading={refreshing}
          disabled={isBusy}
          onClick={handleRefresh}
        >
          {t('刷新手册')}
        </Button>
      </div>

      {/* 控制区 */}
      <Space wrap align='end' spacing={12}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <Text type='tertiary' size='small'>
            {t('范围')}
          </Text>
          <Select
            value={scopeKind}
            onChange={setScopeKind}
            style={{ width: 180 }}
            optionList={[
              { label: t('所有未定价模型'), value: 'all_missing' },
              { label: t('手动输入模型'), value: 'models' },
            ]}
          />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <Text type='tertiary' size='small'>
            {t('模式')}
          </Text>
          <Select
            value={mode}
            onChange={setMode}
            style={{ width: 180 }}
            optionList={[
              { label: t('仅补缺'), value: 'missing_only' },
              { label: t('刷新到最新官方价'), value: 'refresh_latest' },
            ]}
          />
        </div>
        <Button
          theme='solid'
          type='primary'
          loading={previewing}
          disabled={isBusy}
          onClick={handlePreview}
        >
          {t('预览')}
        </Button>
      </Space>

      {scopeKind === 'models' && (
        <TextArea
          value={manualModels}
          onChange={setManualModels}
          rows={3}
          placeholder={t('每行一个模型名，或用逗号分隔')}
        />
      )}

      {/* 预览结果 */}
      {preview && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 8,
            }}
          >
            <Text type='tertiary'>
              {t('已命中')}: {preview.rows.length} · {t('未匹配')}:{' '}
              {preview.unmatched.length} · {t('已选')}: {selected.length}
            </Text>
            <Button
              theme='solid'
              type='primary'
              icon={<CheckSquare size={16} />}
              loading={applying}
              disabled={isBusy || selected.length === 0}
              onClick={handleApply}
            >
              {t('应用所选')} ({selected.length})
            </Button>
          </div>

          {preview.rows.length > 0 && (
            <Table
              rowKey='model'
              columns={columns}
              dataSource={preview.rows}
              rowSelection={rowSelection}
              pagination={false}
              size='small'
            />
          )}

          {preview.unmatched.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <Text type='tertiary'>
                {t('未匹配（无官方价，需人工处理）')}:
              </Text>
              <Space wrap spacing={4}>
                {preview.unmatched.map((m) => (
                  <Tag key={m} color='grey' size='small'>
                    {m}
                  </Tag>
                ))}
              </Space>
            </div>
          )}
        </div>
      )}

      {(refreshing || previewing) && !preview && (
        <div style={{ textAlign: 'center', padding: 16 }}>
          <Spin />
        </div>
      )}
    </div>
  );
}
