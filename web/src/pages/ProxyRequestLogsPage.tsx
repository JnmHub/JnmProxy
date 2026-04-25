import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Search } from 'lucide-react';
import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { listProxyRequestLogs } from '../api/proxyRequestLogs';
import type { ProxyRequestLog } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input, Select } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { PaginationBar } from '../components/ui/Pagination';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function ProxyRequestLogsPage() {
  const [params] = useSearchParams();
  const searchParam = params.get('search') ?? '';
  const [search, setSearch] = useState(searchParam);
  const deferredSearch = useDeferredValue(search.trim());
  const [status, setStatus] = useState('');
  const [entryProtocol, setEntryProtocol] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);

  const filter = useMemo(() => ({
    search: deferredSearch || undefined,
    status: status || undefined,
    entry_protocol: entryProtocol || undefined,
    page,
    page_size: pageSize,
  }), [deferredSearch, entryProtocol, page, pageSize, status]);
  const logsQuery = useQuery({ queryKey: ['proxy-request-logs', filter], queryFn: () => listProxyRequestLogs(filter), refetchInterval: 10_000 });
  const logs = logsQuery.data?.items ?? [];
  const total = logsQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  useEffect(() => {
    setSearch(searchParam);
    setPage(1);
  }, [searchParam]);

  const resetPage = () => setPage(1);

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">Proxy Runtime</p>
        <h1 className="mt-2 text-3xl font-semibold text-white">请求日志</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">记录代理请求最终失败时的入口协议、凭证、目标、尝试节点和失败原因，用来排查为什么这次请求没成功。</p>
      </div>

      <Card>
        <CardHeader title="日志筛选" description="默认只记录失败请求，避免成功请求过多写库。支持按目标、节点、凭证和错误关键词搜索。" />
        <div className="grid gap-3 md:grid-cols-3">
          <Field label="搜索">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
              <Input className="pl-9" value={search} onChange={(event) => { setSearch(event.target.value); resetPage(); }} placeholder="目标 / 凭证 / 节点 / 错误" />
            </div>
          </Field>
          <Field label="入口协议">
            <Select value={entryProtocol} onChange={(event) => { setEntryProtocol(event.target.value); resetPage(); }}>
              <option value="">全部协议</option>
              <option value="HTTP">HTTP</option>
              <option value="SOCKS5">SOCKS5</option>
            </Select>
          </Field>
          <Field label="状态">
            <Select value={status} onChange={(event) => { setStatus(event.target.value); resetPage(); }}>
              <option value="">全部状态</option>
              <option value="failed">失败</option>
              <option value="success">成功</option>
            </Select>
          </Field>
        </div>
      </Card>

      <Card>
        <CardHeader title="代理请求失败记录" description={`共 ${total} 条记录，当前页 ${logs.length} 条。`} />
        {logsQuery.isLoading ? <LoadingState /> : (
          <>
            <DataTable columns={['时间', '入口', '凭证', '目标', '状态', '最终节点', '耗时', '失败原因', '尝试详情']} empty={!logs.length}>
              {logs.map((log) => <ProxyRequestLogRow key={log.id} log={log} />)}
            </DataTable>
            <PaginationBar
              page={page}
              pageSize={pageSize}
              total={total}
              totalPages={totalPages}
              onPageChange={setPage}
              onPageSizeChange={(nextPageSize) => { setPageSize(nextPageSize); setPage(1); }}
            />
          </>
        )}
      </Card>
    </div>
  );
}

function ProxyRequestLogRow({ log }: { log: ProxyRequestLog }) {
  return (
    <tr className="hover:bg-slate-900/50">
      <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-400">{formatTime(log.created_at)}</td>
      <td className="px-4 py-3"><Badge value="unknown">{log.entry_protocol || '—'}</Badge></td>
      <td className="px-4 py-3 text-xs text-slate-400">
        <div className="font-mono text-slate-300">{log.username || '—'}</div>
        <div className="mt-1 font-mono">ID {log.credential_id || '—'}</div>
      </td>
      <td className="px-4 py-3 font-mono text-xs text-slate-300">{log.target_address || '—'}</td>
      <td className="px-4 py-3"><Badge value={log.status}>{statusLabel(log.status)}</Badge></td>
      <td className="px-4 py-3 text-xs text-slate-400">
        <div>{log.selected_node_name || '—'}</div>
        <div className="mt-1 font-mono">ID {log.selected_node_id || '—'}</div>
      </td>
      <td className="px-4 py-3 font-mono text-xs text-slate-400">{log.duration_ms}ms</td>
      <td className="max-w-xs px-4 py-3 text-xs text-red-200">{log.error || '—'}</td>
      <td className="max-w-md px-4 py-3 text-xs text-slate-400">
        <div className="mb-1 flex items-center gap-1 text-amber-300"><AlertTriangle className="h-3.5 w-3.5" />尝试 {log.attempt_count} 次</div>
        <div className="font-mono text-slate-500">{attemptPreview(log.attempts_json)}</div>
      </td>
    </tr>
  );
}

function attemptPreview(value: string) {
  if (!value || value === '[]') return '—';
  try {
    const attempts = JSON.parse(value) as Array<{ node_id?: number; node_name?: string; success?: boolean; error?: string }>;
    const text = attempts.map((attempt, index) => {
      const name = attempt.node_name || `节点 ${attempt.node_id ?? index + 1}`;
      return `${index + 1}. ${name} ${attempt.success ? '成功' : `失败：${attempt.error || '未知错误'}`}`;
    }).join('；');
    return text.length > 180 ? `${text.slice(0, 180)}...` : text;
  } catch {
    return value.length > 180 ? `${value.slice(0, 180)}...` : value;
  }
}

function statusLabel(status: string) {
  if (status === 'failed') return '失败';
  if (status === 'success') return '成功';
  return status || '未知';
}
