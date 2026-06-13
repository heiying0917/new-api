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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { getSuppliers, searchSuppliers } from '../api'
import { useSuppliersColumns } from './suppliers-columns'
import { useSuppliers } from './suppliers-provider'

const route = getRouteApi('/_authenticated/suppliers/')

export function SuppliersTable() {
  const { t } = useTranslation()
  const columns = useSuppliersColumns()
  const { refreshTrigger } = useSuppliers()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const {
    globalFilter,
    onGlobalFilterChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'suppliers',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      refreshTrigger,
    ],
    queryFn: async () => {
      const hasFilter = globalFilter?.trim()
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      }

      const result = hasFilter
        ? await searchSuppliers({ ...params, keyword: globalFilter })
        : await getSuppliers(params)

      if (!result.success) {
        toast.error(
          result.message ||
            `Failed to ${hasFilter ? 'search' : 'load'} suppliers`
        )
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const suppliers = data?.items || []

  const { table } = useDataTable({
    data: suppliers,
    columns,
    getRowId: (row) => String(row.user_id),
    globalFilter,
    pagination,
    globalFilterFn: (row, _columnId, filterValue) => {
      const searchValue = String(filterValue).toLowerCase()
      const fields = [
        row.getValue('username'),
        row.original.email,
        row.original.phone,
      ]
      return fields.some((field) =>
        String(field || '')
          .toLowerCase()
          .includes(searchValue)
      )
    },
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Suppliers Found')}
      emptyDescription={t(
        'No suppliers available. Try adjusting your search.'
      )}
      skeletonKeyPrefix='suppliers-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by username, email or phone...'),
      }}
    />
  )
}
