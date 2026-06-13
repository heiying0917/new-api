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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { PlusCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { createSettlement, getPending } from './api'
import { SettlementCancelDialog } from './components/settlement-cancel-dialog'
import { SettlementDetailDrawer } from './components/settlement-detail-drawer'
import {
  SettlementsProvider,
  useSettlements,
} from './components/settlements-provider'
import { SettlementsTable } from './components/settlements-table'

function SettlementsContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useSettlements()
  const [createOpen, setCreateOpen] = useState(false)
  const [isCreating, setIsCreating] = useState(false)

  const { data: pendingData, refetch: refetchPending } = useQuery({
    queryKey: ['settlements-pending'],
    queryFn: async () => {
      const res = await getPending()
      if (!res.success) return null
      return res.data ?? null
    },
  })

  const handleCreate = async () => {
    setIsCreating(true)
    try {
      const res = await createSettlement()
      if (res.success) {
        toast.success(t('Settlement created'))
        setCreateOpen(false)
        triggerRefresh()
        void refetchPending()
      } else {
        toast.error(res.message || t('Failed to create settlement'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsCreating(false)
    }
  }

  const pendingUsd = pendingData?.official_usd ?? 0

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Billing & Settlement')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setCreateOpen(true)} disabled={pendingUsd <= 0}>
            <PlusCircle className='mr-2 h-4 w-4' />
            {t('Create Settlement')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {pendingData && (
            <div className='mb-4 flex gap-4 rounded-lg border p-4'>
              <div>
                <span className='text-muted-foreground text-sm'>
                  {t('Pending Amount')}
                </span>
                <p className='text-lg font-semibold'>
                  ${pendingData.official_usd?.toFixed(6)}
                </p>
              </div>
              <div>
                <span className='text-muted-foreground text-sm'>
                  {t('Payable')}
                </span>
                <p className='text-lg font-semibold'>
                  ¥{pendingData.payable_cny?.toFixed(2)}
                </p>
              </div>
              <div>
                <span className='text-muted-foreground text-sm'>
                  {t('Records')}
                </span>
                <p className='text-lg font-semibold'>{pendingData.log_count}</p>
              </div>
            </div>
          )}
          <SettlementsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ConfirmDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title={t('Create Settlement')}
        desc={
          pendingUsd > 0
            ? `${t('Confirm settle this bill?')} $${pendingUsd.toFixed(6)}`
            : t('Confirm settle this bill?')
        }
        confirmText={isCreating ? t('Creating...') : t('Confirm')}
        handleConfirm={handleCreate}
        isLoading={isCreating}
      />

      <SettlementDetailDrawer
        open={open === 'detail'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        settlement={open === 'detail' ? currentRow : null}
      />

      <SettlementCancelDialog
        open={open === 'cancel'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'cancel' ? currentRow : null}
      />
    </>
  )
}

export function Settlements() {
  return (
    <SettlementsProvider>
      <SettlementsContent />
    </SettlementsProvider>
  )
}
