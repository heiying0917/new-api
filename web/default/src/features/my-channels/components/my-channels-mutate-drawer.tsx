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
import { Textarea } from '@/components/ui/textarea'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { PasswordInput } from '@/components/password-input'
import { addMyChannel, updateMyChannel } from '../api'
import { CHANNEL_TYPE_OPTIONS } from '../constants'
import { type MyChannel } from '../types'
import { useMyChannels } from './my-channels-provider'

const myChannelFormSchema = z.object({
  type: z.coerce.number().int().min(0),
  name: z.string().min(1, 'Name is required'),
  base_url: z.string().optional().or(z.literal('')),
  group: z.string().min(1, 'Group is required').default('default'),
  models: z.string().min(1, 'Models are required'),
  key: z.string().optional().or(z.literal('')),
  priority: z.coerce.number().int().optional().nullable(),
  cost_price: z.coerce.number().positive('Cost price must be greater than 0'),
  model_mapping: z.string().optional().or(z.literal('')),
  remark: z.string().max(255).optional().or(z.literal('')),
})

type MyChannelFormValues = z.infer<typeof myChannelFormSchema>

type MyChannelsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: 'create' | 'update'
  currentRow?: MyChannel | null
}

export function MyChannelsMutateDrawer({
  open,
  onOpenChange,
  mode,
  currentRow,
}: MyChannelsMutateDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useMyChannels()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const isUpdate = mode === 'update'

  const form = useForm<MyChannelFormValues>({
    resolver: zodResolver(myChannelFormSchema) as unknown as Resolver<MyChannelFormValues>,
    defaultValues: {
      type: 1,
      name: '',
      base_url: '',
      group: 'default',
      models: '',
      key: '',
      priority: null,
      cost_price: undefined,
      model_mapping: '',
      remark: '',
    },
  })

  const handleOpenChange = (isOpen: boolean) => {
    if (isOpen && isUpdate && currentRow) {
      form.reset({
        type: currentRow.type,
        name: currentRow.name,
        base_url: currentRow.base_url ?? '',
        group: currentRow.group ?? 'default',
        models: currentRow.models ?? '',
        key: '',
        priority: currentRow.priority ?? null,
        cost_price: currentRow.cost_price ?? undefined,
        model_mapping: currentRow.model_mapping ?? '',
        remark: currentRow.remark ?? '',
      })
    } else if (isOpen && !isUpdate) {
      form.reset({
        type: 1,
        name: '',
        base_url: '',
        group: 'default',
        models: '',
        key: '',
        priority: null,
        cost_price: undefined,
        model_mapping: '',
        remark: '',
      })
    } else if (!isOpen) {
      form.reset()
    }
    onOpenChange(isOpen)
  }

  const onSubmit = async (data: MyChannelFormValues) => {
    setIsSubmitting(true)
    try {
      const payload = {
        type: data.type,
        name: data.name,
        base_url: data.base_url || undefined,
        group: data.group,
        models: data.models,
        priority: data.priority ?? undefined,
        cost_price: data.cost_price,
        model_mapping: data.model_mapping || undefined,
        remark: data.remark || undefined,
        status: 1,
        ...(data.key ? { key: data.key } : {}),
      }

      let result
      if (isUpdate && currentRow) {
        result = await updateMyChannel({ ...payload, id: currentRow.id })
      } else {
        if (!data.key) {
          form.setError('key', { message: t('API key is required') })
          return
        }
        result = await addMyChannel({ ...payload, key: data.key })
      }

      if (result.success) {
        toast.success(
          isUpdate
            ? t('Channel updated successfully')
            : t('Channel created successfully')
        )
        onOpenChange(false)
        triggerRefresh()
      } else {
        toast.error(
          result.message ||
            (isUpdate
              ? t('Failed to update channel')
              : t('Failed to create channel'))
        )
      }
    } catch (_error) {
      toast.error(t('An unexpected error occurred'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const typeOptions = CHANNEL_TYPE_OPTIONS

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Update Channel') : t('Create Channel')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate && currentRow?.name
              ? `${t('Update settings for channel')} "${currentRow.name}"`
              : t('Add a new channel for your supplier account.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='my-channel-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            {/* Basic Info */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Basic Information')}</h3>

              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Channel Type')}</FormLabel>
                    <Select
                      items={typeOptions.map((o) => ({
                        value: String(o.value),
                        label: o.label,
                      }))}
                      onValueChange={(v) => field.onChange(Number(v))}
                      value={String(field.value)}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('Select channel type')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {typeOptions.map((option) => (
                            <SelectItem
                              key={option.value}
                              value={String(option.value)}
                            >
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
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('e.g., OpenAI GPT-4 Production')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='base_url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Base URL')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Leave empty to use default')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Auth */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Authentication')}</h3>

              <FormField
                control={form.control}
                name='key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('API Key')}
                      {isUpdate && (
                        <span className='text-muted-foreground ml-1 text-xs'>
                          ({t('Leave blank to keep existing')})
                        </span>
                      )}
                    </FormLabel>
                    <FormControl>
                      <PasswordInput
                        {...field}
                        placeholder={
                          isUpdate
                            ? t('Leave blank to keep existing key')
                            : t('API Key')
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Models */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Models & Group')}</h3>

              <FormField
                control={form.control}
                name='models'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Models')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t(
                          'Comma-separated model names, e.g., gpt-4,gpt-3.5-turbo'
                        )}
                        rows={3}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='default' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Pricing & Priority */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Pricing & Priority')}</h3>

              <FormField
                control={form.control}
                name='cost_price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Cost Price')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        step='0.000001'
                        min='0.000001'
                        placeholder='e.g., 0.002'
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

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
                        placeholder='0'
                        value={field.value ?? ''}
                        onChange={(e) =>
                          field.onChange(
                            e.target.value === ''
                              ? null
                              : Number(e.target.value)
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Advanced */}
            <SideDrawerSection>
              <h3 className='text-sm font-medium'>{t('Advanced')}</h3>

              <FormField
                control={form.control}
                name='model_mapping'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model Mapping')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder='{"request_model": "actual_model"}'
                        rows={3}
                      />
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
                      <Input
                        {...field}
                        placeholder={t('Optional notes about this channel')}
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
          <Button
            form='my-channel-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting
              ? t('Saving...')
              : isUpdate
                ? t('Save changes')
                : t('Create Channel')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
