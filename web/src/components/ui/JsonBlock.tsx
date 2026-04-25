export function JsonBlock({ value }: { value?: string }) {
  if (!value) return <div className="text-sm text-slate-500">暂无配置</div>;
  return <pre className="max-h-80 overflow-auto rounded-2xl border border-slate-800 bg-slate-950 p-4 font-mono text-xs leading-5 text-slate-300">{formatJSON(value)}</pre>;
}

function formatJSON(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
