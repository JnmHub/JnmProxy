import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { listSubscriptionLogs, listSubscriptionNodes, getSubscription } from '../api/subscriptions';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function SubscriptionDetailPage() {
  const id = Number(useParams().id);
  const subscriptionQuery = useQuery({ queryKey: ['subscription', id], queryFn: () => getSubscription(id), enabled: id > 0 });
  const logsQuery = useQuery({ queryKey: ['subscription', id, 'logs'], queryFn: () => listSubscriptionLogs(id), enabled: id > 0 });
  const nodesQuery = useQuery({ queryKey: ['subscription', id, 'nodes'], queryFn: () => listSubscriptionNodes(id), enabled: id > 0 });
  const subscription = subscriptionQuery.data;
  return (
    <div className="space-y-6">
      <div><Link className="text-sm text-blue-300" to="/subscriptions">返回订阅</Link><h1 className="mt-2 text-3xl font-semibold text-white">{subscription?.name ?? '订阅详情'}</h1><p className="mt-2 font-mono text-sm text-slate-500">{subscription?.url}</p></div>
      <Card>
        <CardHeader title="刷新日志" description="包含 sing-box 转换统计。" />
        <DataTable columns={['状态', 'HTTP', '节点', 'sing-box', '错误', '时间']} empty={!logsQuery.data?.length}>
          {(logsQuery.data ?? []).map((log) => <tr key={log.id}><td className="px-4 py-3"><Badge value={log.status} /></td><td className="px-4 py-3 font-mono">{log.http_status ?? '—'}</td><td className="px-4 py-3 font-mono">{log.node_count}</td><td className="px-4 py-3 text-xs text-slate-400">支持 {log.sing_box_supported_count} / 错误 {log.sing_box_error_count} / 不支持 {log.unsupported_count}</td><td className="max-w-xs truncate px-4 py-3 text-xs text-red-300">{log.error ?? '—'}</td><td className="px-4 py-3 text-xs text-slate-400">{formatTime(log.started_at)}</td></tr>)}
        </DataTable>
      </Card>
      <Card>
        <CardHeader title="订阅节点" description="该订阅当前保存的节点。" />
        <DataTable columns={['名称', '协议', '健康', 'sing-box', '服务器']} empty={!nodesQuery.data?.length}>
          {(nodesQuery.data ?? []).map((node) => <tr key={node.id}><td className="px-4 py-3"><Link className="text-blue-200" to={`/nodes?subscription_id=${id}`}>{node.name}</Link></td><td className="px-4 py-3 font-mono">{node.protocol}</td><td className="px-4 py-3"><Badge value={node.alive_status} /></td><td className="px-4 py-3"><Badge value={node.sing_box_status} /></td><td className="px-4 py-3 font-mono text-xs text-slate-400">{node.server}:{node.port}</td></tr>)}
        </DataTable>
      </Card>
    </div>
  );
}
