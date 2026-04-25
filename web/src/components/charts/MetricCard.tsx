import type { ReactNode } from 'react';

export function MetricCard({ title, value, hint, icon }: { title: string; value: string; hint?: string; icon?: ReactNode }) {
  return (
    <article className="rounded-3xl border border-slate-800 bg-slate-950/70 p-5 shadow-glow">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm text-slate-400">{title}</p>
        {icon ? <div className="text-blue-300">{icon}</div> : null}
      </div>
      <div className="mt-3 font-mono text-2xl font-semibold text-white">{value}</div>
      {hint ? <p className="mt-2 text-xs text-slate-500">{hint}</p> : null}
    </article>
  );
}
