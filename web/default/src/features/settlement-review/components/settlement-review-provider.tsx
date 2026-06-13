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

import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import { type Settlement, type SettlementReviewDialogType } from '../types'

type SettlementReviewContextType = {
  open: SettlementReviewDialogType | null
  setOpen: (str: SettlementReviewDialogType | null) => void
  currentRow: Settlement | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Settlement | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const SettlementReviewContext = React.createContext<SettlementReviewContextType | null>(null)

export function SettlementReviewProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<SettlementReviewDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Settlement | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <SettlementReviewContext
      value={{ open, setOpen, currentRow, setCurrentRow, refreshTrigger, triggerRefresh }}
    >
      {children}
    </SettlementReviewContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useSettlementReview = () => {
  const ctx = React.useContext(SettlementReviewContext)
  if (!ctx) {
    throw new Error('useSettlementReview has to be used within <SettlementReviewContext>')
  }
  return ctx
}
