import { Activity, BarChart3, FolderKanban, Gauge, KeyRound, Network, Rss, Search, ServerCog, Settings, Tags } from 'lucide-react';
import { NavLink, Outlet } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getSingBoxStatus, getSystemHealth } from '../../api/system';

const navItems = [
  { to: '/dashboard', label: '仪表盘', icon: Gauge },
  { to: '/subscriptions', label: '订阅管理', icon: Rss },
  { to: '/nodes', label: '节点管理', icon: Network },
  { to: '/groups', label: '分组管理', icon: FolderKanban },
  { to: '/keyword-groups', label: '关键词分组', icon: Tags },
  { to: '/credentials', label: '凭证管理', icon: KeyRound },
  { to: '/stats', label: '流量统计', icon: BarChart3 },
  { to: '/system', label: '系统状态', icon: ServerCog },
  { to: '/settings', label: '设置', icon: Settings },
];

export function AppShell() {
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth, refetchInterval: 30_000 });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus, refetchInterval: 30_000 });

  return (
    <div className="min-h-screen text-slate-100">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 border-r border-slate-800/80 bg-slate-950/85 backdrop-blur-xl lg:block">
        <div className="flex h-20 items-center gap-3 border-b border-slate-800 px-6">
          <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-blue-500/15 text-blue-300 shadow-glow">
            <Activity className="h-6 w-6" />
          </div>
          <div>
            <div className="font-mono text-lg font-semibold tracking-tight">JnmProxy</div>
            <div className="text-xs text-slate-400">代理池控制台</div>
          </div>
        </div>
        <nav className="space-y-1 p-4">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-xl px-4 py-3 text-sm transition-colors duration-200 ${
                    isActive ? 'bg-blue-500/15 text-blue-200 ring-1 ring-blue-400/20' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-100'
                  }`
                }
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            );
          })}
        </nav>
      </aside>
      <div className="lg:pl-72">
        <header className="sticky top-0 z-20 border-b border-slate-800/80 bg-slate-950/75 backdrop-blur-xl">
          <div className="flex h-20 items-center justify-between gap-4 px-5 lg:px-8">
            <div className="hidden min-w-0 items-center gap-3 rounded-2xl border border-slate-800 bg-slate-900/70 px-4 py-2 text-slate-400 md:flex">
              <Search className="h-4 w-4" />
              <span className="text-sm">搜索节点、订阅、分组（后续接入）</span>
            </div>
            <div className="flex flex-1 items-center gap-3 lg:hidden">
              <Activity className="h-6 w-6 text-blue-300" />
              <span className="font-mono font-semibold">JnmProxy</span>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <StatusPill label="API" ok={healthQuery.data?.status === 'ok'} loading={healthQuery.isLoading} />
              <StatusPill label="sing-box" ok={Boolean(singBoxQuery.data?.enabled)} loading={singBoxQuery.isLoading} />
              <StatusPill label="QUIC" ok={Boolean(singBoxQuery.data?.quic_enabled)} loading={singBoxQuery.isLoading} mutedWhenOff />
            </div>
          </div>
        </header>
        <main className="px-5 py-6 lg:px-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function StatusPill({ label, ok, loading, mutedWhenOff = false }: { label: string; ok: boolean; loading?: boolean; mutedWhenOff?: boolean }) {
  const tone = loading ? 'border-slate-700 bg-slate-900 text-slate-400' : ok ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300' : mutedWhenOff ? 'border-slate-700 bg-slate-900 text-slate-400' : 'border-red-400/30 bg-red-500/10 text-red-300';
  return <span className={`rounded-full border px-3 py-1 font-mono ${tone}`}>{label}</span>;
}
