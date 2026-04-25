import type { PropsWithChildren, ReactNode } from 'react';
import { cx } from '../../utils/status';

export function Card({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <section className={cx('rounded-3xl border border-slate-800 bg-slate-950/70 p-5 shadow-glow', className)}>{children}</section>;
}

export function CardHeader({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div>
        <h2 className="text-lg font-semibold text-white">{title}</h2>
        {description ? <p className="mt-1 text-sm text-slate-400">{description}</p> : null}
      </div>
      {action ? <div className="flex flex-wrap items-center gap-2">{action}</div> : null}
    </div>
  );
}
