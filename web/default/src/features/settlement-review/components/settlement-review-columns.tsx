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
import { Eye, CheckCircle, XCircle, Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { DataTableColumnHeader } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatTimestampToDate } from '@/lib/format'
import { type Settlement, SettlementStatus } from '../types'
import { exportUrl } from '../api'
import { useSettlementReview } from './settlement-review-provider'

const STATUS_CONFIG: Record<number, { label: string; variant: 'warning' | 'success' | 'neutral' }> = {
  [SettlementStatus.Applied]: { label: 'Applied', variant: 'warning' },
  [SettlementStatus.Settled]: { label: 'Settled', variant: 'success' },
  [SettlementStatus.Cancelled]: { label: 'Cancelled', variant: 'neutral' },
}

export function useSettlementReviewColumns(): ColumnDef<Settlement>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useSettlementReview()

  return [
    {
      accessorKey: 'id',
      header: ({ column }) => <DataTableColumnHeader column={column} title='ID' />,
      cell: ({ row }) => <TableId value={row.getValue('id') as number} className='w-[60px]' />,
      size: 80,
      meta: { label: t('ID'), mobileHidden: true },
    },
    {
      accessorKey: 'supplier_id',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Supplier ID')} />,
      cell: ({ row }) => <span className='text-muted-foreground text-sm'>{row.getValue('supplier_id') as number}</span>,
      size: 110,
      meta: { label: t('Supplier ID'), mobileHidden: true },
    },
    {
      accessorKey: 'period_start',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Period')} />,
      cell: ({ row }) => {
        const s = row.original
        return (
          <span className='text-muted-foreground text-sm'>
            {formatTimestampToDate(s.period_start)} ~ {formatTimestampToDate(s.period_end)}
          </span>
        )
      },
      enableSorting: false,
      size: 280,
      meta: { label: t('Period'), mobileHidden: true },
    },
    {
      accessorKey: 'official_usd',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Official Amount')} />,
      cell: ({ row }) => <span className='text-sm'>${(row.getValue('official_usd') as number)?.toFixed(6)}</span>,
      size: 140,
      meta: { label: t('Official Amount') },
    },
    {
      accessorKey: 'computed_cny',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Payable')} />,
      cell: ({ row }) => <span className='text-sm'>¥{(row.getValue('computed_cny') as number)?.toFixed(2)}</span>,
      size: 120,
      meta: { label: t('Payable') },
    },
    {
      id: 'actual',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Actual Amount')} />,
      cell: ({ row }) => {
        const s = row.original
        return <span className='text-sm'>{s.actual_amount ? `${s.actual_amount} ${s.actual_currency}` : '-'}</span>
      },
      enableSorting: false,
      size: 140,
      meta: { label: t('Actual Amount'), mobileHidden: true },
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Status')} />,
      cell: ({ row }) => {
        const status = row.getValue('status') as number
        const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG[SettlementStatus.Applied]
        return <StatusBadge label={t(cfg.label)} variant={cfg.variant} copyable={false} />
      },
      size: 110,
      meta: { label: t('Status'), mobileBadge: true },
    },
    {
      accessorKey: 'source',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Source')} />,
      cell: ({ row }) => <LongText className='text-muted-foreground max-w-[120px] text-sm'>{(row.getValue('source') as string) || '-'}</LongText>,
      enableSorting: false,
      size: 140,
      meta: { label: t('Source'), mobileHidden: true },
    },
    {
      accessorKey: 'created_at',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('Created At')} />,
      cell: ({ row }) => <span className='text-muted-foreground text-sm'>{formatTimestampToDate(row.getValue('created_at') as number)}</span>,
      size: 170,
      meta: { label: t('Created At'), mobileHidden: true },
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const s = row.original
        return (
          <div className='flex items-center gap-1'>
            <Button variant='ghost' size='sm' onClick={() => { setCurrentRow(s); setOpen('detail') }}>
              <Eye className='mr-1 h-4 w-4' />{t('Detail')}
            </Button>
            {s.status === SettlementStatus.Applied && (
              <>
                <Button variant='ghost' size='sm' onClick={() => { setCurrentRow(s); setOpen('confirm') }}>
                  <CheckCircle className='mr-1 h-4 w-4' />{t('Confirm Settlement')}
                </Button>
                <Button variant='ghost' size='sm' className='text-destructive hover:text-destructive' onClick={() => { setCurrentRow(s); setOpen('cancel') }}>
                  <XCircle className='mr-1 h-4 w-4' />{t('Cancel Settlement')}
                </Button>
              </>
            )}
            <a href={exportUrl(s.id)} target='_blank' rel='noreferrer'>
              <Button variant='ghost' size='sm'>
                <Download className='mr-1 h-4 w-4' />{t('Export')}
              </Button>
            </a>
          </div>
        )
      },
      meta: { label: t('Actions') },
    },
  ]
}
