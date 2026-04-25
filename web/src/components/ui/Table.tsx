import type { ReactNode } from 'react';

export function DataTable({ columns, children, empty }: { columns: string[]; children: ReactNode; empty?: boolean }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-slate-800">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-slate-800 text-left text-sm">
          <thead className="bg-slate-900/80 text-xs uppercase tracking-wide text-slate-500">
            <tr>
              {columns.map((column) => (
                <th key={column} className="whitespace-nowrap px-4 py-3 font-semibold">
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800 bg-slate-950/40 text-slate-300">{children}</tbody>
        </table>
      </div>
      {empty ? <div className="bg-slate-950/40 px-4 py-10 text-center text-sm text-slate-500">暂无数据</div> : null}
    </div>
  );
}
