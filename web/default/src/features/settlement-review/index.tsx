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

import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { SettlementAdminCancelDialog } from './components/settlement-cancel-dialog'
import { SettlementConfirmDrawer } from './components/settlement-confirm-drawer'
import { SettlementDetailDrawer } from './components/settlement-detail-drawer'
import { SettlementReviewProvider, useSettlementReview } from './components/settlement-review-provider'
import { SettlementReviewTable } from './components/settlement-review-table'

function SettlementReviewContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useSettlementReview()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Settlement Review')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <SettlementReviewTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SettlementDetailDrawer
        open={open === 'detail'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        settlement={open === 'detail' ? currentRow : null}
      />

      <SettlementConfirmDrawer
        open={open === 'confirm'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        settlement={open === 'confirm' ? currentRow : null}
      />

      <SettlementAdminCancelDialog
        open={open === 'cancel'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'cancel' ? currentRow : null}
      />
    </>
  )
}

export function SettlementReview() {
  return (
    <SettlementReviewProvider>
      <SettlementReviewContent />
    </SettlementReviewProvider>
  )
}
