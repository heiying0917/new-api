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
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
  SheetClose,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
  sideDrawerFormClassName,
  sideDrawerFooterClassName,
  SideDrawerSection,
} from '@/components/drawer-layout'
import { confirmSettlement } from '../api'
import { type Settlement } from '../types'
import { useSettlementReview } from './settlement-review-provider'

const confirmSchema = z.object({
  actual_amount: z.coerce.number().positive(),
  actual_currency: z.enum(['CNY', 'USD']),
  settle_method: z.string().min(1),
  remark: z.string().optional().or(z.literal('')),
})

type ConfirmFormValues = z.infer<typeof confirmSchema>

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  settlement: Settlement | null
}

export function SettlementConfirmDrawer({ open, onOpenChange, settlement }: Props) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSettlementReview()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<ConfirmFormValues>({
    resolver: zodResolver(confirmSchema) as unknown as Resolver<ConfirmFormValues>,
    defaultValues: {
      actual_amount: 0,
      actual_currency: 'CNY',
      settle_method: '',
      remark: '',
    },
  })

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) form.reset()
    onOpenChange(isOpen)
  }

  const onSubmit = async (data: ConfirmFormValues) => {
    if (!settlement) return
    setIsSubmitting(true)
    try {
      const res = await confirmSettlement(settlement.id, {
        actual_amount: data.actual_amount,
        actual_currency: data.actual_currency,
        settle_method: data.settle_method,
        remark: data.remark ?? '',
      })
      if (res.success) {
        toast.success(t('Settlement confirmed'))
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(res.message || t('Failed to confirm settlement'))
      }
    } catch {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[480px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Confirm Settlement')}</SheetTitle>
          <SheetDescription>
            {settlement ? `${t('Settlement')} #${settlement.id}` : ''}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='confirm-settlement-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='actual_amount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Actual Amount')}</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' step='0.01' min='0.01' placeholder='0.00' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='actual_currency'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Currency')}</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('Select currency')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='CNY'>CNY</SelectItem>
                          <SelectItem value='USD'>USD</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='settle_method'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Settle Method')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('e.g. bank transfer, alipay')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='remark'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Remark')}</FormLabel>
                    <FormControl>
                      <Textarea {...field} rows={3} placeholder={t('Optional notes')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>{t('Close')}</SheetClose>
          <Button form='confirm-settlement-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Confirming...') : t('Confirm Settlement')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
