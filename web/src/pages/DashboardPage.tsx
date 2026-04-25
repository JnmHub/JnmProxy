import { useQuery } from '@tanstack/react-query';
import { getSingBoxStatus, getSystemHealth } from '../api/system';

export function DashboardPage() {
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus });

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">JnmProxy Console</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">仪表盘</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">第一阶段已接入系统健康和 sing-box 状态，后续按计划继续补订阅、节点、分组、凭证和统计页面。</p>
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        <InfoCard title="API 状态" value={healthQuery.data?.status ?? (healthQuery.isLoading ? '加载中' : '异常')} hint={healthQuery.data?.time ?? healthQuery.error?.message} tone={healthQuery.data?.status === 'ok' ? 'green' : 'red'} />
        <InfoCard title="sing-box" value={singBoxQuery.data?.enabled ? '已启用' : singBoxQuery.isLoading ? '加载中' : '未启用'} hint={singBoxQuery.data?.version ? `版本 ${singBoxQuery.data.version}` : singBoxQuery.error?.message} tone={singBoxQuery.data?.enabled ? 'blue' : 'red'} />
        <InfoCard title="QUIC" value={singBoxQuery.data?.quic_enabled ? '已启用' : '未启用'} hint={singBoxQuery.data?.quic_enabled ? '支持 Hysteria2 / TUIC' : '需要 -tags with_quic'} tone={singBoxQuery.data?.quic_enabled ? 'amber' : 'slate'} />
      </div>
      <section className="rounded-3xl border border-slate-800 bg-slate-950/70 p-6 shadow-glow">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold text-white">支持协议</h2>
            <p className="mt-1 text-sm text-slate-400">来自后端 `/system/sing-box` 的实时状态。</p>
          </div>
        </div>
        <div className="mt-5 flex flex-wrap gap-2">
          {(singBoxQuery.data?.supported_protocols ?? ['等待后端返回']).map((protocol) => (
            <span key={protocol} className="rounded-full border border-blue-400/20 bg-blue-500/10 px-3 py-1 font-mono text-xs text-blue-200">
              {protocol}
            </span>
          ))}
        </div>
      </section>
    </div>
  );
}

function InfoCard({ title, value, hint, tone }: { title: string; value: string; hint?: string; tone: 'green' | 'blue' | 'amber' | 'red' | 'slate' }) {
  const tones = {
    green: 'from-emerald-500/20 to-slate-950 text-emerald-200',
    blue: 'from-blue-500/20 to-slate-950 text-blue-200',
    amber: 'from-amber-500/20 to-slate-950 text-amber-200',
    red: 'from-red-500/20 to-slate-950 text-red-200',
    slate: 'from-slate-600/20 to-slate-950 text-slate-200',
  };
  return (
    <article className={`rounded-3xl border border-slate-800 bg-gradient-to-br p-5 ${tones[tone]}`}>
      <p className="text-sm text-slate-400">{title}</p>
      <div className="mt-3 text-2xl font-semibold text-white">{value}</div>
      {hint ? <p className="mt-3 truncate font-mono text-xs text-slate-400">{hint}</p> : null}
    </article>
  );
}
