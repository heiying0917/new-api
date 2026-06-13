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
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { MyChannelsDeleteDialog } from './components/my-channels-delete-dialog'
import { MyChannelsMutateDrawer } from './components/my-channels-mutate-drawer'
import { MyChannelsProvider, useMyChannels } from './components/my-channels-provider'
import { MyChannelsTable } from './components/my-channels-table'

function MyChannelsContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useMyChannels()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('My Channels')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setOpen('create')}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Create Channel')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <MyChannelsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <MyChannelsMutateDrawer
        open={open === 'create'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        mode='create'
      />

      <MyChannelsMutateDrawer
        open={open === 'update'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        mode='update'
        currentRow={open === 'update' ? currentRow : null}
      />

      <MyChannelsDeleteDialog
        open={open === 'delete'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'delete' ? currentRow : null}
      />
    </>
  )
}

export function MyChannels() {
  return (
    <MyChannelsProvider>
      <MyChannelsContent />
    </MyChannelsProvider>
  )
}
