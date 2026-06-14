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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { Activity, BarChart3, DollarSign, RadioTower, Trophy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { formatNumber } from '@/lib/format'
import { getRollingDateRange } from '@/lib/time'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'
import { useTheme } from '@/context/theme-provider'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StaggerContainer, StaggerItem } from '@/components/page-transition'
import {
  getSupplierDashboard,
  getSupplierRealtime,
  type SupplierDashboardRankItem,
  type SupplierDashboardSeriesPoint,
} from '../api'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

const THEME_CHART_COLOR_VARIABLES = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
] as const

function getThemeChartColors(): string[] {
  if (typeof document === 'undefined') return []
  const bodyStyle = window.getComputedStyle(document.body)
  const rootStyle = window.getComputedStyle(document.documentElement)
  return THEME_CHART_COLOR_VARIABLES.map((name) =>
    (
      bodyStyle.getPropertyValue(name) || rootStyle.getPropertyValue(name)
    ).trim()
  ).filter(Boolean)
}

function formatUsd(value: number | null | undefined): string {
  return `$${(value ?? 0).toFixed(2)}`
}

function formatDay(tsSec: number): string {
  return dayjs(tsSec * 1000).format('MM-DD')
}

type RangeOption = { labelKey: string; days: number }

const RANGE_OPTIONS: RangeOption[] = [
  { labelKey: 'Today', days: 1 },
  { labelKey: 'Last 7 days', days: 7 },
  { labelKey: 'Last 30 days', days: 30 },
]

/**
 * Hook: load + watch the VChart theme (light/dark), matching the existing
 * dashboard chart components.
 */
function useVChartTheme() {
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  useEffect(() => {
    let cancelled = false
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      if (cancelled) return
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    void updateTheme()
    return () => {
      cancelled = true
    }
  }, [resolvedTheme])

  return { resolvedTheme, themeReady }
}

interface ChartCardProps {
  title: string
  icon: typeof BarChart3
  spec: Record<string, unknown> | null
  resolvedTheme: string
  themeReady: boolean
  chartKey: string
  isEmpty: boolean
  emptyLabel: string
}

