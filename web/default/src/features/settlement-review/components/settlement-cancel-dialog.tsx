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

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { cancelSettlement } from '../api'
import { type Settlement } from '../types'
import { useSettlementReview } from './settlement-review-provider'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Settlement | null
}

export function SettlementAdminCancelDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSettlementReview()
  const [isLoading, setIsLoading] = useState(false)

  const handleConfirm = async () => {
    if (!currentRow) return
    setIsLoading(true)
    try {
      const res = await cancelSettlement(currentRow.id)
      if (res.success) {
        toast.success(t('Settlement cancelled'))
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(res.message || t('Failed to cancel settlement'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Cancel Settlement')}
      desc={t('Are you sure you want to cancel this settlement? This action cannot be undone.')}
      destructive
      confirmText={isLoading ? t('Cancelling...') : t('Confirm')}
      handleConfirm={handleConfirm}
      isLoading={isLoading}
    />
  )
}
