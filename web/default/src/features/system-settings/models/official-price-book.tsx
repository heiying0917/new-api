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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckSquare, RefreshCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  applyOfficialPriceFill,
  getOfficialPriceBook,
  previewOfficialPriceFill,
  refreshOfficialPriceBook,
} from '../api'
import type {
  OfficialFillMode,
  OfficialFillPreviewResult,
  OfficialFillScopeKind,
  OfficialRatioSet,
} from '../types'

// 1 ratio unit = $0.002 / 1K tokens = $2 / 1M tokens, so input $/1M = ratio * 2.
const RATIO_TO_USD_PER_1M = 2

function formatInputUsd(modelRatio?: number): string {
  if (modelRatio == null) return '-'
  const usd = modelRatio * RATIO_TO_USD_PER_1M
  return `$${Number(usd.toFixed(4))}/1M`
}

function formatRatioSet(
  rs: OfficialRatioSet | null,
  t: (key: string) => string
): string {
  if (!rs || rs.model_ratio == null) return t('Unset price')
  const parts = [`${t('Input')} ${formatInputUsd(rs.model_ratio)}`]
  if (rs.completion_ratio != null)
    parts.push(`${t('Output')} ${Number(rs.completion_ratio.toFixed(4))}×`)
  if (rs.cache_ratio != null)
    parts.push(`${t('Cache')} ${Number(rs.cache_ratio.toFixed(4))}×`)
  if (rs.create_cache_ratio != null)
    parts.push(`${t('Cache write')} ${Number(rs.create_cache_ratio.toFixed(4))}×`)
  return parts.join(' · ')
}

