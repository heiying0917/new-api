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
import { getMyChannels } from '../api'
import { useMyChannelsColumns } from './my-channels-columns'
import { useMyChannels } from './my-channels-provider'

const route = getRouteApi('/_authenticated/my-channels/')

export function MyChannelsTable() {
  const { t } = useTranslation()
  const columns = useMyChannelsColumns()
  const { refreshTrigger } = useMyChannels()
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
      'my-channels',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      refreshTrigger,
    ],
    queryFn: async () => {
      const params = {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        keyword: globalFilter?.trim() || '',
      }

      const result = await getMyChannels(params)

      if (!result.success) {
        toast.error(result.message || 'Failed to load channels')
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const channels = data?.items || []

  const { table } = useDataTable({
    data: channels,
    columns,
    getRowId: (row) => String(row.id),
    globalFilter,
    pagination,
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
      emptyTitle={t('No Channels Found')}
      emptyDescription={t('No channels available. Try adjusting your search.')}
      skeletonKeyPrefix='my-channels-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by channel name...'),
      }}
    />
  )
}
