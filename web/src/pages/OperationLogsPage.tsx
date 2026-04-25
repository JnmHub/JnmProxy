import { useQuery } from '@tanstack/react-query';
import { Search } from 'lucide-react';
import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { listOperationLogs } from '../api/operationLogs';
import type { OperationLog } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input, Select } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { PaginationBar } from '../components/ui/Pagination';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function OperationLogsPage() {
  const [params] = useSearchParams();
  const searchParam = params.get('search') ?? '';
  const [search, setSearch] = useState(searchParam);
  const deferredSearch = useDeferredValue(search.trim());
  const [action, setAction] = useState('');
  const [targetType, setTargetType] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);

  const filter = useMemo(() => ({
    action: action || undefined,
    target_type: targetType || undefined,
    search: deferredSearch || undefined,
    page,
    page_size: pageSize,
  }), [action, deferredSearch, page, pageSize, targetType]);
  const logsQuery = useQuery({ queryKey: ['operation-logs', filter], queryFn: () => listOperationLogs(filter) });
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

  const resetPage = () => {
    setPage(1);
  };

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">Audit</p>
        <h1 className="mt-2 text-3xl font-semibold text-white">操作日志</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">记录订阅刷新、凭证变更、节点批量操作和关键词分组，方便回看是谁、什么时候、做了什么。</p>
      </div>

      <Card>
        <CardHeader title="日志筛选" description="服务端分页和搜索，节点多、操作多时也不会一次拉全量日志。" />
        <div className="grid gap-3 md:grid-cols-3">
          <Field label="搜索">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
              <Input className="pl-9" value={search} onChange={(event) => { setSearch(event.target.value); resetPage(); }} placeholder="动作 / 对象 / 描述 / IP" />
            </div>
          </Field>
          <Field label="动作">
            <Select value={action} onChange={(event) => { setAction(event.target.value); resetPage(); }}>
              <option value="">全部动作</option>
              {operationActions.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
            </Select>
          </Field>
          <Field label="对象">
            <Select value={targetType} onChange={(event) => { setTargetType(event.target.value); resetPage(); }}>
              <option value="">全部对象</option>
              <option value="subscription">订阅</option>
              <option value="node">节点</option>
              <option value="group">分组</option>
              <option value="group_keyword">关键词规则</option>
              <option value="credential">凭证</option>
            </Select>
          </Field>
        </div>
      </Card>

      <Card>
        <CardHeader title="审计记录" description={`共 ${total} 条记录，当前页 ${logs.length} 条。`} />
        {logsQuery.isLoading ? <LoadingState /> : (
          <>
            <DataTable columns={['时间', '动作', '对象', '操作者', '说明', '详情']} empty={!logs.length}>
              {logs.map((log) => <OperationLogRow key={log.id} log={log} />)}
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

function OperationLogRow({ log }: { log: OperationLog }) {
  return (
    <tr className="hover:bg-slate-900/50">
      <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-400">{formatTime(log.created_at)}</td>
      <td className="px-4 py-3"><Badge value={actionTone(log.action)}>{actionLabel(log.action)}</Badge></td>
      <td className="px-4 py-3 text-xs text-slate-400">
        <div>{targetLabel(log.target_type)}</div>
        <div className="mt-1 font-mono">ID {log.target_id || '—'}</div>
      </td>
      <td className="px-4 py-3 text-xs text-slate-400">
        <div className="font-mono text-slate-300">{log.actor || '—'}</div>
        <div className="mt-1 font-mono">{log.ip || '—'}</div>
      </td>
      <td className="px-4 py-3 text-slate-300">{log.message || '—'}</td>
      <td className="max-w-md px-4 py-3 font-mono text-xs text-slate-500">{detailPreview(log.detail_json)}</td>
    </tr>
  );
}

function detailPreview(value: string) {
  if (!value || value === '{}') return '—';
  try {
    const parsed = JSON.parse(value) as Record<string, unknown>;
    const text = Object.entries(parsed).map(([key, item]) => `${key}: ${String(item)}`).join('，');
    return text.length > 120 ? `${text.slice(0, 120)}...` : text;
  } catch {
    return value.length > 120 ? `${value.slice(0, 120)}...` : value;
  }
}

function actionTone(action: string) {
  if (action.includes('delete') || action.includes('disable')) return 'error';
  if (action.includes('refresh') || action.includes('check')) return 'alive';
  if (action.includes('create') || action.includes('add')) return 'supported';
  return 'unknown';
}

function actionLabel(action: string) {
  return operationActions.find((item) => item.value === action)?.label ?? action;
}

function targetLabel(targetType: string) {
  const labels: Record<string, string> = {
    subscription: '订阅',
    node: '节点',
    group: '分组',
    group_keyword: '关键词规则',
    credential: '凭证',
  };
  return labels[targetType] ?? targetType;
}

const operationActions = [
  { value: 'subscription.create', label: '创建订阅' },
  { value: 'subscription.update', label: '更新订阅' },
  { value: 'subscription.delete', label: '删除订阅' },
  { value: 'subscription.refresh', label: '刷新订阅' },
  { value: 'node.batch.enable', label: '批量启用节点' },
  { value: 'node.batch.disable', label: '批量禁用节点' },
  { value: 'node.check_all', label: '全部健康检查' },
  { value: 'node.check', label: '检查节点' },
  { value: 'node.rebuild_adapter', label: '重建适配器' },
  { value: 'group.create', label: '创建分组' },
  { value: 'group.update', label: '更新分组' },
  { value: 'group.delete', label: '删除分组' },
  { value: 'group.add_nodes', label: '分组加节点' },
  { value: 'group.remove_nodes', label: '分组移节点' },
  { value: 'keyword.apply', label: '执行关键词分组' },
  { value: 'credential.create', label: '创建凭证' },
  { value: 'credential.update', label: '更新凭证' },
  { value: 'credential.reset_password', label: '重置密码' },
  { value: 'credential.delete', label: '删除凭证' },
];
