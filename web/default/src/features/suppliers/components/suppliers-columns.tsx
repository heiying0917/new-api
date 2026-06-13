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
import { Pencil } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { DataTableColumnHeader } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  getSettlementModeLabel,
  getSettlementCycleLabel,
} from '../constants'
import { type Supplier } from '../types'
import { useSuppliers } from './suppliers-provider'

export function useSuppliersColumns(): ColumnDef<Supplier>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useSuppliers()

  return [
    {
      accessorKey: 'user_id',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title='ID' />
      ),
      cell: ({ row }) => (
        <TableId
          value={row.getValue('user_id') as number}
          className='w-[60px]'
        />
      ),
      size: 80,
      meta: { label: t('ID'), mobileHidden: true },
    },
    {
      accessorKey: 'username',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Username')} />
      ),
      cell: ({ row }) => {
        const username = row.getValue('username') as string
        return (
          <LongText className='max-w-[160px] font-medium'>{username}</LongText>
        )
      },
      enableHiding: false,
      size: 180,
      meta: { label: t('Username'), mobileTitle: true },
    },
    {
      accessorKey: 'email',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Email')} />
      ),
      cell: ({ row }) => {
        const email = row.getValue('email') as string
        return (
          <LongText className='text-muted-foreground max-w-[200px] text-sm'>
            {email || '-'}
          </LongText>
        )
      },
      size: 220,
      meta: { label: t('Email'), mobileHidden: true },
    },
    {
      accessorKey: 'phone',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Phone')} />
      ),
      cell: ({ row }) => {
        const phone = row.getValue('phone') as string
        return (
          <span className='text-muted-foreground text-sm'>{phone || '-'}</span>
        )
      },
      size: 140,
      meta: { label: t('Phone'), mobileHidden: true },
    },
    {
      accessorKey: 'priority',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Priority')} />
      ),
      cell: ({ row }) => {
        const priority = row.getValue('priority') as number
        return (
          <StatusBadge
            label={String(priority)}
            variant='neutral'
            copyable={false}
          />
        )
      },
      size: 100,
      meta: { label: t('Priority') },
    },
    {
      accessorKey: 'enabled',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Enabled')} />
      ),
      cell: ({ row }) => {
        const enabled = row.getValue('enabled') as boolean
        return (
          <StatusBadge
            label={enabled ? t('Enabled') : t('Disabled')}
            variant={enabled ? 'success' : 'neutral'}
            copyable={false}
          />
        )
      },
      size: 110,
      meta: { label: t('Enabled'), mobileBadge: true },
    },
    {
      id: 'settlement',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Settlement')} />
      ),
      cell: ({ row }) => {
        const supplier = row.original
        const modeLabel = getSettlementModeLabel(supplier.settlement_mode, t)
        const cycleLabel = getSettlementCycleLabel(
          supplier.settlement_cycle,
          t
        )
        return (
          <span className='text-muted-foreground text-sm'>
            {modeLabel} / {cycleLabel}
          </span>
        )
      },
      enableSorting: false,
      size: 160,
      meta: { label: t('Settlement'), mobileHidden: true },
    },
    {
      accessorKey: 'remark',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Remark')} />
      ),
      cell: ({ row }) => {
        const remark = row.getValue('remark') as string
        return (
          <LongText className='text-muted-foreground max-w-[160px] text-sm'>
            {remark || '-'}
          </LongText>
        )
      },
      enableSorting: false,
      size: 180,
      meta: { label: t('Remark'), mobileHidden: true },
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const supplier = row.original
        const handleEdit = () => {
          setCurrentRow(supplier)
          setOpen('update')
        }
        return (
          <Button variant='ghost' size='sm' onClick={handleEdit}>
            <Pencil className='mr-1 h-4 w-4' />
            {t('Edit')}
          </Button>
        )
      },
      meta: { label: t('Actions') },
    },
  ]
}
