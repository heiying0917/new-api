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
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { updateSupplier } from '../api'
import { SETTLEMENT_MODE_OPTIONS, SETTLEMENT_CYCLE_OPTIONS } from '../constants'
import { type Supplier } from '../types'
import { useSuppliers } from './suppliers-provider'

const supplierFormSchema = z.object({
  priority: z.coerce.number().int().min(0),
  enabled: z.boolean(),
  settlement_mode: z.enum(['manual', 'auto']),
  settlement_cycle: z.enum(['day', 'week', 'month']),
  remark: z.string().max(255).optional().or(z.literal('')),
})

type SupplierFormValues = z.infer<typeof supplierFormSchema>

type SuppliersMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Supplier | null
}

export function SuppliersMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: SuppliersMutateDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSuppliers()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<SupplierFormValues>({
    resolver: zodResolver(supplierFormSchema) as unknown as Resolver<SupplierFormValues>,
    defaultValues: {
      priority: 0,
      enabled: true,
      settlement_mode: 'manual',
      settlement_cycle: 'month',
      remark: '',
    },
  })

  // Reset form when drawer opens with current row data
  const handleOpenChange = (isOpen: boolean) => {
    if (isOpen && currentRow) {
      form.reset({
        priority: currentRow.priority,
        enabled: currentRow.enabled,
        settlement_mode: currentRow.settlement_mode,
        settlement_cycle: currentRow.settlement_cycle,
        remark: currentRow.remark ?? '',
      })
    } else if (!isOpen) {
      form.reset()
    }
    onOpenChange(isOpen)
  }

  const onSubmit = async (data: SupplierFormValues) => {
    if (!currentRow) return

    setIsSubmitting(true)
    try {
      const result = await updateSupplier({
        user_id: currentRow.user_id,
        ...data,
      })

      if (result.success) {
        toast.success(t('Supplier updated successfully'))
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(result.message || t('Failed to update supplier'))
      }
    } catch (_error) {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const modeOptions = SETTLEMENT_MODE_OPTIONS(t)
  const cycleOptions = SETTLEMENT_CYCLE_OPTIONS(t)

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[520px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t('Update')} {t('Supplier')}
          </SheetTitle>
          <SheetDescription>
            {currentRow?.username
              ? `${t('Update supplier settings for')} ${currentRow.username}`
              : t('Update supplier settings.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='supplier-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            {/* Basic Settings */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Supplier Settings')}</h3>

              <FormField
                control={form.control}
                name='priority'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Priority')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        placeholder='0'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <FormLabel className='!mt-0'>{t('Enabled')}</FormLabel>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Settlement Settings */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Settlement')}</h3>

              <FormField
                control={form.control}
                name='settlement_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Settlement Mode')}</FormLabel>
                    <Select
                      items={modeOptions}
                      onValueChange={field.onChange}
                      value={field.value}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t('Select settlement mode')}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {modeOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='settlement_cycle'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Settlement Cycle')}</FormLabel>
                    <Select
                      items={cycleOptions}
                      onValueChange={field.onChange}
                      value={field.value}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t('Select settlement cycle')}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {cycleOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Remarks */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Remark')}</h3>

              <FormField
                control={form.control}
                name='remark'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Remark')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t('Admin notes (only visible to admins)')}
                        rows={3}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button form='supplier-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
