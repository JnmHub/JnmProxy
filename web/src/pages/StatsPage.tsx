import { useQuery } from '@tanstack/react-query';
import { CheckCircle2, Download, Upload, XCircle } from 'lucide-react';
import { getTrafficOverview } from '../api/stats';
import { MetricCard } from '../components/charts/MetricCard';
import { Card, CardHeader } from '../components/ui/Card';
import { formatBytes } from '../utils/bytes';
import { compactNumber } from '../utils/format';

export function StatsPage() {
  const statsQuery = useQuery({ queryKey: ['stats', 'overview'], queryFn: getTrafficOverview });
  const stats = statsQuery.data;
  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Traffic</p><h1 className="mt-2 text-3xl font-semibold text-white">流量统计</h1></div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <MetricCard title="总连接" value={compactNumber(stats?.connections)} />
        <MetricCard title="成功连接" value={compactNumber(stats?.success_connections)} icon={<CheckCircle2 className="h-5 w-5 text-emerald-300" />} />
        <MetricCard title="失败连接" value={compactNumber(stats?.failed_connections)} icon={<XCircle className="h-5 w-5 text-red-300" />} />
        <MetricCard title="上传" value={formatBytes(stats?.upload_bytes)} icon={<Upload className="h-5 w-5 text-blue-300" />} />
        <MetricCard title="下载" value={formatBytes(stats?.download_bytes)} icon={<Download className="h-5 w-5 text-amber-300" />} />
      </div>
      <Card>
        <CardHeader title="趋势图预留" description="后端后续补小时/天趋势接口后，这里展示折线图和节点排行。" />
        <div className="rounded-2xl border border-dashed border-slate-800 p-12 text-center text-sm text-slate-500">当前后端已提供总体统计，趋势图等待后续聚合接口。</div>
      </Card>
    </div>
  );
}