function ChartCard(props: ChartCardProps) {
  const Icon = props.icon
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <Icon className='text-muted-foreground/60 size-4' />
        <div className='text-sm font-semibold'>{props.title}</div>
      </div>
      <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
        {props.isEmpty ? (
          <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
            {props.emptyLabel}
          </div>
        ) : (
          props.themeReady &&
          props.spec && (
            <VChart
              key={props.chartKey}
              spec={{
                ...props.spec,
                theme: props.resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          )
        )}
      </div>
    </div>
  )
}

interface UsageTrendChartProps {
  series: SupplierDashboardSeriesPoint[]
}

function UsageTrendChart(props: UsageTrendChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useVChartTheme()
  const { customization } = useThemeCustomization()

  const { spec, isEmpty } = useMemo(() => {
    const requestsLabel = t('Requests')
    const tokensLabel = t('Tokens')
    const colors = getThemeChartColors()
    const requestsColor = colors[0] || '#5B8FF9'
    const tokensColor = colors[1] || '#5AD8A6'

    const values: Array<{ Time: string; Metric: string; Value: number }> = []
    props.series.forEach((point) => {
      const time = formatDay(point.day)
      values.push({ Time: time, Metric: requestsLabel, Value: point.requests })
      values.push({ Time: time, Metric: tokensLabel, Value: point.tokens })
    })

    return {
      isEmpty: props.series.length === 0,
      spec: {
        type: 'line',
        data: [{ id: 'usageTrend', values }],
        xField: 'Time',
        yField: 'Value',
        seriesField: 'Metric',
        legends: { visible: true },
        color: {
          type: 'ordinal',
          domain: [requestsLabel, tokensLabel],
          range: [requestsColor, tokensColor],
        },
        axes: [
          { orient: 'bottom', type: 'band' },
          {
            orient: 'left',
            type: 'linear',
            label: {
              formatMethod: (value: number) => formatNumber(value),
            },
          },
        ],
        line: {
          style: { lineWidth: 2, curveType: 'monotone' },
        },
        point: { visible: false },
        tooltip: {
          dimension: {
            content: [
              {
                key: (datum: Record<string, unknown>) => datum?.Metric,
                value: (datum: Record<string, unknown>) =>
                  formatNumber(Number(datum?.Value) || 0),
              },
            ],
          },
        },
      } as Record<string, unknown>,
    }
  }, [props.series, t])

  const chartKey = [
    'usage-trend',
    props.series.length,
    resolvedTheme,
    customization.preset,
  ].join('-')

  return (
    <ChartCard
      title={t('Channel usage trend')}
      icon={BarChart3}
      spec={spec}
      resolvedTheme={resolvedTheme}
      themeReady={themeReady}
      chartKey={chartKey}
      isEmpty={isEmpty}
      emptyLabel={t('No data available')}
    />
  )
}

interface RevenueTrendChartProps {
  series: SupplierDashboardSeriesPoint[]
}

function RevenueTrendChart(props: RevenueTrendChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useVChartTheme()
  const { customization } = useThemeCustomization()

  const { spec, isEmpty } = useMemo(() => {
    const colors = getThemeChartColors()
    const barColor = colors[2] || colors[0] || '#F6BD16'
    const values = props.series.map((point) => ({
      Time: formatDay(point.day),
      Usd: Number((point.official_usd ?? 0).toFixed(4)),
    }))

    return {
      isEmpty: props.series.length === 0,
      spec: {
        type: 'bar',
        data: [{ id: 'revenueTrend', values }],
        xField: 'Time',
        yField: 'Usd',
        legends: { visible: false },
        color: [barColor],
        bar: {
          state: { hover: { stroke: '#000', lineWidth: 1 } },
        },
        axes: [
          { orient: 'bottom', type: 'band' },
          {
            orient: 'left',
            type: 'linear',
            label: {
              formatMethod: (value: number) => formatUsd(value),
            },
          },
        ],
        tooltip: {
          mark: {
            content: [
              {
                key: () => t('Official amount'),
                value: (datum: Record<string, unknown>) =>
                  formatUsd(Number(datum?.Usd) || 0),
              },
            ],
          },
        },
      } as Record<string, unknown>,
    }
  }, [props.series, t])

  const chartKey = [
    'revenue-trend',
    props.series.length,
    resolvedTheme,
    customization.preset,
  ].join('-')

  return (
    <ChartCard
      title={t('Revenue trend')}
      icon={DollarSign}
      spec={spec}
      resolvedTheme={resolvedTheme}
      themeReady={themeReady}
      chartKey={chartKey}
      isEmpty={isEmpty}
      emptyLabel={t('No data available')}
    />
  )
}

interface ChannelRankingTableProps {
  ranking: SupplierDashboardRankItem[]
}

function ChannelRankingTable(props: ChannelRankingTableProps) {
  const { t } = useTranslation()

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <Trophy className='text-muted-foreground/60 size-4' />
        <div className='text-sm font-semibold'>{t('Channel ranking')}</div>
      </div>
      {props.ranking.length === 0 ? (
        <div className='text-muted-foreground py-12 text-center text-sm'>
          {t('No data available')}
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Channel')}</TableHead>
              <TableHead className='text-right'>{t('Requests')}</TableHead>
              <TableHead className='text-right'>{t('Tokens')}</TableHead>
              <TableHead className='text-right'>
                {t('Official amount')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.ranking.map((item) => (
              <TableRow key={item.channel_id}>
                <TableCell className='font-medium'>
                  {item.channel_name || `#${item.channel_id}`}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatNumber(item.requests)}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatNumber(item.tokens)}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatUsd(item.official_usd)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

function RealtimeCards() {
  const { t } = useTranslation()

  const realtimeQuery = useQuery({
    queryKey: ['supplier', 'realtime'],
    queryFn: getSupplierRealtime,
    refetchInterval: 10000,
    refetchIntervalInBackground: false,
  })

  const data = realtimeQuery.data
  const loading = realtimeQuery.isLoading
  const error = realtimeQuery.isError

  const cards = [
    {
      key: 'rpm',
      title: t('Realtime RPM'),
      value: formatNumber(data?.rpm ?? 0),
      description: t('Requests per minute (last 60s)'),
      icon: Activity,
      tone: 'teal' as const,
    },
    {
      key: 'tpm',
      title: t('Realtime TPM'),
      value: formatNumber(data?.tpm ?? 0),
      description: t('Tokens per minute (last 60s)'),
      icon: RadioTower,
      tone: 'gray' as const,
    },
  ]

  return (
    <StaggerContainer className='grid gap-3 sm:grid-cols-2'>
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
            loading={loading}
            error={error}
          />
        </StaggerItem>
      ))}
    </StaggerContainer>
  )
}

interface RangeSelectorProps {
  value: number
  onChange: (days: number) => void
}

function RangeSelector(props: RangeSelectorProps) {
  const { t } = useTranslation()
  return (
    <div className='bg-muted/60 inline-flex h-8 overflow-x-auto rounded-lg border p-0.5'>
      {RANGE_OPTIONS.map((option) => (
        <button
          key={option.days}
          type='button'
          onClick={() => props.onChange(option.days)}
          className={cn(
            'shrink-0 rounded-md px-3 text-xs font-medium transition-colors',
            props.value === option.days
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {t(option.labelKey)}
        </button>
      ))}
    </div>
  )
}

function DashboardSkeleton() {
  return (
    <div className='flex flex-col gap-4'>
      <div className='grid gap-3 sm:grid-cols-2'>
        {Array.from({ length: 2 }).map((_, i) => (
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
      <div className='grid gap-4 xl:grid-cols-2'>
        {Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className='overflow-hidden rounded-lg border'>
            <div className='border-b px-4 py-3'>
              <Skeleton className='h-5 w-32' />
            </div>
            <div className='h-96 p-2'>
              <Skeleton className='h-full w-full' />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export function SupplierDataDashboard() {
  const { t } = useTranslation()
  const [rangeDays, setRangeDays] = useState(7)

  const range = useMemo(() => {
    const { start, end } = getRollingDateRange(rangeDays)
    return {
      start: Math.floor(start.getTime() / 1000),
      end: Math.floor(end.getTime() / 1000),
    }
  }, [rangeDays])

  const dashboardQuery = useQuery({
    queryKey: ['supplier', 'dashboard', range.start, range.end],
    queryFn: () => getSupplierDashboard(range.start, range.end),
    staleTime: 30 * 1000,
  })

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h2 className='text-base font-semibold'>
          {t('Supplier data dashboard')}
        </h2>
        <RangeSelector value={rangeDays} onChange={setRangeDays} />
      </div>

      <RealtimeCards />

      {dashboardQuery.isLoading ? (
        <DashboardSkeleton />
      ) : (
        <>
          <div className='grid gap-4 xl:grid-cols-2'>
            <UsageTrendChart series={dashboardQuery.data?.series ?? []} />
            <RevenueTrendChart series={dashboardQuery.data?.series ?? []} />
          </div>
          <ChannelRankingTable ranking={dashboardQuery.data?.ranking ?? []} />
        </>
      )}

      {dashboardQuery.isError && (
        <div className='text-muted-foreground py-4 text-center text-sm'>
          {t('No data available')}
        </div>
      )}
    </div>
  )
}