function parseManualModels(raw: string): string[] {
  return raw
    .split(/[\s,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export function OfficialPriceBook() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [scopeKind, setScopeKind] = useState<OfficialFillScopeKind>('all_missing')
  const [manualModels, setManualModels] = useState('')
  const [mode, setMode] = useState<OfficialFillMode>('missing_only')
  const [preview, setPreview] = useState<OfficialFillPreviewResult | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: metaData, isLoading: metaLoading } = useQuery({
    queryKey: ['official-price-book'],
    queryFn: getOfficialPriceBook,
  })
  const meta = metaData?.data

  const refreshMutation = useMutation({
    mutationFn: refreshOfficialPriceBook,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to refresh official price book'))
        return
      }
      toast.success(
        t('Official price book refreshed: {{count}} models', {
          count: data.data.model_count,
        })
      )
      queryClient.invalidateQueries({ queryKey: ['official-price-book'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to refresh official price book'))
    },
  })

  const previewMutation = useMutation({
    mutationFn: previewOfficialPriceFill,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to preview official prices'))
        return
      }
      setPreview(data.data)
      // Pre-select every row that would actually change.
      const next = new Set<string>()
      for (const row of data.data.rows) {
        if (row.action !== 'skip') next.add(row.model)
      }
      setSelected(next)
      if (data.data.rows.length === 0 && data.data.unmatched.length === 0) {
        toast.info(t('No models to fill'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to preview official prices'))
    },
  })

  const applyMutation = useMutation({
    mutationFn: applyOfficialPriceFill,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to apply official prices'))
        return
      }
      toast.success(
        t('Official prices applied to {{count}} models', {
          count: data.data.applied_count,
        })
      )
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      setPreview(null)
      setSelected(new Set())
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to apply official prices'))
    },
  })

  const isBusy =
    refreshMutation.isPending ||
    previewMutation.isPending ||
    applyMutation.isPending

  const handlePreview = () => {
    if (scopeKind === 'models') {
      const models = parseManualModels(manualModels)
      if (models.length === 0) {
        toast.warning(t('Please enter at least one model name'))
        return
      }
      previewMutation.mutate({ scope: { kind: 'models', models }, mode })
      return
    }
    previewMutation.mutate({ scope: { kind: 'all_missing' }, mode })
  }

  const actionableRows = useMemo(
    () => (preview?.rows ?? []).filter((r) => r.action !== 'skip'),
    [preview]
  )

  const allActionableSelected =
    actionableRows.length > 0 &&
    actionableRows.every((r) => selected.has(r.model))

  const toggle = (model: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (checked) next.add(model)
      else next.delete(model)
      return next
    })
  }

  const toggleAll = (checked: boolean) => {
    if (!checked) {
      setSelected(new Set())
      return
    }
    setSelected(new Set(actionableRows.map((r) => r.model)))
  }

  const handleApply = () => {
    const models = [...selected]
    if (models.length === 0) {
      toast.warning(t('Please select at least one model'))
      return
    }
    applyMutation.mutate(models)
  }

  const actionVariant = (
    action: string
  ): 'default' | 'secondary' | 'outline' => {
    if (action === 'add') return 'default'
    if (action === 'update') return 'secondary'
    return 'outline'
  }

  const fetchedLabel = meta?.fetched_at
    ? new Date(meta.fetched_at * 1000).toLocaleString()
    : t('Never refreshed')

  return (
    <div className='space-y-4'>
      {/* Status row */}
      <div className='flex flex-col gap-2 rounded-lg border p-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground space-y-0.5'>
          <div>
            {t('Source')}: <span className='font-medium'>models.dev</span> ·{' '}
            {t('Models')}:{' '}
            <span className='font-medium'>
              {metaLoading ? '…' : (meta?.model_count ?? 0)}
            </span>{' '}
            · {t('First-party')}:{' '}
            <span className='font-medium'>
              {metaLoading ? '…' : (meta?.first_party_count ?? 0)}
            </span>
          </div>
          <div>
            {t('Last refreshed')}: <span className='font-medium'>{fetchedLabel}</span>
          </div>
        </div>
        <Button
          variant='secondary'
          onClick={() => refreshMutation.mutate()}
          disabled={isBusy}
        >
          {refreshMutation.isPending && (
            <span className='mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
          )}
          <RefreshCcw className='mr-2 h-4 w-4' />
          {t('Refresh book')}
        </Button>
      </div>

      {/* Controls */}
      <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
        <label className='flex flex-col gap-1 text-sm'>
          <span className='text-muted-foreground'>{t('Scope')}</span>
          <NativeSelect
            value={scopeKind}
            onChange={(e) =>
              setScopeKind(e.target.value as OfficialFillScopeKind)
            }
          >
            <NativeSelectOption value='all_missing'>
              {t('All unpriced models')}
            </NativeSelectOption>
            <NativeSelectOption value='models'>
              {t('Enter models manually')}
            </NativeSelectOption>
          </NativeSelect>
        </label>

        <label className='flex flex-col gap-1 text-sm'>
          <span className='text-muted-foreground'>{t('Mode')}</span>
          <NativeSelect
            value={mode}
            onChange={(e) => setMode(e.target.value as OfficialFillMode)}
          >
            <NativeSelectOption value='missing_only'>
              {t('Fill missing only')}
            </NativeSelectOption>
            <NativeSelectOption value='refresh_latest'>
              {t('Refresh to latest official')}
            </NativeSelectOption>
          </NativeSelect>
        </label>

        <Button onClick={handlePreview} disabled={isBusy}>
          {previewMutation.isPending && (
            <span className='mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
          )}
          {t('Preview')}
        </Button>
      </div>

      {scopeKind === 'models' && (
        <Textarea
          value={manualModels}
          onChange={(e) => setManualModels(e.target.value)}
          placeholder={t('One model name per line, or comma-separated')}
          rows={3}
        />
      )}

      {/* Preview result */}
      {preview && (
        <div className='space-y-3'>
          <div className='flex items-center justify-between'>
            <div className='text-muted-foreground text-sm'>
              {t('Matched')}: {preview.rows.length} · {t('Unmatched')}:{' '}
              {preview.unmatched.length} · {t('Selected')}: {selected.size}
            </div>
            <Button
              onClick={handleApply}
              disabled={isBusy || selected.size === 0}
            >
              {applyMutation.isPending && (
                <span className='mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
              )}
              <CheckSquare className='mr-2 h-4 w-4' />
              {t('Apply selected')} ({selected.size})
            </Button>
          </div>

          {preview.rows.length > 0 && (
            <div className='rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-10'>
                      <Checkbox
                        checked={allActionableSelected}
                        onCheckedChange={(v) => toggleAll(!!v)}
                        disabled={actionableRows.length === 0}
                      />
                    </TableHead>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead>{t('Source')}</TableHead>
                    <TableHead>{t('Official price')}</TableHead>
                    <TableHead>{t('Current price')}</TableHead>
                    <TableHead>{t('Action')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {preview.rows.map((row) => {
                    const disabled = row.action === 'skip'
                    return (
                      <TableRow key={row.model}>
                        <TableCell>
                          <Checkbox
                            checked={selected.has(row.model)}
                            onCheckedChange={(v) => toggle(row.model, !!v)}
                            disabled={disabled}
                          />
                        </TableCell>
                        <TableCell className='font-medium'>{row.model}</TableCell>
                        <TableCell>
                          <div className='flex items-center gap-1'>
                            <span>{row.provider}</span>
                            {row.first_party ? (
                              <Badge variant='default'>{t('Official')}</Badge>
                            ) : (
                              <Badge variant='outline'>{t('Non-official')}</Badge>
                            )}
                            {row.match_type === 'normalized' && (
                              <Badge variant='secondary'>{t('Fuzzy')}</Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className='text-xs'>
                          {formatRatioSet(row.official, t)}
                        </TableCell>
                        <TableCell className='text-muted-foreground text-xs'>
                          {formatRatioSet(row.current, t)}
                        </TableCell>
                        <TableCell>
                          <Badge variant={actionVariant(row.action)}>
                            {t(
                              row.action === 'add'
                                ? 'Add'
                                : row.action === 'update'
                                  ? 'Update'
                                  : 'No change'
                            )}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {preview.unmatched.length > 0 && (
            <div className='space-y-1'>
              <div className='text-muted-foreground text-sm'>
                {t('Unmatched (no official price, handle manually)')}:
              </div>
              <div className='flex flex-wrap gap-1'>
                {preview.unmatched.map((m) => (
                  <Badge key={m} variant='outline'>
                    {m}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
