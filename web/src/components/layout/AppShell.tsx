import { Activity, BarChart3, Command, FileText, FolderKanban, Gauge, KeyRound, Loader2, Menu, Network, Rss, Search, ServerCog, Settings, Tags, X } from 'lucide-react';
import type { KeyboardEvent as ReactKeyboardEvent } from 'react';
import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { globalSearch } from '../../api/search';
import { getSingBoxStatus, getSystemHealth } from '../../api/system';
import type { SearchItem } from '../../api/types';
import { Button } from '../ui/Button';

const navItems = [
  { to: '/dashboard', label: '仪表盘', icon: Gauge },
  { to: '/subscriptions', label: '订阅管理', icon: Rss },
  { to: '/nodes', label: '节点管理', icon: Network },
  { to: '/groups', label: '分组管理', icon: FolderKanban },
  { to: '/keyword-groups', label: '关键词分组', icon: Tags },
  { to: '/credentials', label: '凭证管理', icon: KeyRound },
  { to: '/stats', label: '流量统计', icon: BarChart3 },
  { to: '/operation-logs', label: '操作日志', icon: FileText },
  { to: '/system', label: '系统状态', icon: ServerCog },
  { to: '/settings', label: '设置', icon: Settings },
];

export function AppShell() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth, refetchInterval: 30_000 });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus, refetchInterval: 30_000 });

  return (
    <div className="min-h-screen text-slate-100">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 border-r border-slate-800/80 bg-slate-950/85 backdrop-blur-xl lg:block">
        <Brand />
        <SidebarNav />
      </aside>
      {mobileOpen ? (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm lg:hidden">
          <aside className="flex h-full w-80 max-w-[85vw] flex-col border-r border-slate-800 bg-slate-950 shadow-2xl">
            <div className="flex items-center justify-between border-b border-slate-800">
              <Brand compact />
              <Button variant="ghost" className="mr-3 px-2" onClick={() => setMobileOpen(false)} aria-label="关闭菜单">
                <X className="h-4 w-4" />
              </Button>
            </div>
            <SidebarNav onNavigate={() => setMobileOpen(false)} />
          </aside>
        </div>
      ) : null}
      <div className="lg:pl-72">
        <header className="sticky top-0 z-20 border-b border-slate-800/80 bg-slate-950/75 backdrop-blur-xl">
          <div className="flex h-20 items-center justify-between gap-4 px-5 lg:px-8">
            <Button variant="ghost" className="px-2 lg:hidden" onClick={() => setMobileOpen(true)} aria-label="打开菜单">
              <Menu className="h-5 w-5" />
            </Button>
            <GlobalSearchBox />
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

function GlobalSearchBox() {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const deferredQuery = useDeferredValue(query.trim());
  const searchQuery = useQuery({
    queryKey: ['global-search', deferredQuery],
    queryFn: () => globalSearch(deferredQuery),
    enabled: open && deferredQuery.length > 0,
  });
  const items = useMemo(() => searchQuery.data?.items ?? [], [searchQuery.data]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const typing = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable;
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen(true);
        setTimeout(() => inputRef.current?.focus(), 0);
        return;
      }
      if (!typing && event.key === '/') {
        event.preventDefault();
        setOpen(true);
        setTimeout(() => inputRef.current?.focus(), 0);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  useEffect(() => {
    setActiveIndex(0);
  }, [deferredQuery]);

  useEffect(() => {
    setActiveIndex((index) => Math.min(index, Math.max(items.length - 1, 0)));
  }, [items.length]);

  const go = (item: SearchItem) => {
    navigate(item.url);
    setOpen(false);
    setQuery('');
  };

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') {
      setOpen(false);
      inputRef.current?.blur();
      return;
    }
    if (!items.length) return;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((index) => (index + 1) % items.length);
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((index) => (index - 1 + items.length) % items.length);
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const item = items[activeIndex] ?? items[0];
      go(item);
    }
  };

  return (
    <div className="relative hidden min-w-0 flex-1 max-w-2xl md:block">
      <div className="flex items-center gap-3 rounded-2xl border border-slate-800 bg-slate-900/70 px-4 py-2 text-slate-400 transition-colors focus-within:border-blue-400/60 focus-within:ring-2 focus-within:ring-blue-500/20">
        <Search className="h-4 w-4 shrink-0" />
        <input
          ref={inputRef}
          value={query}
          onFocus={() => setOpen(true)}
          onChange={(event) => { setQuery(event.target.value); setOpen(true); }}
          onKeyDown={handleKeyDown}
          placeholder="搜索节点、订阅、分组、凭证、日志"
          className="min-w-0 flex-1 bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-500"
        />
        {searchQuery.isFetching ? <Loader2 className="h-4 w-4 animate-spin text-blue-300" /> : <span className="rounded-lg border border-slate-700 px-2 py-0.5 font-mono text-[10px] text-slate-500">Ctrl K</span>}
      </div>
      {open ? (
        <div className="absolute left-0 right-0 top-12 z-50 overflow-hidden rounded-2xl border border-slate-800 bg-slate-950 shadow-2xl shadow-black/40">
          {deferredQuery ? (
            <div className="max-h-96 overflow-y-auto p-2">
              {items.length ? items.map((item, index) => (
                <button
                  key={`${item.type}-${item.id}`}
                  type="button"
                  onMouseDown={(event) => { event.preventDefault(); go(item); }}
                  onMouseEnter={() => setActiveIndex(index)}
                  className={`flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors ${activeIndex === index ? 'bg-blue-500/15 text-blue-100' : 'text-slate-300 hover:bg-slate-900'}`}
                >
                  <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border border-slate-800 bg-slate-900 text-blue-300">
                    <Command className="h-4 w-4" />
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium">{item.title}</span>
                    <span className="mt-1 block truncate text-xs text-slate-500">{item.subtitle}</span>
                  </span>
                </button>
              )) : (
                <div className="px-4 py-8 text-center text-sm text-slate-500">没有匹配结果</div>
              )}
            </div>
          ) : (
            <div className="px-4 py-5 text-sm text-slate-500">输入关键词搜索节点、订阅、分组、凭证和日志。</div>
          )}
        </div>
      ) : null}
    </div>
  );
}

function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`flex h-20 items-center gap-3 px-6 ${compact ? '' : 'border-b border-slate-800'}`}>
      <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-blue-500/15 text-blue-300 shadow-glow">
        <Activity className="h-6 w-6" />
      </div>
      <div>
        <div className="font-mono text-lg font-semibold tracking-tight">JnmProxy</div>
        <div className="text-xs text-slate-400">代理池控制台</div>
      </div>
    </div>
  );
}

function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <nav className="space-y-1 p-4">
      {navItems.map((item) => {
        const Icon = item.icon;
        return (
          <NavLink
            key={item.to}
            to={item.to}
            onClick={onNavigate}
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
  );
}

function StatusPill({ label, ok, loading, mutedWhenOff = false }: { label: string; ok: boolean; loading?: boolean; mutedWhenOff?: boolean }) {
  const tone = loading ? 'border-slate-700 bg-slate-900 text-slate-400' : ok ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300' : mutedWhenOff ? 'border-slate-700 bg-slate-900 text-slate-400' : 'border-red-400/30 bg-red-500/10 text-red-300';
  return <span className={`rounded-full border px-3 py-1 font-mono ${tone}`}>{label}</span>;
}
