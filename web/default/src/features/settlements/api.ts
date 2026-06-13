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
import { api } from '@/lib/api'
import type {
  GetSettlementsResponse,
  GetSettlementLogsResponse,
  ApiResponse,
  Settlement,
  PendingInfo,
} from './types'

export async function createSettlement(): Promise<ApiResponse<Settlement>> {
  const res = await api.post('/api/supplier/self/settlement/')
  return res.data
}

export async function getSettlements(
  params: { p?: number; page_size?: number } = {}
): Promise<GetSettlementsResponse> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(
    `/api/supplier/self/settlement/?p=${p}&page_size=${page_size}`
  )
  return res.data
}

export async function getSettlement(
  id: number
): Promise<ApiResponse<Settlement>> {
  const res = await api.get(`/api/supplier/self/settlement/${id}`)
  return res.data
}

export async function getSettlementLogs(
  id: number,
  params: { p?: number; page_size?: number } = {}
): Promise<GetSettlementLogsResponse> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(
    `/api/supplier/self/settlement/${id}/logs?p=${p}&page_size=${page_size}`
  )
  return res.data
}

export async function cancelSettlement(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/supplier/self/settlement/${id}/cancel`)
  return res.data
}

export async function getPending(): Promise<ApiResponse<PendingInfo>> {
  const res = await api.get('/api/supplier/self/pending')
  return res.data
}

export function settlementExportUrl(id: number): string {
  return `/api/supplier/self/settlement/${id}/export`
}
