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
import { Typography } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import SettlementReviewTable from './SettlementReviewTable';
import SettlementReviewFilters from './SettlementReviewFilters';
import DetailModal from './modals/DetailModal';
import ConfirmModal from './modals/ConfirmModal';
import { useSettlementReviewData } from '../../../hooks/settlement-review/useSettlementReviewData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const { Title } = Typography;

function SettlementReviewPage() {
  const data = useSettlementReviewData();
  const isMobile = useIsMobile();

  const {
    // List
    settlements,
    loading,
    activePage,
    pageSize,
    total,

    // Filters
    statusFilter,
    supplierId,
    keyword,
    setKeyword,
    changeStatus,
    clearSupplierFilter,
    searchByKeyword,
    resetFilters,

    // Detail
    showDetail,
    detailRecord,
    openDetail,
    closeDetail,

    // Confirm
    showConfirm,
    confirmRecord,
    openConfirm,
    closeConfirm,

    // Actions
    handlePageChange,
    handlePageSizeChange,
    confirmSettlement,
    cancelSettlement,
    exportSettlement,
    fetchBreakdown,
    fetchLogs,

    t,
  } = data;

  return (
    <>
      <DetailModal
        visible={showDetail}
        record={detailRecord}
        onCancel={closeDetail}
        fetchBreakdown={fetchBreakdown}
        fetchLogs={fetchLogs}
        t={t}
      />

      <ConfirmModal
        visible={showConfirm}
        record={confirmRecord}
        onCancel={closeConfirm}
        confirmSettlement={confirmSettlement}
        t={t}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex items-center'>
            <Title heading={5} className='m-0'>
              {t('结算审核')}
            </Title>
          </div>
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <div className='order-2 md:order-1' />
            <SettlementReviewFilters
              statusFilter={statusFilter}
              changeStatus={changeStatus}
              supplierId={supplierId}
              clearSupplierFilter={clearSupplierFilter}
              keyword={keyword}
              setKeyword={setKeyword}
              searchByKeyword={searchByKeyword}
              resetFilters={resetFilters}
              loading={loading}
              t={t}
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
        <SettlementReviewTable
          settlements={settlements}
          loading={loading}
          activePage={activePage}
          pageSize={pageSize}
          total={total}
          handlePageChange={handlePageChange}
          handlePageSizeChange={handlePageSizeChange}
          onDetail={openDetail}
          onConfirm={openConfirm}
          onCancel={cancelSettlement}
          onExport={exportSettlement}
          t={t}
        />
      </CardPro>
    </>
  );
}

export default SettlementReviewPage;
