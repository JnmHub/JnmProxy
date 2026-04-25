export function JsonBlock({ value }: { value?: string }) {
  if (!value) return <div className="text-sm text-slate-500">暂无配置</div>;
  return (
    <details className="rounded-2xl border border-slate-800 bg-slate-950">
      <summary className="cursor-pointer px-4 py-3 text-sm text-slate-300 hover:text-blue-200">展开查看配置 JSON（可能包含敏感字段）</summary>
      <pre className="max-h-80 overflow-auto border-t border-slate-800 p-4 font-mono text-xs leading-5 text-slate-300">{formatJSON(value)}</pre>
    </details>
  );
}

function formatJSON(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
