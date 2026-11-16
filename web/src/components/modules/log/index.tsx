'use client';

import { useCallback, useMemo, useState, type FormEvent } from 'react';
import { type LogFilters, useClearLogs, useLogs } from '@/api/endpoints/log';
import { LogCard } from './Item';
import { ChevronDown, ChevronUp, Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/common/Toast';

type FilterKey = keyof LogFilters;
type FilterValues = Record<FilterKey, string>;
const emptyFilters: FilterValues = {
  start_time: '', end_time: '', min_input_tokens: '', max_input_tokens: '',
  min_output_tokens: '', max_output_tokens: '', min_ftut: '', max_ftut: '',
  min_use_time: '', max_use_time: '', channel_id: '', request_model_name: '',
  actual_model_name: '', status: '',
};
const numericKeys = ['min_input_tokens', 'max_input_tokens', 'min_output_tokens', 'max_output_tokens', 'min_ftut', 'max_ftut', 'min_use_time', 'max_use_time', 'channel_id'] as const;

function toLogFilters(values: FilterValues): LogFilters | null {
  const filters: LogFilters = {};
  for (const key of numericKeys) {
    if (!values[key]) continue;
    const value = Number(values[key]);
    if (!Number.isInteger(value) || value < 0) return null;
    filters[key] = value;
  }
  for (const key of ['start_time', 'end_time'] as const) {
    if (!values[key]) continue;
    const value = Date.parse(values[key]);
    if (Number.isNaN(value)) return null;
    filters[key] = Math.floor(value / 1000);
  }
  if (filters.start_time !== undefined && filters.end_time !== undefined && filters.start_time > filters.end_time) return null;
  for (const [min, max] of [[filters.min_input_tokens, filters.max_input_tokens], [filters.min_output_tokens, filters.max_output_tokens], [filters.min_ftut, filters.max_ftut], [filters.min_use_time, filters.max_use_time]]) {
    if (min !== undefined && max !== undefined && min > max) return null;
  }
  if (values.request_model_name.trim()) filters.request_model_name = values.request_model_name.trim();
  if (values.actual_model_name.trim()) filters.actual_model_name = values.actual_model_name.trim();
  if (values.status === 'success' || values.status === 'failed') filters.status = values.status;
  return filters;
}

function Range({ label, minKey, maxKey, values, change, ms = false }: { label: string; minKey: FilterKey; maxKey: FilterKey; values: FilterValues; change: (key: FilterKey, value: string) => void; ms?: boolean }) {
  return <div className='grid gap-1.5'><span className='text-xs font-medium text-muted-foreground'>{label}{ms ? ' (ms)' : ''}</span><div className='grid grid-cols-2 gap-2'><Input type='number' min='0' step='1' value={values[minKey]} onChange={(e) => change(minKey, e.target.value)} placeholder='Min' /><Input type='number' min='0' step='1' value={values[maxKey]} onChange={(e) => change(maxKey, e.target.value)} placeholder='Max' /></div></div>;
}

export function Log() {
  const tt = useTranslations('setting');
  const t = useTranslations('log');
  const [draft, setDraft] = useState<FilterValues>(emptyFilters);
  const [filters, setFilters] = useState<LogFilters>({});
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filterError, setFilterError] = useState(false);
  const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ pageSize: 10, filters });
  const [isClearing, setIsClearing] = useState(false);
  const clearLogs = useClearLogs();
  const change = useCallback((key: FilterKey, value: string) => { setDraft((current) => ({ ...current, [key]: value })); setFilterError(false); }, []);
  const apply = useCallback((event: FormEvent) => { event.preventDefault(); const next = toLogFilters(draft); if (!next) return setFilterError(true); setFilters(next); }, [draft]);
  const reset = useCallback(() => { setDraft(emptyFilters); setFilters({}); setFilterError(false); }, []);
  const clear = () => { setIsClearing(true); clearLogs.mutate(undefined, { onSuccess: () => { toast.success(tt('log.clearSuccess')); setIsClearing(false); }, onError: () => { toast.error(tt('log.clearFailed')); setIsClearing(false); } }); };
  const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
  const reachEnd = useCallback(() => { if (canLoadMore) void loadMore(); }, [canLoadMore, loadMore]);
  const footer = useMemo(() => {
    if (hasMore && (isLoading || isLoadingMore)) return <div className='flex justify-center py-4'><Loader2 className='h-6 w-6 animate-spin text-muted-foreground' /></div>;
    if (!hasMore && logs.length > 0) return <div className='flex justify-center py-4'><span className='text-sm text-muted-foreground'>{t('list.noMore')}</span></div>;
    return null;
  }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

  return <div className='flex h-full min-h-0 flex-col gap-3'>
    <section className='shrink-0 rounded-2xl border bg-card'>
      <div className='flex flex-wrap items-center justify-between gap-3 p-4'>
        <h2 className='text-sm font-semibold'>{t('filters.title')}</h2>
        <Button type='button' variant='outline' size='sm' onClick={() => setIsFiltersOpen((open) => !open)} aria-expanded={isFiltersOpen}>
          {isFiltersOpen ? <ChevronUp className='size-4' /> : <ChevronDown className='size-4' />}
          {isFiltersOpen ? t('filters.collapse') : t('filters.expand')}
        </Button>
      </div>
      {isFiltersOpen && <form onSubmit={apply} className='max-h-[42dvh] space-y-4 overflow-y-auto border-t p-4 overscroll-contain'>
        <div className='flex justify-end gap-2'><Button type='button' variant='outline' size='sm' onClick={reset}>{t('filters.reset')}</Button><Button type='submit' size='sm'>{t('filters.apply')}</Button></div>
        <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4'>
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.startTime')}<Input type='datetime-local' value={draft.start_time} onChange={(e) => change('start_time', e.target.value)} /></label>
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.endTime')}<Input type='datetime-local' value={draft.end_time} onChange={(e) => change('end_time', e.target.value)} /></label>
        <Range label={t('filters.inputTokens')} minKey='min_input_tokens' maxKey='max_input_tokens' values={draft} change={change} />
        <Range label={t('filters.outputTokens')} minKey='min_output_tokens' maxKey='max_output_tokens' values={draft} change={change} />
        <Range label={t('filters.firstToken')} minKey='min_ftut' maxKey='max_ftut' values={draft} change={change} ms />
        <Range label={t('filters.totalTime')} minKey='min_use_time' maxKey='max_use_time' values={draft} change={change} ms />
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.channelId')}<Input type='number' min='0' step='1' value={draft.channel_id} onChange={(e) => change('channel_id', e.target.value)} /></label>
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.status')}<select className='h-9 rounded-md border border-input bg-transparent px-3 text-sm' value={draft.status} onChange={(e) => change('status', e.target.value)}><option value=''>{t('filters.all')}</option><option value='success'>{t('filters.success')}</option><option value='failed'>{t('filters.failed')}</option></select></label>
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.requestModel')}<Input value={draft.request_model_name} onChange={(e) => change('request_model_name', e.target.value)} placeholder={t('filters.exactMatch')} /></label>
        <label className='grid gap-1.5 text-xs font-medium text-muted-foreground'>{t('filters.actualModel')}<Input value={draft.actual_model_name} onChange={(e) => change('actual_model_name', e.target.value)} placeholder={t('filters.exactMatch')} /></label>
        </div>
        {filterError && <p className='text-xs text-destructive'>{t('filters.invalidRange')}</p>}
      </form>}
    </section>
    <div className='flex justify-end'><Button variant='destructive' size='sm' onClick={clear} disabled={isClearing} className='rounded-xl'>{isClearing ? tt('log.clear.clearing') : tt('log.clear.button')}</Button></div>
    <div className='min-h-0 flex-1'><VirtualizedGrid items={logs} layout='list' columns={{ default: 1 }} estimateItemHeight={80} overscan={8} getItemKey={(log) => 'log-' + log.id} renderItem={(log) => <LogCard log={log} />} footer={footer} onReachEnd={reachEnd} reachEndEnabled={canLoadMore} reachEndOffset={2} /></div>
  </div>;
}
