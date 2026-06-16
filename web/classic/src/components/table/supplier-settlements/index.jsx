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
import { Button, Modal, Typography } from '@douyinfe/semi-ui';
import { IconPlus } from '@douyinfe/semi-icons';
import CardPro from '../../common/ui/CardPro';
import SupplierSettlementsTable from './SupplierSettlementsTable';
import SettlementDetailModal from './modals/SettlementDetailModal';
import { useSupplierSettlementsData } from '../../../hooks/supplier-settlements/useSupplierSettlementsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const { Title, Text } = Typography;

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
    applySettlement,
    cancelSettlement,
    getDetail,
    getPendingAmount,
    getBreakdown,
    getSettlementLogs,
    exportSettlement,
    handlePageChange,
    handlePageSizeChange,
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

      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex items-center'>
            <Title heading={5} className='m-0'>
              {t('账单结算')}
            </Title>
          </div>
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <div className='flex flex-wrap gap-2 w-full md:w-auto'>
              <Button
                type='primary'
                className='flex-1 md:flex-initial'
                icon={<IconPlus />}
                onClick={handleApply}
                loading={applying}
                size='small'
              >
                {t('申请结算')}
              </Button>
            </div>
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
