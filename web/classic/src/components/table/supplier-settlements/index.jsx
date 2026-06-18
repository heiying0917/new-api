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

import React, { useCallback, useState } from 'react';
import {
  Button,
  Modal,
  Typography,
  Select,
  DatePicker,
  Card,
  Tooltip,
} from '@douyinfe/semi-ui';
import { IconPlus } from '@douyinfe/semi-icons';
import { Wallet, Info } from 'lucide-react';
import CardPro from '../../common/ui/CardPro';
import SupplierSettlementsTable from './SupplierSettlementsTable';
import SettlementDetailModal from './modals/SettlementDetailModal';
import { useSupplierSettlementsData } from '../../../hooks/supplier-settlements/useSupplierSettlementsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const { Title, Text } = Typography;

// 安全两位小数 + 千分位
const fixed2 = (num) => {
  const n = Number(num);
  if (!Number.isFinite(n)) return '0.00';
  return n.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
};

function SupplierSettlementsPage() {
  const data = useSupplierSettlementsData();
  const isMobile = useIsMobile();
  const {
    settlements,
    loading,
    applying,
    activePage,
    pageSize,
    total,
    pending,
    statusFilter,
    applySettlement,
    cancelSettlement,
    getDetail,
    getPendingAmount,
    getBreakdown,
    getSettlementLogs,
    exportSettlement,
    handlePageChange,
    handlePageSizeChange,
    handleStatusChange,
    handleDateRangeChange,
    t,
  } = data;

  const [showDetail, setShowDetail] = useState(false);
  const [detailId, setDetailId] = useState(null);

  const handleDetail = useCallback((record) => {
    setDetailId(record.id);
    setShowDetail(true);
  }, []);

  const closeDetail = useCallback(() => {
    setShowDetail(false);
    setTimeout(() => setDetailId(null), 300);
  }, []);

  const handleCancel = useCallback(
    (id) => {
      cancelSettlement(id);
    },
    [cancelSettlement],
  );

  const handleExport = useCallback(
    (id) => {
      exportSettlement(id);
    },
    [exportSettlement],
  );

  const handleApply = useCallback(async () => {
    const pending = await getPendingAmount();
    if (!pending) {
      return;
    }

    const payableCny = Number(pending.payable_cny || 0);
    const officialUsd = Number(pending.official_usd || 0);
    const logCount = Number(pending.log_count || 0);
    const nothingToSettle = logCount <= 0 || payableCny <= 0;

    Modal.confirm({
      title: t('确定申请结算？'),
      content: (
        <div className='flex flex-col gap-2'>
          {nothingToSettle ? (
            <Text type='tertiary'>{t('当前没有可结算的消费')}</Text>
          ) : (
            <>
              <div>
                <Text strong style={{ fontSize: 18 }}>
                  {t('当前应结算金额：')}
                </Text>
                <Text strong type='success' style={{ fontSize: 22 }}>
                  ¥{payableCny.toFixed(2)}
                </Text>
              </div>
              <Text type='tertiary' size='small'>
                {t('官方价')} ${officialUsd.toFixed(2)} · {t('共')} {logCount}{' '}
                {t('条待结算')}
              </Text>
            </>
          )}
          <Text type='tertiary' size='small'>
            {t(
              '申请结算将把当前所有待结算的用量快照为一笔新的待审核结算，提交后当前待结算计费将清零。',
            )}
          </Text>
        </div>
      ),
      okButtonProps: { disabled: nothingToSettle },
      onOk: () => {
        if (nothingToSettle) {
          return;
        }
        applySettlement();
      },
    });
  }, [applySettlement, getPendingAmount, t]);

  const payableCny = Number(pending?.payable_cny || 0);
  const officialUsd = Number(pending?.official_usd || 0);
  const logCount = Number(pending?.log_count || 0);

  return (
    <>
      <SettlementDetailModal
        visible={showDetail}
        settlementId={detailId}
        handleClose={closeDetail}
        getDetail={getDetail}
        getBreakdown={getBreakdown}
        getSettlementLogs={getSettlementLogs}
      />

      {/* 待结算汇总：应收金额 + 申请结算按钮 + 官方价/笔数（顶部清晰展示当前待结算账单） */}
      <Card
        className='!rounded-2xl mb-4'
        bordered={false}
        bodyStyle={{ padding: 20 }}
        style={{
          border: '1px solid var(--semi-color-border)',
          background: 'var(--semi-color-bg-1)',
        }}
      >
        <div className='flex flex-wrap items-center justify-between gap-4'>
          {/* 左：应收金额 + 申请结算 */}
          <div className='flex items-center gap-4'>
            <div
              className='flex items-center justify-center shrink-0'
              style={{
                width: 48,
                height: 48,
                borderRadius: 14,
                background: 'rgba(16, 185, 129, 0.14)',
                color: '#10b981',
                boxShadow: 'inset 0 0 0 1px rgba(16, 185, 129, 0.45)',
              }}
            >
              <Wallet size={24} strokeWidth={2.2} />
            </div>
            <div>
              <div className='flex items-center gap-1'>
                <Text
                  type='tertiary'
                  style={{
                    fontSize: 12,
                    fontWeight: 600,
                    letterSpacing: '0.04em',
                  }}
                >
                  {t('待结算金额（应收 ¥）')}
                </Text>
                <Tooltip
                  position='top'
                  content={
                    <div style={{ lineHeight: 1.8, maxWidth: 260 }}>
                      <div>
                        <b>{t('应收')} (¥)</b>: 
                        {t('按成本价×用量折算的人民币，即你将收到的金额')}
                      </div>
                      <div>
                        <b>{t('官方价')} ($)</b>: {t('上游官方计费的美元金额')}
                      </div>
                      <div>
                        <b>{t('笔数')}</b>: {t('当前待结算的调用条数')}
                      </div>
                    </div>
                  }
                >
                  <Info
                    size={12}
                    style={{ color: 'var(--semi-color-text-2)', cursor: 'help' }}
                  />
                </Tooltip>
              </div>
              <div className='flex items-baseline gap-3 mt-1 flex-wrap'>
                <span
                  style={{
                    fontSize: 30,
                    fontWeight: 700,
                    color:
                      payableCny > 0
                        ? 'var(--semi-color-success)'
                        : 'var(--semi-color-text-0)',
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  ¥{fixed2(payableCny)}
                </span>
                <Button
                  type='primary'
                  theme='solid'
                  icon={<IconPlus />}
                  onClick={handleApply}
                  loading={applying}
                >
                  {t('申请结算')}
                </Button>
              </div>
            </div>
          </div>
          {/* 右：官方价 + 笔数 */}
          <div className='flex items-center gap-8'>
            <div>
              <Text type='tertiary' style={{ fontSize: 12 }}>
                {t('官方价')} ($)
              </Text>
              <div
                style={{
                  fontSize: 20,
                  fontWeight: 600,
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                ${fixed2(officialUsd)}
              </div>
            </div>
            <div>
              <Text type='tertiary' style={{ fontSize: 12 }}>
                {t('待结算笔数')}
              </Text>
              <div
                style={{
                  fontSize: 20,
                  fontWeight: 600,
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {logCount} {t('笔')}
              </div>
            </div>
          </div>
        </div>
        <Text
          type='tertiary'
          style={{ fontSize: 12, marginTop: 12, display: 'block' }}
        >
          {t(
            '应收 = 成本价 × 用量；申请结算会把当前待结算用量快照为一笔待审核结算，提交后待结算清零。',
          )}
        </Text>
      </Card>

      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex items-center'>
            <Title heading={5} className='m-0'>
              {t('结算记录')}
            </Title>
          </div>
        }
        actionsArea={
          <div className='flex flex-wrap items-center gap-2 w-full'>
            <Select
              value={statusFilter}
              onChange={handleStatusChange}
              size='small'
              style={{ width: 120 }}
              optionList={[
                { label: t('全部'), value: 0 },
                { label: t('已完成'), value: 2 },
                { label: t('已取消'), value: 3 },
              ]}
            />
            <DatePicker
              type='dateRange'
              density='compact'
              size='small'
              style={{ width: 240 }}
              placeholder={t('按申请时间段筛选')}
              onChange={(date) => handleDateRangeChange(date)}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize: pageSize,
          total: total,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile: isMobile,
          t: t,
        })}
        t={t}
      >
        <SupplierSettlementsTable
          settlements={settlements}
          loading={loading}
          activePage={activePage}
          pageSize={pageSize}
          total={total}
          handlePageChange={handlePageChange}
          handlePageSizeChange={handlePageSizeChange}
          onDetail={handleDetail}
          onCancel={handleCancel}
          onExport={handleExport}
          t={t}
        />
      </CardPro>
    </>
  );
}

export default SupplierSettlementsPage;
