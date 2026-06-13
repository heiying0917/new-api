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
import type { GetSuppliersResponse, ApiResponse } from './types'

/**
 * Get paginated suppliers list
 */
export async function getSuppliers(
  params: { p?: number; page_size?: number } = {}
): Promise<GetSuppliersResponse> {
  const { p = 1, page_size = 20 } = params
  const res = await api.get(`/api/supplier/?p=${p}&page_size=${page_size}`)
  return res.data
}

/**
 * Search suppliers by keyword
 */
export async function searchSuppliers(params: {
  keyword?: string
  p?: number
  page_size?: number
}): Promise<GetSuppliersResponse> {
  const { keyword = '', p = 1, page_size = 20 } = params
  const qs = new URLSearchParams()
  qs.set('keyword', keyword)
  qs.set('p', String(p))
  qs.set('page_size', String(page_size))
  const res = await api.get(`/api/supplier/search?${qs.toString()}`)
  return res.data
}

/**
 * Update an existing supplier
 */
export async function updateSupplier(payload: {
  user_id: number
  priority?: number
  enabled?: boolean
  settlement_mode?: 'manual' | 'auto'
  settlement_cycle?: 'day' | 'week' | 'month'
  remark?: string
}): Promise<ApiResponse> {
  const res = await api.put('/api/supplier/', payload)
  return res.data
}
