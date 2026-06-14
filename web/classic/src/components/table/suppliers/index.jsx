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
import CardPro from '../../common/ui/CardPro';
import SuppliersTable from './SuppliersTable';
import SuppliersFilters from './SuppliersFilters';
import SuppliersDescription from './SuppliersDescription';
import EditSupplierModal from './modals/EditSupplierModal';
import { useSuppliersData } from '../../../hooks/suppliers/useSuppliersData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

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

    // Description state
    compactMode,
    setCompactMode,

    // Translation
    t,
  } = suppliersData;

  return (
    <>
      <EditSupplierModal
        visible={showEditSupplier}
        handleClose={closeEditSupplier}
        refresh={refresh}
        updateSupplier={updateSupplier}
        editingSupplier={editingSupplier}
      />

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
        <SuppliersTable {...suppliersData} />
      </CardPro>
    </>
  );
};

export default SuppliersPage;
