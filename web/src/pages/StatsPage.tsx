import { useQuery } from '@tanstack/react-query';
import { CheckCircle2, Download, Upload, XCircle } from 'lucide-react';
import { Bar, BarChart, CartesianGrid, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { getTrafficOverview } from '../api/stats';
import { MetricCard } from '../components/charts/MetricCard';
import { Card, CardHeader } from '../components/ui/Card';
import { LoadingState } from '../components/ui/LoadingState';
import { formatBytes } from '../utils/bytes';
import { compactNumber } from '../utils/format';

export function StatsPage() {
  const statsQuery = useQuery({ queryKey: ['stats', 'overview'], queryFn: getTrafficOverview });
  const stats = statsQuery.data;
  const trafficData = [
    { name: '上传', value: stats?.upload_bytes ?? 0 },
    { name: '下载', value: stats?.download_bytes ?? 0 },
  ];
  const connectionData = [
    { name: '成功', value: stats?.success_connections ?? 0, color: '#22c55e' },
    { name: '失败', value: stats?.failed_connections ?? 0, color: '#ef4444' },
  ];
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
      {statsQuery.isLoading ? <LoadingState /> : (
        <div className="grid gap-4 xl:grid-cols-2">
          <Card>
            <CardHeader title="上传 / 下载" description="当前总体流量对比。" />
            <div className="h-72">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={trafficData}>
                  <CartesianGrid stroke="#1e293b" strokeDasharray="3 3" />
                  <XAxis dataKey="name" stroke="#94a3b8" />
                  <YAxis stroke="#94a3b8" tickFormatter={(value) => formatBytes(Number(value))} />
                  <Tooltip formatter={(value) => formatBytes(Number(value))} contentStyle={{ background: '#020617', border: '1px solid #1e293b', borderRadius: 12 }} />
                  <Bar dataKey="value" radius={[10, 10, 0, 0]} fill="#3b82f6" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </Card>
          <Card>
            <CardHeader title="成功 / 失败" description="当前连接结果比例。" />
            <div className="h-72">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={connectionData} dataKey="value" nameKey="name" innerRadius={70} outerRadius={105} paddingAngle={4}>
                    {connectionData.map((entry) => <Cell key={entry.name} fill={entry.color} />)}
                  </Pie>
                  <Tooltip contentStyle={{ background: '#020617', border: '1px solid #1e293b', borderRadius: 12 }} />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}
