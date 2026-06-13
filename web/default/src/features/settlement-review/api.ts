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
import type { GetSettlementsResponse, GetSettlementLogsResponse, ApiResponse, Settlement, ConfirmSettlementPayload } from './types'

export async function listSettlements(params: { status?: number; p?: number; page_size?: number } = {}): Promise<GetSettlementsResponse> {
  const { status, p = 1, page_size = 20 } = params
  const qs = new URLSearchParams()
  if (status) qs.set('status', String(status))
  qs.set('p', String(p))
  qs.set('page_size', String(page_size))
  const res = await api.get(`/api/admin/settlement/?${qs.toString()}`)
  return res.data
}

export async function getSettlement(id: number): Promise<ApiResponse<Settlement>> {
  const res = await api.get(`/api/admin/settlement/${id}`)
  return res.data
}

export async function getSettlementLogs(id: number, params: { p?: number; page_size?: number } = {}): Promise<GetSettlementLogsResponse> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(`/api/admin/settlement/${id}/logs?p=${p}&page_size=${page_size}`)
  return res.data
}

export async function confirmSettlement(id: number, payload: ConfirmSettlementPayload): Promise<ApiResponse> {
  const res = await api.post(`/api/admin/settlement/${id}/confirm`, payload)
  return res.data
}

export async function cancelSettlement(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/admin/settlement/${id}/cancel`)
  return res.data
}

export function exportUrl(id: number): string {
  return `/api/admin/settlement/${id}/export`
}
