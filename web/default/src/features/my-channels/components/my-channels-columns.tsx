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
import { Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { DataTableColumnHeader } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { getChannelTypeLabel, MY_CHANNEL_STATUS_CONFIG } from '../constants'
import { type MyChannel } from '../types'
import { useMyChannels } from './my-channels-provider'

export function useMyChannelsColumns(): ColumnDef<MyChannel>[] {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useMyChannels()

  return [
    {
      accessorKey: 'id',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title='ID' />
      ),
      cell: ({ row }) => (
        <TableId
          value={row.getValue('id') as number}
          className='w-[60px]'
        />
      ),
      size: 80,
      meta: { label: t('ID'), mobileHidden: true },
    },
    {
      accessorKey: 'name',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Name')} />
      ),
      cell: ({ row }) => {
        const name = row.getValue('name') as string
        return (
          <LongText className='max-w-[160px] font-medium'>{name}</LongText>
        )
      },
      enableHiding: false,
      size: 180,
      meta: { label: t('Name'), mobileTitle: true },
    },
    {
      accessorKey: 'type',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Channel Type')} />
      ),
      cell: ({ row }) => {
        const type = row.getValue('type') as number
        return (
          <span className='text-muted-foreground text-sm'>
            {getChannelTypeLabel(type)}
          </span>
        )
      },
      size: 130,
      meta: { label: t('Channel Type'), mobileHidden: true },
    },
    {
      accessorKey: 'group',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Group')} />
      ),
      cell: ({ row }) => {
        const group = row.getValue('group') as string
        return (
          <span className='text-muted-foreground text-sm'>{group || '-'}</span>
        )
      },
      size: 100,
      meta: { label: t('Group'), mobileHidden: true },
    },
    {
      accessorKey: 'models',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Models')} />
      ),
      cell: ({ row }) => {
        const models = row.getValue('models') as string
        return (
          <LongText className='text-muted-foreground max-w-[200px] text-sm'>
            {models || '-'}
          </LongText>
        )
      },
      enableSorting: false,
      size: 220,
      meta: { label: t('Models'), mobileHidden: true },
    },
    {
      accessorKey: 'priority',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Priority')} />
      ),
      cell: ({ row }) => {
        const priority = row.getValue('priority') as number | null
        return (
          <span className='text-muted-foreground text-sm'>
            {priority != null ? priority : '-'}
          </span>
        )
      },
      size: 90,
      meta: { label: t('Priority'), mobileHidden: true },
    },
    {
      accessorKey: 'cost_price',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Cost Price')} />
      ),
      cell: ({ row }) => {
        const cost = row.getValue('cost_price') as number | null
        return (
          <span className='text-muted-foreground text-sm'>
            {cost != null ? cost : '-'}
          </span>
        )
      },
      size: 110,
      meta: { label: t('Cost Price'), mobileHidden: true },
    },
    {
      accessorKey: 'status',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as number
        const config =
          MY_CHANNEL_STATUS_CONFIG[status as keyof typeof MY_CHANNEL_STATUS_CONFIG] ??
          MY_CHANNEL_STATUS_CONFIG[0]
        return (
          <StatusBadge
            label={t(config.label)}
            variant={config.variant}
            copyable={false}
          />
        )
      },
      size: 120,
      meta: { label: t('Status'), mobileBadge: true },
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const channel = row.original
        return (
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='sm'
              onClick={() => {
                setCurrentRow(channel)
                setOpen('update')
              }}
            >
              <Pencil className='mr-1 h-4 w-4' />
              {t('Edit')}
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='text-destructive hover:text-destructive'
              onClick={() => {
                setCurrentRow(channel)
                setOpen('delete')
              }}
            >
              <Trash2 className='mr-1 h-4 w-4' />
              {t('Delete')}
            </Button>
          </div>
        )
      },
      meta: { label: 'Actions' },
    },
  ]
}
