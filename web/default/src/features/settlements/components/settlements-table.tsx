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
import { getSettlements } from '../api'
import { useSettlementsColumns } from './settlements-columns'
import { useSettlements } from './settlements-provider'

const route = getRouteApi('/_authenticated/settlements/')

export function SettlementsTable() {
  const { t } = useTranslation()
  const columns = useSettlementsColumns()
  const { refreshTrigger } = useSettlements()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: route.useSearch(),
      navigate: route.useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'settlements',
      pagination.pageIndex + 1,
      pagination.pageSize,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getSettlements({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      if (!result.success) {
        toast.error(result.message || 'Failed to load settlements')
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (prev) => prev,
  })

  const settlements = data?.items || []

  const { table } = useDataTable({
    data: settlements,
    columns,
    getRowId: (row) => String(row.id),
    pagination,
    onPaginationChange,
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
      emptyTitle={t('No Settlements Found')}
      emptyDescription={t('No settlements available.')}
      skeletonKeyPrefix='settlements-skeleton'
      applyHeaderSize
    />
  )
}
