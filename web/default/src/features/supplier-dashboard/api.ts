import { api } from '@/lib/api'

export interface SupplierPending {
  official_usd: number
  payable_cny: number
  log_count: number
}

export type MarketPrices = Record<string, number>

export async function getSupplierPending(): Promise<SupplierPending> {
  const res = await api.get<{ success: boolean; data: SupplierPending }>(
    '/api/supplier/self/pending'
  )
  return res.data.data
}

export async function getMarketPrices(): Promise<MarketPrices> {
  const res = await api.get<{ success: boolean; data: MarketPrices }>(
    '/api/supplier/self/market-price'
  )
  return res.data.data
}

export interface SupplierSettledOverview {
  today: number
  last7: number
  total: number
}

export interface SupplierChannelsOverview {
  available: number
  unavailable: number
}

export interface SupplierTodayUsageOverview {
  requests: number
  tokens: number
}

export interface SupplierBidEntry {
  price: number
  mine: boolean
}

export interface SupplierBidBucket {
  type: number
  type_name: string
  group: string
  bids: SupplierBidEntry[]
  my_rank: number
  my_best: number | null
  total: number
}

export interface SupplierOverview {
  pending: SupplierPending
  settled: SupplierSettledOverview
  channels: SupplierChannelsOverview
  today_usage: SupplierTodayUsageOverview
  bids: SupplierBidBucket[]
}

export async function getSupplierOverview(): Promise<SupplierOverview> {
  const res = await api.get<{ success: boolean; data: SupplierOverview }>(
    '/api/supplier/self/overview'
  )
  return res.data.data
}

export interface SupplierDashboardSeriesPoint {
  /** Unix seconds, day-bucket start */
  day: number
  requests: number
  tokens: number
  official_usd: number
}

export interface SupplierDashboardRankItem {
  channel_id: number
  channel_name: string
  requests: number
  tokens: number
  official_usd: number
}

export interface SupplierDashboard {
  series: SupplierDashboardSeriesPoint[]
  ranking: SupplierDashboardRankItem[]
}

export async function getSupplierDashboard(
  startTs?: number,
  endTs?: number
): Promise<SupplierDashboard> {
  const params: Record<string, number> = {}
  if (startTs != null) params.start_timestamp = startTs
  if (endTs != null) params.end_timestamp = endTs

  const res = await api.get<{ success: boolean; data: SupplierDashboard }>(
    '/api/supplier/self/dashboard',
    { params }
  )
  const data = res.data.data
  return {
    series: data?.series ?? [],
    ranking: data?.ranking ?? [],
  }
}

export interface SupplierRealtime {
  rpm: number
  tpm: number
}

export async function getSupplierRealtime(): Promise<SupplierRealtime> {
  const res = await api.get<{ success: boolean; data: SupplierRealtime }>(
    '/api/supplier/self/realtime'
  )
  const data = res.data.data
  return { rpm: data?.rpm ?? 0, tpm: data?.tpm ?? 0 }
}
