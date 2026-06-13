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
import { useTranslation } from 'react-i18next'
import { Download } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
  SheetClose,
} from '@/components/ui/sheet'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
  sideDrawerFormClassName,
  sideDrawerFooterClassName,
  SideDrawerSection,
} from '@/components/drawer-layout'
import { formatTimestampToDate } from '@/lib/format'
import { getSettlementLogs, settlementExportUrl } from '../api'
import { type Settlement, SettlementStatus } from '../types'

const STATUS_LABEL: Record<number, string> = {
  [SettlementStatus.Applied]: 'Applied',
  [SettlementStatus.Settled]: 'Settled',
  [SettlementStatus.Cancelled]: 'Cancelled',
}

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  settlement: Settlement | null
}

export function SettlementDetailDrawer({
  open,
  onOpenChange,
  settlement,
}: Props) {
  const { t } = useTranslation()
  const settlementId = settlement?.id

  const { data: logsData } = useQuery({
    queryKey: ['settlement-logs', settlementId],
    queryFn: async () => {
      if (!settlementId) return { items: [], total: 0 }
      const res = await getSettlementLogs(settlementId, {
        p: 1,
        page_size: 20,
      })
      return { items: res.data?.items || [], total: res.data?.total || 0 }
    },
    enabled: open && !!settlementId,
  })

  if (!settlement) return null

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t('Settlement')} #{settlement.id}
          </SheetTitle>
          <SheetDescription>
            {t('Period')}:{' '}
            {formatTimestampToDate(settlement.period_start)} ~{' '}
            {formatTimestampToDate(settlement.period_end)}
          </SheetDescription>
        </SheetHeader>
        <div className={sideDrawerFormClassName()}>
          <SideDrawerSection>
            <dl className='grid grid-cols-2 gap-3 text-sm'>
              <div>
                <dt className='text-muted-foreground'>{t('Status')}</dt>
                <dd className='font-medium'>
                  {t(STATUS_LABEL[settlement.status] ?? '')}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>
                  {t('Official Amount')}
                </dt>
                <dd className='font-medium'>
                  ${settlement.official_usd?.toFixed(6)}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Payable')}</dt>
                <dd className='font-medium'>
                  ¥{settlement.computed_cny?.toFixed(2)}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Actual Amount')}</dt>
                <dd className='font-medium'>
                  {settlement.actual_amount
                    ? `${settlement.actual_amount} ${settlement.actual_currency}`
                    : '-'}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Settle Method')}</dt>
                <dd className='font-medium'>
                  {settlement.settle_method || '-'}
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Source')}</dt>
                <dd className='font-medium'>{settlement.source || '-'}</dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Log Count')}</dt>
                <dd className='font-medium'>{settlement.log_count}</dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Created At')}</dt>
                <dd className='font-medium'>
                  {formatTimestampToDate(settlement.created_at)}
                </dd>
              </div>
              {settlement.settled_at > 0 && (
                <div>
                  <dt className='text-muted-foreground'>{t('Settled At')}</dt>
                  <dd className='font-medium'>
                    {formatTimestampToDate(settlement.settled_at)}
                  </dd>
                </div>
              )}
              {settlement.remark && (
                <div className='col-span-2'>
                  <dt className='text-muted-foreground'>{t('Remark')}</dt>
                  <dd className='font-medium'>{settlement.remark}</dd>
                </div>
              )}
            </dl>
          </SideDrawerSection>
          {(logsData?.items?.length ?? 0) > 0 && (
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>
                {t('Usage Logs')} ({logsData?.total})
              </h3>
              <div className='text-muted-foreground text-xs'>
                {t('Showing first 20 records')}
              </div>
            </SideDrawerSection>
          )}
        </div>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <a
            href={settlementExportUrl(settlement.id)}
            target='_blank'
            rel='noreferrer'
          >
            <Button variant='default'>
              <Download className='mr-2 h-4 w-4' />
              {t('Export')}
            </Button>
          </a>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
