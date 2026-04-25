import { useQuery } from '@tanstack/react-query';
import { Activity, CheckCircle2, Database, Network, Rss, ShieldCheck, TrafficCone, XCircle } from 'lucide-react';
import { getSingBoxStatus, getSystemHealth } from '../api/system';
import { getTrafficOverview } from '../api/stats';
import { listSubscriptions } from '../api/subscriptions';
import { getNodeSummary } from '../api/nodes';
import { MetricCard } from '../components/charts/MetricCard';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { formatBytes } from '../utils/bytes';
import { compactNumber, formatTime } from '../utils/format';

export function DashboardPage() {
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus });
  const statsQuery = useQuery({ queryKey: ['stats', 'overview'], queryFn: getTrafficOverview });
  const subscriptionsQuery = useQuery({ queryKey: ['subscriptions'], queryFn: listSubscriptions });
  const nodeSummaryQuery = useQuery({ queryKey: ['nodes', 'summary'], queryFn: getNodeSummary });

  const nodeSummary = nodeSummaryQuery.data;
  const subscriptions = subscriptionsQuery.data ?? [];
  const aliveCount = nodeSummary?.alive ?? 0;
  const deadCount = nodeSummary?.dead ?? 0;
  const nodeCount = nodeSummary?.total ?? 0;

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">JnmProxy Console</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">仪表盘</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">查看代理池整体状态、流量概览、节点健康和 sing-box 协议能力。</p>
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard title="连接数" value={compactNumber(statsQuery.data?.connections)} hint="累计连接" icon={<TrafficCone className="h-5 w-5" />} />
        <MetricCard title="成功连接" value={compactNumber(statsQuery.data?.success_connections)} hint="代理转发成功次数" icon={<CheckCircle2 className="h-5 w-5 text-emerald-300" />} />
        <MetricCard title="失败连接" value={compactNumber(statsQuery.data?.failed_connections)} hint="认证或转发失败次数" icon={<XCircle className="h-5 w-5 text-red-300" />} />
        <MetricCard title="订阅数量" value={compactNumber(subscriptions.length)} hint="已保存订阅链接" icon={<Rss className="h-5 w-5" />} />
        <MetricCard title="上传流量" value={formatBytes(statsQuery.data?.upload_bytes)} hint="内存统计会先 flush" icon={<Activity className="h-5 w-5" />} />
        <MetricCard title="下载流量" value={formatBytes(statsQuery.data?.download_bytes)} hint="所有入口统一统计" icon={<Database className="h-5 w-5" />} />
        <MetricCard title="节点数量" value={compactNumber(nodeCount)} hint={`可用 ${aliveCount} / 死亡 ${deadCount}`} icon={<Network className="h-5 w-5" />} />
        <MetricCard title="异常节点" value={compactNumber(deadCount)} hint={`未知 ${nodeSummary?.unknown ?? 0}`} icon={<XCircle className="h-5 w-5 text-amber-300" />} />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <Card>
          <CardHeader title="系统状态" description="API 与 sing-box 实时状态" />
          <div className="space-y-3 text-sm">
            <Row label="API"><Badge value={healthQuery.data?.status === 'ok' ? 'alive' : 'dead'}>{healthQuery.data?.status ?? '加载中'}</Badge></Row>
            <Row label="API 时间"><span className="font-mono text-slate-300">{formatTime(healthQuery.data?.time)}</span></Row>
            <Row label="sing-box"><Badge value={singBoxQuery.data?.enabled ? 'supported' : 'unsupported'}>{singBoxQuery.data?.enabled ? '已启用' : '未启用'}</Badge></Row>
            <Row label="QUIC"><Badge value={singBoxQuery.data?.quic_enabled ? 'supported' : 'unsupported'}>{singBoxQuery.data?.quic_enabled ? '已启用' : '未启用'}</Badge></Row>
          </div>
        </Card>
        <Card>
          <CardHeader title="订阅概览" description="当前保存的订阅链接" />
          <div className="flex items-end gap-3">
            <div className="font-mono text-5xl font-semibold text-white">{subscriptionsQuery.data?.length ?? 0}</div>
            <div className="pb-2 text-sm text-slate-500">条订阅</div>
          </div>
          <div className="mt-5 flex flex-wrap gap-2">
            {subscriptions.slice(0, 6).map((item) => <Badge key={item.id} value={item.last_status}>{item.name}</Badge>)}
          </div>
        </Card>
        <Card>
          <CardHeader title="支持协议" description="由后端 sing-box 状态接口返回" />
          <div className="flex flex-wrap gap-2">
            {(singBoxQuery.data?.supported_protocols ?? []).map((protocol) => <span key={protocol} className="rounded-full border border-blue-400/20 bg-blue-500/10 px-3 py-1 font-mono text-xs text-blue-200">{protocol}</span>)}
          </div>
          <p className="mt-4 text-xs text-slate-500"><ShieldCheck className="mr-1 inline h-3 w-3" />Hysteria2/TUIC 需要 with_quic 构建。</p>
        </Card>
      </div>
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className="flex items-center justify-between gap-4 border-b border-slate-800/70 pb-3 last:border-0 last:pb-0"><span className="text-slate-500">{label}</span>{children}</div>;
}
