import { AlertTriangle } from 'lucide-react';
import { Button } from './Button';

export interface ConfirmState {
  title: string;
  message: string;
  confirmText?: string;
  danger?: boolean;
  onConfirm: () => void;
}

export function ConfirmDialog({ state, onClose }: { state: ConfirmState | null; onClose: () => void }) {
  if (!state) return null;
  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/80 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-3xl border border-slate-800 bg-slate-950 p-5 shadow-2xl">
        <div className="flex gap-4">
          <div className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${state.danger ? 'bg-red-500/10 text-red-300' : 'bg-amber-500/10 text-amber-300'}`}>
            <AlertTriangle className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">{state.title}</h2>
            <p className="mt-2 text-sm leading-6 text-slate-400">{state.message}</p>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button variant={state.danger ? 'danger' : 'primary'} onClick={() => { state.onConfirm(); onClose(); }}>{state.confirmText ?? '确认'}</Button>
        </div>
      </div>
    </div>
  );
}
