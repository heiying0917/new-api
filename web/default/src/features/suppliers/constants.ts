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

export const SETTLEMENT_MODE_OPTIONS = (t: (k: string) => string) => [
  { label: t('Manual'), value: 'manual' },
  { label: t('Auto'), value: 'auto' },
]

export const SETTLEMENT_CYCLE_OPTIONS = (t: (k: string) => string) => [
  { label: t('Daily'), value: 'day' },
  { label: t('Weekly'), value: 'week' },
  { label: t('Monthly'), value: 'month' },
]

export const getSettlementModeLabel = (
  mode: string,
  t: (k: string) => string
): string => {
  const option = SETTLEMENT_MODE_OPTIONS(t).find((o) => o.value === mode)
  return option?.label ?? mode
}

export const getSettlementCycleLabel = (
  cycle: string,
  t: (k: string) => string
): string => {
  const option = SETTLEMENT_CYCLE_OPTIONS(t).find((o) => o.value === cycle)
  return option?.label ?? cycle
}

export const ERROR_MESSAGES = {
  UNEXPECTED: 'An unexpected error occurred',
  LOAD_FAILED: 'Failed to load suppliers',
  UPDATE_FAILED: 'Failed to update supplier',
} as const

export const SUCCESS_MESSAGES = {
  SUPPLIER_UPDATED: 'Supplier updated successfully',
} as const
