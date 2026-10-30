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

import React from 'react';
import { Modal, Typography } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import SuppliersTable from './SuppliersTable';
import SuppliersFilters from './SuppliersFilters';
import SuppliersDescription from './SuppliersDescription';
import SuppliersSummaryBar from './SuppliersSummaryBar';
import EditSupplierModal from './modals/EditSupplierModal';
import ConfirmModal from '../settlement-review/modals/ConfirmModal';
import { useSuppliersData } from '../../../hooks/suppliers/useSuppliersData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const { Text } = Typography;

const SuppliersPage = () => {
  const suppliersData = useSuppliersData();
  const isMobile = useIsMobile();

  const {
    // Modal state
    showEditSupplier,
    editingSupplier,
    closeEditSupplier,
    refresh,
    updateSupplier,

    // Form state
    formInitValues,
    setFormApi,
    searchSuppliers,
    loadSuppliers,
    pageSize,
    loading,
    searching,

    // Summary + confirm
    summary,
    initiateSettlement,
    showConfirm,
    confirmRecord,
    confirmSettlement,
    closeConfirm,

    // Description state
    compactMode,
    setCompactMode,

    // Translation
    t,
  } = suppliersData;

  // 立即结算：先二次确认，再生成结算单。onOk 返回 Promise，生成期间确定按钮 loading，
  // 防止大账单打包较慢时重复点击生成多张结算单。
  const handleInitiateSettlement = (record) => {
    const pendingCny = Number(record.pending_cny || 0);
    const pendingUsd = Number(record.pending_usd || 0);
    const nothingToSettle = pendingCny <= 0;

    Modal.confirm({
      title: t('确定为该供应商发起结算？'),
      content: (
        <div className='flex flex-col gap-2'>
          <Text>
            {t('供应商')}: <Text strong>{record.username}</Text>
          </Text>
          {nothingToSettle ? (
            <Text type='tertiary'>{t('当前没有可结算的消费')}</Text>
          ) : (
            <>
              <div>
                <Text strong>{t('当前应结算金额：')}</Text>
                <Text strong type='success' style={{ fontSize: 18 }}>
                  ¥{pendingCny.toFixed(2)}
                </Text>
              </div>
              <Text type='tertiary' size='small'>
                {t('官方价')} ${pendingUsd.toFixed(2)}
              </Text>
            </>
          )}
          <Text type='tertiary' size='small'>
            {t(
              '将把该供应商当前所有待结算消费打包为一张待审核结算单（金额以生成时为准），待结算随即清零；生成后可立即登记打款，也可稍后在「结算审核」中处理。',
            )}
          </Text>
        </div>
      ),
      okButtonProps: { disabled: nothingToSettle },
      onOk: () => (nothingToSettle ? undefined : initiateSettlement(record)),
    });
  };

  const confirmDescription = confirmRecord ? (
    <div className='flex flex-col gap-1'>
      <Text>
        {t(
          '结算单 #${id} 已生成（待审核）：应付 ¥${amount}，共 ${count} 条消费。',
        )
          .replace('${id}', confirmRecord.id)
          .replace(
            '${amount}',
            Number(confirmRecord.computed_cny || 0).toFixed(2),
          )
          .replace('${count}', confirmRecord.log_count || 0)}
      </Text>
      <Text type='tertiary' size='small'>
        {t(
          '确定即完成结算；直接关闭不会撤销该结算单，可稍后在「结算审核」中确认或取消。',
        )}
      </Text>
    </div>
  ) : null;

  return (
    <>
      <EditSupplierModal
        visible={showEditSupplier}
        handleClose={closeEditSupplier}
        refresh={refresh}
        updateSupplier={updateSupplier}
        editingSupplier={editingSupplier}
      />

      <ConfirmModal
        visible={showConfirm}
        record={confirmRecord}
        onCancel={closeConfirm}
        confirmSettlement={confirmSettlement}
        description={confirmDescription}
        t={t}
      />

      <SuppliersSummaryBar summary={summary} t={t} />

      <CardPro
        type='type1'
        descriptionArea={
          <SuppliersDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-end items-center gap-2 w-full'>
            <SuppliersFilters
              formInitValues={formInitValues}
              setFormApi={setFormApi}
              searchSuppliers={searchSuppliers}
              loadSuppliers={loadSuppliers}
              pageSize={pageSize}
              loading={loading}
              searching={searching}
              t={t}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: suppliersData.activePage,
          pageSize: suppliersData.pageSize,
          total: suppliersData.supplierCount,
          onPageChange: suppliersData.handlePageChange,
          onPageSizeChange: suppliersData.handlePageSizeChange,
          isMobile: isMobile,
          t: suppliersData.t,
        })}
        t={suppliersData.t}
      >
        <SuppliersTable
          {...suppliersData}
          onInitiateSettlement={handleInitiateSettlement}
        />
      </CardPro>
    </>
  );
};

export default SuppliersPage;
