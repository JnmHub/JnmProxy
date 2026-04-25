import { Button } from './Button';
import { Select } from './Input';

export function PaginationBar({ page, pageSize, total, totalPages, onPageChange, onPageSizeChange }: {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(total, page * pageSize);
  return (
    <div className="mt-4 flex flex-col gap-3 rounded-2xl border border-slate-800 bg-slate-950/50 p-4 text-sm text-slate-400 md:flex-row md:items-center md:justify-between">
      <div>
        显示 <span className="font-mono text-slate-200">{start}-{end}</span> / <span className="font-mono text-slate-200">{total}</span>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Select className="w-28" value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))}>
          <option value={20}>20 / 页</option>
          <option value={50}>50 / 页</option>
          <option value={100}>100 / 页</option>
          <option value={200}>200 / 页</option>
        </Select>
        <Button disabled={page <= 1} onClick={() => onPageChange(1)}>首页</Button>
        <Button disabled={page <= 1} onClick={() => onPageChange(page - 1)}>上一页</Button>
        <span className="rounded-xl border border-slate-800 bg-slate-900 px-3 py-2 font-mono text-xs text-slate-300">
          {page} / {totalPages}
        </span>
        <Button disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>下一页</Button>
        <Button disabled={page >= totalPages} onClick={() => onPageChange(totalPages)}>末页</Button>
      </div>
    </div>
  );
}
