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
import type { GetMyChannelsResponse, ApiResponse, MyChannel } from './types'

/**
 * Get paginated list of the caller's own channels
 */
export async function getMyChannels(params: {
  p?: number
  page_size?: number
  keyword?: string
} = {}): Promise<GetMyChannelsResponse> {
  const { p = 1, page_size = 20, keyword = '' } = params
  const qs = new URLSearchParams()
  qs.set('p', String(p))
  qs.set('page_size', String(page_size))
  if (keyword) qs.set('keyword', keyword)
  const res = await api.get(`/api/supplier/channel/?${qs.toString()}`)
  return res.data
}

/**
 * Get a single channel by ID (includes key)
 */
export async function getMyChannel(id: number): Promise<ApiResponse<MyChannel>> {
  const res = await api.get(`/api/supplier/channel/${id}`)
  return res.data
}

/**
 * Create a new channel
 */
export async function addMyChannel(payload: Omit<MyChannel, 'id' | 'supplier_id'>): Promise<ApiResponse<MyChannel>> {
  const res = await api.post('/api/supplier/channel/', payload)
  return res.data
}

/**
 * Update an existing channel (payload must include id)
 */
export async function updateMyChannel(payload: Partial<MyChannel> & { id: number }): Promise<ApiResponse<MyChannel>> {
  const res = await api.put('/api/supplier/channel/', payload)
  return res.data
}

/**
 * Delete a channel by ID
 */
export async function deleteMyChannel(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/supplier/channel/${id}`)
  return res.data
}
