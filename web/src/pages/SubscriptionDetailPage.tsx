import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import { listSubscriptionLogs, listSubscriptionNodes, getSubscription, refreshSubscription } from '../api/subscriptions';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { LoadingState } from '../components/ui/LoadingState';
import { DataTable } from '../components/ui/Table';
import { formatBytes, usagePercent } from '../utils/bytes';
import { formatTime, maskURL } from '../utils/format';

export function SubscriptionDetailPage() {
  const id = Number(useParams().id);
  const queryClient = useQueryClient();
  const subscriptionQuery = useQuery({ queryKey: ['subscription', id], queryFn: () => getSubscription(id), enabled: id > 0 });
  const logsQuery = useQuery({ queryKey: ['subscription', id, 'logs'], queryFn: () => listSubscriptionLogs(id), enabled: id > 0 });
  const nodesQuery = useQuery({ queryKey: ['subscription', id, 'nodes'], queryFn: () => listSubscriptionNodes(id), enabled: id > 0 });
  const refreshMutation = useMutation({
    mutationFn: () => refreshSubscription(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['subscription', id] });
      void queryClient.invalidateQueries({ queryKey: ['subscription', id, 'logs'] });
      void queryClient.invalidateQueries({ queryKey: ['subscription', id, 'nodes'] });
      void queryClient.invalidateQueries({ queryKey: ['subscriptions'] });
    },
  });
  const subscription = subscriptionQuery.data;
  const nodes = nodesQuery.data ?? [];
  const nodeStats = useMemo(() => ({
    total: nodes.length,
    alive: nodes.filter((node) => node.alive_status === 'alive').length,
    dead: nodes.filter((node) => node.alive_status === 'dead').length,
    supported: nodes.filter((node) => node.sing_box_status === 'supported').length,
    error: nodes.filter((node) => node.sing_box_status === 'error').length,
  }), [nodes]);
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div><Link className="text-sm text-blue-300" to="/subscriptions">返回订阅</Link><h1 className="mt-2 text-3xl font-semibold text-white">{subscription?.name ?? '订阅详情'}</h1><p className="mt-2 font-mono text-sm text-slate-500">{subscription?.url ? maskURL(subscription.url) : '加载中'}</p></div>
        <Button onClick={() => refreshMutation.mutate()} disabled={refreshMutation.isPending || id <= 0}><RefreshCw className="h-4 w-4" />立即刷新</Button>
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <SummaryCard title="节点数量" value={nodeStats.total} hint="当前订阅保存节点" />
        <SummaryCard title="可用节点" value={nodeStats.alive} hint="alive 状态节点" tone="success" />
        <SummaryCard title="死亡节点" value={nodeStats.dead} hint="健康检查失败" tone="danger" />
        <SummaryCard title="sing-box 支持" value={nodeStats.supported} hint="可走复杂协议适配" tone="success" />
        <SummaryCard title="转换错误" value={nodeStats.error} hint="查看节点错误原因" tone="danger" />
      </div>
      <Card>
        <CardHeader title="订阅信息" description="这条订阅的用量、到期和刷新状态。" />
        {subscriptionQuery.isLoading ? <LoadingState /> : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <Info label="启用状态"><Badge value={subscription?.enabled ? 'supported' : 'unsupported'}>{subscription?.enabled ? '启用' : '禁用'}</Badge></Info>
            <Info label="最近状态"><Badge value={subscription?.last_status}>{subscription?.last_status ?? '—'}</Badge></Info>
            <Info label="到期时间" value={formatTime(subscription?.expire_at)} />
            <Info label="刷新间隔" value={`${subscription?.refresh_interval_seconds ?? 0}s`} />
            <Info label="已用流量" value={formatBytes((subscription?.upload_bytes ?? 0) + (subscription?.download_bytes ?? 0))} />
            <Info label="总流量" value={formatBytes(subscription?.total_bytes)} />
            <Info label="最近刷新" value={formatTime(subscription?.last_refresh_at)} />
            <Info label="下次刷新" value={formatTime(subscription?.next_refresh_at)} />
          </div>
        )}
        {subscription ? <div className="mt-4 h-2 overflow-hidden rounded-full bg-slate-800"><div className="h-full bg-blue-500" style={{ width: `${usagePercent((subscription.upload_bytes ?? 0) + (subscription.download_bytes ?? 0), subscription.total_bytes)}%` }} /></div> : null}
        {subscription?.last_error ? <div className="mt-4 rounded-2xl border border-red-400/30 bg-red-500/10 p-4 text-sm leading-6 text-red-200">{subscription.last_error}</div> : null}
      </Card>
      <Card>
        <CardHeader title="刷新日志" description="包含 sing-box 转换统计。" />
        {logsQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['状态', 'HTTP', '节点', 'sing-box', '错误', '开始/完成']} empty={!logsQuery.data?.length}>
            {(logsQuery.data ?? []).map((log) => <tr key={log.id}><td className="px-4 py-3"><Badge value={log.status} /></td><td className="px-4 py-3 font-mono">{log.http_status ?? '—'}</td><td className="px-4 py-3 font-mono">{log.node_count}</td><td className="px-4 py-3 text-xs text-slate-400"><div className="text-emerald-300">支持 {log.sing_box_supported_count}</div><div className="text-red-300">错误 {log.sing_box_error_count}</div><div>不支持 {log.unsupported_count}</div></td><td className="max-w-xs px-4 py-3 text-xs text-red-300">{log.error ?? '—'}</td><td className="px-4 py-3 text-xs text-slate-400"><div>{formatTime(log.started_at)}</div><div className="mt-1">{formatTime(log.finished_at)}</div></td></tr>)}
          </DataTable>
        )}
      </Card>
      <Card>
        <CardHeader title="订阅节点" description="该订阅当前保存的节点。" />
        {nodesQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['名称', '协议', '健康', 'sing-box', '延迟', '服务器']} empty={!nodes.length}>
            {nodes.map((node) => <tr key={node.id}><td className="px-4 py-3"><Link className="text-blue-200" to={`/nodes?subscription_id=${id}`}>{node.name}</Link></td><td className="px-4 py-3 font-mono">{node.protocol}</td><td className="px-4 py-3"><Badge value={node.alive_status} /></td><td className="px-4 py-3"><Badge value={node.sing_box_status} /></td><td className="px-4 py-3 font-mono text-xs">{node.latency_ms ? `${node.latency_ms}ms` : '—'}</td><td className="px-4 py-3 font-mono text-xs text-slate-400">{node.server}:{node.port}</td></tr>)}
          </DataTable>
        )}
      </Card>
    </div>
  );
}

function SummaryCard({ title, value, hint, tone = 'default' }: { title: string; value: number; hint: string; tone?: 'default' | 'success' | 'danger' }) {
  const color = tone === 'success' ? 'text-emerald-200' : tone === 'danger' ? 'text-red-200' : 'text-white';
  return (
    <Card>
      <div className={`font-mono text-3xl font-semibold ${color}`}>{value}</div>
      <div className="mt-2 text-sm font-medium text-slate-200">{title}</div>
      <div className="mt-1 text-xs text-slate-500">{hint}</div>
    </Card>
  );
}

function Info({ label, value, children }: { label: string; value?: string; children?: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/40 p-4">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 font-mono text-sm text-slate-200">{children ?? value ?? '—'}</div>
    </div>
  );
}
