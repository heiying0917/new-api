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
import { Activity, Gavel, RadioTower, Receipt, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import {
  CardStaggerContainer,
  CardStaggerItem,
  StaggerContainer,
  StaggerItem,
} from '@/components/page-transition'
import { getChannelTypeLabel } from '@/features/my-channels/constants'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import {
  getSupplierOverview,
  type SupplierBidBucket,
  type SupplierOverview as SupplierOverviewData,
} from '../api'

function formatCny(value: number | null | undefined): string {
  return `¥${(value ?? 0).toFixed(2)}`
}

function formatUsd(value: number | null | undefined): string {
  return `$${(value ?? 0).toFixed(2)}`
}

function SummaryCardsRow(props: { data: SupplierOverviewData }) {
  const { t } = useTranslation()
  const { pending, settled, channels, today_usage } = props.data

  const cards = [
    {
      key: 'pending',
      title: t('Pending Settlement'),
      value: formatCny(pending.payable_cny),
      description: `${formatUsd(pending.official_usd)} · ${t('{{count}} records', {
        count: pending.log_count,
      })}`,
      icon: Receipt,
      tone: 'rose' as const,
    },
    {
      key: 'settled',
      title: t('Settled'),
      value: formatCny(settled.total),
      description: t('Total settled'),
      icon: Wallet,
      tone: 'teal' as const,
      details: [
        { label: t('Today'), value: formatCny(settled.today) },
        { label: t('Last 7 days'), value: formatCny(settled.last7) },
      ],
    },
    {
      key: 'channels',
      title: t('Channels'),
      value: formatNumber(channels.available),
      description: t('{{count}} available', { count: channels.available }),
      icon: RadioTower,
      tone: 'teal' as const,
      details: [
        { label: t('Available'), value: formatNumber(channels.available) },
        {
          label: t('Unavailable'),
          value: formatNumber(channels.unavailable),
          tone:
            channels.unavailable > 0
              ? ('destructive' as const)
              : ('muted' as const),
        },
      ],
    },
    {
      key: 'today',
      title: t("Today's Usage"),
      value: formatNumber(today_usage.requests),
      description: t('{{count}} requests today', {
        count: today_usage.requests,
      }),
      icon: Activity,
      tone: 'gray' as const,
      details: [
        { label: t('Requests'), value: formatNumber(today_usage.requests) },
        { label: t('Tokens'), value: formatNumber(today_usage.tokens) },
      ],
    },
  ]

  return (
    <StaggerContainer className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
      {cards.map((card) => (
        <StaggerItem
          key={card.key}
          className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'
        >
          <StatCard
            title={card.title}
            value={card.value}
            description={card.description}
            icon={card.icon}
            tone={card.tone}
            details={card.details}
          />
        </StaggerItem>
      ))}
    </StaggerContainer>
  )
}

function BidBucketCard(props: { bucket: SupplierBidBucket }) {
  const { t } = useTranslation()
  const { bucket } = props
  const provider = getChannelTypeLabel(bucket.type) || bucket.type_name
  const lowest = bucket.bids[0]?.price ?? null

  return (
    <CardStaggerItem className='bg-card overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center justify-between gap-2 border-b px-4 py-3 sm:px-5'>
        <div className='flex min-w-0 items-center gap-2'>
          <Gavel
            className='text-muted-foreground/60 size-4 shrink-0'
            aria-hidden='true'
          />
          <h3 className='truncate text-sm font-semibold'>
            {provider} · {bucket.group}
          </h3>
        </div>
        <span className='text-muted-foreground bg-muted/60 shrink-0 rounded-md px-2 py-0.5 text-[11px] tabular-nums'>
          {t('{{count}} bids', { count: bucket.total })}
        </span>
      </div>

      <div className='p-4 sm:p-5'>
        {bucket.bids.length === 0 ? (
          <div className='text-muted-foreground py-2 text-center text-xs'>
            {t('No bids yet')}
          </div>
        ) : (
          <div className='flex flex-col gap-1.5'>
            {bucket.bids.map((bid, index) => (
              <div
                key={index}
                className={cn(
                  'flex items-center justify-between gap-2 rounded-xl px-3 py-2',
                  bid.mine ? 'bg-primary/10 ring-primary/30 ring-1' : 'bg-muted/40'
                )}
              >
                <span className='flex min-w-0 items-center gap-2'>
                  <span className='text-muted-foreground font-mono text-xs tabular-nums'>
                    {t('Rank {{rank}}', { rank: index + 1 })}
                  </span>
                  {bid.mine && (
                    <span className='bg-primary text-primary-foreground rounded px-1.5 py-0.5 text-[10px] font-semibold'>
                      {t('You')}
                    </span>
                  )}
                </span>
                <span
                  className={cn(
                    'shrink-0 font-mono text-sm font-semibold tabular-nums',
                    bid.mine && 'text-primary'
                  )}
                >
                  {formatCny(bid.price)}
                </span>
              </div>
            ))}
          </div>
        )}

        <div className='text-muted-foreground mt-3 border-t pt-3 text-xs'>
          {bucket.my_rank > 0 ? (
            <span className='tabular-nums'>
              {t('My rank: {{rank}}/{{total}}', {
                rank: bucket.my_rank,
                total: bucket.total,
              })}
            </span>
          ) : bucket.bids.length > 0 ? (
            <span>
              {t('Not participating (current lowest {{price}})', {
                price: formatCny(lowest),
              })}
            </span>
          ) : (
            <span>{t('No bids yet')}</span>
          )}
        </div>
      </div>
    </CardStaggerItem>
  )
}

function SupplierOverviewSkeleton() {
  return (
    <div className='flex flex-col gap-4'>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, i) => (
          <div
            key={i}
            className='bg-card rounded-2xl border p-4 shadow-xs sm:p-5'
          >
            <Skeleton className='h-3.5 w-20' />
            <Skeleton className='mt-3 h-7 w-24' />
            <Skeleton className='mt-2 h-3.5 w-28' />
          </div>
        ))}
      </div>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='bg-card rounded-2xl border p-4 shadow-xs'>
            <Skeleton className='h-5 w-40' />
            <Skeleton className='mt-4 h-8 w-full' />
            <Skeleton className='mt-2 h-8 w-full' />
          </div>
        ))}
      </div>
    </div>
  )
}

export function SupplierOverview() {
  const { t } = useTranslation()

  const overviewQuery = useQuery({
    queryKey: ['supplier', 'overview'],
    queryFn: getSupplierOverview,
    staleTime: 60 * 1000,
  })

  if (overviewQuery.isLoading) {
    return <SupplierOverviewSkeleton />
  }

  const data = overviewQuery.data
  if (!data) {
    return (
      <div className='text-muted-foreground py-12 text-center text-sm'>
        {t('No data available')}
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-4'>
      <SummaryCardsRow data={data} />

      <div className='flex flex-col gap-3'>
        <div className='flex items-center gap-2'>
          <Gavel
            className='text-muted-foreground/60 size-4 shrink-0'
            aria-hidden='true'
          />
          <h2 className='text-base font-semibold'>{t('Live market bidding')}</h2>
        </div>

        {data.bids.length === 0 ? (
          <div className='bg-card text-muted-foreground rounded-2xl border py-12 text-center text-sm shadow-xs'>
            {t('You are not participating in any market yet')}
          </div>
        ) : (
          <CardStaggerContainer className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
            {data.bids.map((bucket) => (
              <BidBucketCard
                key={`${bucket.type}-${bucket.group}`}
                bucket={bucket}
              />
            ))}
          </CardStaggerContainer>
        )}
      </div>
    </div>
  )
}
