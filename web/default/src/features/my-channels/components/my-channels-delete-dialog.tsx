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
import { deleteMyChannel } from '../api'
import { type MyChannel } from '../types'
import { useMyChannels } from './my-channels-provider'

type MyChannelsDeleteDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: MyChannel | null
}

export function MyChannelsDeleteDialog({
  open,
  onOpenChange,
  currentRow,
}: MyChannelsDeleteDialogProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useMyChannels()
  const [isDeleting, setIsDeleting] = useState(false)

  const handleConfirm = async () => {
    if (!currentRow) return

    setIsDeleting(true)
    try {
      const result = await deleteMyChannel(currentRow.id)

      if (result.success) {
        toast.success(t('Channel deleted successfully'))
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to delete channel'))
      }
    } catch (_error) {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Delete Channel')}
      desc={
        currentRow?.name
          ? `${t('Are you sure you want to delete channel')} "${currentRow.name}"? ${t('This action cannot be undone.')}`
          : t('Are you sure you want to delete this channel? This action cannot be undone.')
      }
      destructive
      confirmText={isDeleting ? t('Deleting...') : t('Delete')}
      handleConfirm={handleConfirm}
      isLoading={isDeleting}
    />
  )
}
