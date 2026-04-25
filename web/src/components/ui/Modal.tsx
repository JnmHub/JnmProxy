import type { PropsWithChildren, ReactNode } from 'react';
import { X } from 'lucide-react';
import { Button } from './Button';

export function Modal({ open, title, children, footer, onClose }: PropsWithChildren<{ open: boolean; title: string; footer?: ReactNode; onClose: () => void }>) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4 backdrop-blur-sm">
      <div className="w-full max-w-2xl rounded-3xl border border-slate-800 bg-slate-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-4">
          <h2 className="text-lg font-semibold text-white">{title}</h2>
          <Button variant="ghost" className="px-2" onClick={onClose} aria-label="关闭">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="max-h-[70vh] overflow-y-auto p-5">{children}</div>
        {footer ? <div className="flex justify-end gap-2 border-t border-slate-800 px-5 py-4">{footer}</div> : null}
      </div>
    </div>
  );
}
