import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { Button } from './Button';

type ToastEvent = CustomEvent<{ title: string; message?: string; tone?: 'error' | 'success' | 'info' }>;

type ToastItem = {
  id: number;
  title: string;
  message?: string;
  tone: 'error' | 'success' | 'info';
};

export function notifyToast(title: string, message?: string, tone: ToastItem['tone'] = 'info') {
  window.dispatchEvent(new CustomEvent('jnmproxy:toast', { detail: { title, message, tone } }));
}

export function ToastViewport() {
  const [items, setItems] = useState<ToastItem[]>([]);
  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as ToastEvent).detail;
      const item: ToastItem = { id: Date.now() + Math.random(), title: detail.title, message: detail.message, tone: detail.tone ?? 'info' };
      setItems((current) => [...current, item].slice(-4));
      window.setTimeout(() => setItems((current) => current.filter((candidate) => candidate.id !== item.id)), 5000);
    };
    window.addEventListener('jnmproxy:toast', handler);
    return () => window.removeEventListener('jnmproxy:toast', handler);
  }, []);

  if (!items.length) return null;
  return (
    <div className="fixed bottom-5 right-5 z-[60] flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-3">
      {items.map((item) => (
        <div key={item.id} className={`rounded-2xl border p-4 shadow-2xl backdrop-blur-xl ${toneClass(item.tone)}`}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-white">{item.title}</div>
              {item.message ? <p className="mt-1 text-sm text-slate-300">{item.message}</p> : null}
            </div>
            <Button variant="ghost" className="px-2 py-1" onClick={() => setItems((current) => current.filter((candidate) => candidate.id !== item.id))} aria-label="关闭提示">
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}

function toneClass(tone: ToastItem['tone']) {
  switch (tone) {
    case 'error':
      return 'border-red-400/30 bg-red-950/90';
    case 'success':
      return 'border-emerald-400/30 bg-emerald-950/90';
    default:
      return 'border-blue-400/30 bg-slate-950/90';
  }
}
