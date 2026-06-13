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
