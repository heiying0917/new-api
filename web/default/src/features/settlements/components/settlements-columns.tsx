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
import { type ColumnDef } from '@tanstack/react-table'
import { Eye, XCircle, Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { DataTableColumnHeader } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatTimestampToDate } from '@/lib/format'
import { SettlementStatus, type Settlement } from '../types'
import { settlementExportUrl } from '../api'
import { useSettlements } from './settlements-provider'

export function useSettlementsColumns(): ColumnDef<Settlement>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useSettlements()

  return [
    {
      accessorKey: 'id',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title='ID' />
      ),
      cell: ({ row }) => (
        <TableId value={row.getValue('id') as number} className='w-[60px]' />
      ),
      size: 80,
      meta: { label: t('ID'), mobileHidden: true },
    },
    {
      accessorKey: 'period_start',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Period')} />
      ),
      cell: ({ row }) => {
        const s = row.original
        return (
          <span className='text-sm'>
            {formatTimestampToDate(s.period_start, 'seconds')} ~{' '}
            {formatTimestampToDate(s.period_end, 'seconds')}
          </span>
        )
      },
      enableSorting: false,
      size: 280,
      meta: { label: t('Period') },
    },
    {
      accessorKey: 'official_usd',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Official USD')} />
      ),
      cell: ({ row }) => {
        const val = row.getValue('official_usd') as number
        return <span className='text-sm'>${val?.toFixed(6)}</span>
      },
      size: 140,
      meta: { label: t('Official USD'), mobileHidden: true },
    },
    {
      accessorKey: 'computed_cny',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Payable CNY')} />
      ),
      cell: ({ row }) => {
        const val = row.getValue('computed_cny') as number
        return <span className='text-sm'>¥{val?.toFixed(2)}</span>
      },
      size: 120,
      meta: { label: t('Payable CNY'), mobileHidden: true },
    },
    {
      id: 'actual',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Actual')} />
      ),
      cell: ({ row }) => {
        const s = row.original
        return (
          <span className='text-sm'>
            {s.actual_amount ? `${s.actual_amount} ${s.actual_currency}` : '-'}
          </span>
        )
      },
      enableSorting: false,
      size: 140,
      meta: { label: t('Actual'), mobileHidden: true },
    },
    {
      accessorKey: 'status',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as SettlementStatus
        const config: Record<
          number,
          { label: string; variant: 'warning' | 'success' | 'neutral' }
        > = {
          [SettlementStatus.Applied]: {
            label: t('Applied'),
            variant: 'warning',
          },
          [SettlementStatus.Settled]: {
            label: t('Settled'),
            variant: 'success',
          },
          [SettlementStatus.Cancelled]: {
            label: t('Cancelled'),
            variant: 'neutral',
          },
        }
        const c = config[status] ?? { label: String(status), variant: 'neutral' as const }
        return (
          <StatusBadge label={c.label} variant={c.variant} copyable={false} />
        )
      },
      size: 110,
      meta: { label: t('Status'), mobileBadge: true },
    },
    {
      accessorKey: 'source',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Source')} />
      ),
      cell: ({ row }) => {
        const source = row.getValue('source') as string
        return (
          <LongText className='text-muted-foreground max-w-[120px] text-sm'>
            {source || '-'}
          </LongText>
        )
      },
      enableSorting: false,
      size: 140,
      meta: { label: t('Source'), mobileHidden: true },
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const s = row.original
        return (
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='sm'
              onClick={() => {
                setCurrentRow(s)
                setOpen('detail')
              }}
            >
              <Eye className='mr-1 h-4 w-4' />
              {t('Detail')}
            </Button>
            {s.status === SettlementStatus.Applied && (
              <Button
                variant='ghost'
                size='sm'
                className='text-destructive hover:text-destructive'
                onClick={() => {
                  setCurrentRow(s)
                  setOpen('cancel')
                }}
              >
                <XCircle className='mr-1 h-4 w-4' />
                {t('Cancel Settlement')}
              </Button>
            )}
            <a
              href={settlementExportUrl(s.id)}
              target='_blank'
              rel='noreferrer'
            >
              <Button variant='ghost' size='sm'>
                <Download className='mr-1 h-4 w-4' />
                {t('Export')}
              </Button>
            </a>
          </div>
        )
      },
      meta: { label: t('Actions') },
    },
  ]
}
