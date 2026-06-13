import { useQuery } from '@tanstack/react-query'
import { DollarSign, Receipt, Tag } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Skeleton } from '@/components/ui/skeleton'
import { getSupplierPending, getMarketPrices } from '../api'

export function SupplierOverviewCard() {
  const { t } = useTranslation()
  const role = useAuthStore((s) => s.auth.user?.role ?? 0)

  const pendingQuery = useQuery({
    queryKey: ['supplier', 'pending'],
    queryFn: getSupplierPending,
    staleTime: 60 * 1000,
    enabled: role >= ROLE.SUPPLIER,
  })

  const marketQuery = useQuery({
    queryKey: ['supplier', 'market-prices'],
    queryFn: getMarketPrices,
    staleTime: 5 * 60 * 1000,
    enabled: role >= ROLE.SUPPLIER,
  })

  if (role < ROLE.SUPPLIER) return null

  const pending = pendingQuery.data
  const prices = marketQuery.data
  const priceEntries = prices ? Object.entries(prices) : []

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <DollarSign
          className='text-muted-foreground/60 size-4 shrink-0'
          aria-hidden='true'
        />
        <h3 className='text-sm font-semibold'>{t('Supplier Overview')}</h3>
      </div>

      <div className='grid gap-4 p-4 sm:grid-cols-2 sm:p-5'>
        {/* Pending Settlement */}
        <div>
          <div className='mb-2 flex items-center gap-1.5'>
            <Receipt
              className='text-muted-foreground/60 size-3.5 shrink-0'
              aria-hidden='true'
            />
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Pending Settlement')}
            </span>
          </div>

          {pendingQuery.isLoading ? (
            <div className='space-y-1.5'>
              <Skeleton className='h-5 w-24' />
              <Skeleton className='h-4 w-20' />
              <Skeleton className='h-4 w-16' />
            </div>
          ) : (
            <div className='bg-muted/40 space-y-1.5 rounded-xl px-3 py-2.5'>
              <div className='flex items-baseline justify-between gap-2'>
                <span className='text-muted-foreground text-[11px]'>
                  {t('Payable')}
                </span>
                <span className='font-mono text-sm font-semibold tabular-nums'>
                  ${pending?.official_usd.toFixed(2) ?? '0.00'}
                </span>
              </div>
              <div className='flex items-baseline justify-between gap-2'>
                <span className='text-muted-foreground text-[11px]'>CNY</span>
                <span className='font-mono text-sm font-semibold tabular-nums'>
                  ¥{pending?.payable_cny.toFixed(2) ?? '0.00'}
                </span>
              </div>
              <div className='flex items-baseline justify-between gap-2'>
                <span className='text-muted-foreground text-[11px]'>
                  {t('Records')}
                </span>
                <span className='text-muted-foreground font-mono text-xs tabular-nums'>
                  {pending?.log_count ?? 0}
                </span>
              </div>
            </div>
          )}
        </div>

        {/* Market Prices */}
        <div>
          <div className='mb-2 flex items-center gap-1.5'>
            <Tag
              className='text-muted-foreground/60 size-3.5 shrink-0'
              aria-hidden='true'
            />
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Market Prices')}
            </span>
          </div>

          {marketQuery.isLoading ? (
            <div className='space-y-1.5'>
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className='h-5 w-full rounded' />
              ))}
            </div>
          ) : priceEntries.length === 0 ? (
            <div className='text-muted-foreground text-xs'>—</div>
          ) : (
            <div className='bg-muted/40 rounded-xl px-3 py-2'>
              {priceEntries.map(([group, price]) => (
                <div
                  key={group}
                  className='flex items-baseline justify-between gap-2 py-1'
                >
                  <span className='text-muted-foreground min-w-0 truncate font-mono text-[11px]'>
                    {group}
                  </span>
                  <span className='shrink-0 font-mono text-xs font-semibold tabular-nums'>
                    ¥{price.toFixed(4)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </section>
  )
}
