export function LoadingState({ title = '正在加载', description = '正在从后端获取最新数据。' }: { title?: string; description?: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-950/50 px-6 py-10 text-center">
      <div className="mx-auto h-9 w-9 animate-spin rounded-full border-2 border-blue-400 border-t-transparent" />
      <div className="mt-4 text-sm font-medium text-slate-200">{title}</div>
      <p className="mt-2 text-sm text-slate-500">{description}</p>
    </div>
  );
}
