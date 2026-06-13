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

export enum SettlementStatus {
  Applied = 1,
  Settled = 2,
  Cancelled = 3,
}

export interface Settlement {
  id: number
  supplier_id: number
  status: SettlementStatus
  period_start: number
  period_end: number
  official_usd: number
  computed_cny: number
  actual_amount: number
  actual_currency: string
  settle_method: string
  remark: string
  source: string
  log_count: number
  created_at: number
  settled_at: number
}

export interface SettlementLog {
  id: number
  [key: string]: unknown
}

export type SettlementReviewDialogType = 'detail' | 'confirm' | 'cancel'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetSettlementsResponse {
  success: boolean
  message?: string
  data?: {
    items: Settlement[]
    total: number
  }
}

export interface GetSettlementLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: SettlementLog[]
    total: number
  }
}

export interface ConfirmSettlementPayload {
  actual_amount: number
  actual_currency: 'CNY' | 'USD'
  settle_method: string
  remark: string
}
