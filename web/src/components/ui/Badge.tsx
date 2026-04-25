import { cx, statusLabel } from '../../utils/status';

export function Badge({ value, children }: { value?: string; children?: React.ReactNode }) {
  const tone = badgeTone(value);
  return <span className={cx('inline-flex items-center rounded-full border px-2.5 py-1 font-mono text-xs', tone)}>{children ?? statusLabel(value)}</span>;
}

function badgeTone(value?: string) {
  switch (value) {
    case 'alive':
    case 'success':
    case 'supported':
    case 'enable':
      return 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300';
    case 'dead':
    case 'failed':
    case 'error':
      return 'border-red-400/30 bg-red-500/10 text-red-300';
    case 'never':
    case 'unknown':
    case 'unsupported':
      return 'border-slate-700 bg-slate-900 text-slate-400';
    case 'random':
    case 'fixed':
      return 'border-blue-400/30 bg-blue-500/10 text-blue-300';
    default:
      return 'border-amber-400/30 bg-amber-500/10 text-amber-300';
  }
}
