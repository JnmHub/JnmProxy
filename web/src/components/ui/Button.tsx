import type { ButtonHTMLAttributes, PropsWithChildren } from 'react';
import { cx } from '../../utils/status';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'success';

export function Button({ children, className, variant = 'secondary', ...props }: PropsWithChildren<ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }>) {
  const variants: Record<ButtonVariant, string> = {
    primary: 'border-blue-400/30 bg-blue-500/15 text-blue-100 hover:bg-blue-500/25',
    secondary: 'border-slate-700 bg-slate-900 text-slate-100 hover:bg-slate-800',
    danger: 'border-red-400/30 bg-red-500/10 text-red-200 hover:bg-red-500/20',
    ghost: 'border-transparent bg-transparent text-slate-300 hover:bg-slate-900',
    success: 'border-emerald-400/30 bg-emerald-500/10 text-emerald-200 hover:bg-emerald-500/20',
  };
  return (
    <button
      {...props}
      className={cx(
        'inline-flex items-center justify-center gap-2 rounded-xl border px-3.5 py-2 text-sm font-medium transition-colors duration-200 disabled:cursor-not-allowed disabled:opacity-50',
        variants[variant],
        className,
      )}
    >
      {children}
    </button>
  );
}
