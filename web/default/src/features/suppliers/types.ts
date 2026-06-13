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

export interface Supplier {
  user_id: number
  username: string
  email: string
  phone: string
  user_status: number
  priority: number
  enabled: boolean
  settlement_mode: 'manual' | 'auto'
  settlement_cycle: 'day' | 'week' | 'month'
  remark: string
}

export type SuppliersDialogType = 'update'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetSuppliersResponse {
  success: boolean
  message?: string
  data?: {
    items: Supplier[]
    total: number
  }
}
