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
import { SuppliersMutateDrawer } from './components/suppliers-mutate-drawer'
import { SuppliersPrimaryButtons } from './components/suppliers-primary-buttons'
import { SuppliersProvider, useSuppliers } from './components/suppliers-provider'
import { SuppliersTable } from './components/suppliers-table'

function SuppliersContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useSuppliers()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Supplier Management')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <SuppliersPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <SuppliersTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SuppliersMutateDrawer
        open={open === 'update'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'update' ? currentRow : null}
      />
    </>
  )
}

export function Suppliers() {
  return (
    <SuppliersProvider>
      <SuppliersContent />
    </SuppliersProvider>
  )
}
