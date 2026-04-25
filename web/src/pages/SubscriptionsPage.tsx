import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Plus, RefreshCw, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { createSubscription, deleteSubscription, listSubscriptions, refreshSubscription, type SubscriptionInput } from '../api/subscriptions';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatBytes, usagePercent } from '../utils/bytes';
import { formatTime, maskURL } from '../utils/format';

const defaultInput: SubscriptionInput = { name: '', url: '', user_agent: 'clash/1.18.0', refresh_interval_seconds: 3600, enabled: true };

export function SubscriptionsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<SubscriptionInput>(defaultInput);
  const subscriptionsQuery = useQuery({ queryKey: ['subscriptions'], queryFn: listSubscriptions });
  const createMutation = useMutation({ mutationFn: createSubscription, onSuccess: () => { setOpen(false); setForm(defaultInput); void queryClient.invalidateQueries({ queryKey: ['subscriptions'] }); } });
  const refreshMutation = useMutation({ mutationFn: refreshSubscription, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['subscriptions'] }); } });
  const deleteMutation = useMutation({ mutationFn: deleteSubscription, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['subscriptions'] }); } });

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Subscriptions</p><h1 className="mt-2 text-3xl font-semibold text-white">订阅管理</h1></div>
      <Card>
        <CardHeader title="订阅列表" description="管理机场订阅链接、刷新节点和查看用量。" action={<Button variant="primary" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />新增订阅</Button>} />
        <DataTable columns={['名称', 'URL', '状态', '用量', '刷新时间', '操作']} empty={!subscriptionsQuery.data?.length}>
          {(subscriptionsQuery.data ?? []).map((item) => {
            const used = (item.upload_bytes ?? 0) + (item.download_bytes ?? 0);
            return (
              <tr key={item.id} className="hover:bg-slate-900/50">
                <td className="px-4 py-3"><Link className="font-medium text-blue-200 hover:text-blue-100" to={`/subscriptions/${item.id}`}>{item.name}</Link><div className="mt-1 text-xs text-slate-500">间隔 {item.refresh_interval_seconds}s</div></td>
                <td className="px-4 py-3 font-mono text-xs text-slate-400">{maskURL(item.url)}</td>
                <td className="px-4 py-3"><Badge value={item.last_status} /></td>
                <td className="px-4 py-3"><div className="text-sm">{formatBytes(used)} / {formatBytes(item.total_bytes)}</div><div className="mt-1 h-1.5 w-32 overflow-hidden rounded-full bg-slate-800"><div className="h-full bg-blue-500" style={{ width: `${usagePercent(used, item.total_bytes)}%` }} /></div></td>
                <td className="px-4 py-3 text-xs text-slate-400"><div>{formatTime(item.last_refresh_at)}</div><div>下次 {formatTime(item.next_refresh_at)}</div></td>
                <td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button onClick={() => refreshMutation.mutate(item.id)}><RefreshCw className="h-4 w-4" />刷新</Button><Button variant="danger" onClick={() => window.confirm('确定删除该订阅吗？') && deleteMutation.mutate(item.id)}><Trash2 className="h-4 w-4" />删除</Button></div></td>
              </tr>
            );
          })}
        </DataTable>
      </Card>
      <Modal open={open} title="新增订阅" onClose={() => setOpen(false)} footer={<><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button variant="primary" disabled={!form.name || !form.url || createMutation.isPending} onClick={() => createMutation.mutate(form)}>保存</Button></>}>
        <div className="grid gap-4">
          <Field label="名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="示例订阅" /></Field>
          <Field label="订阅 URL"><Input value={form.url} onChange={(event) => setForm({ ...form, url: event.target.value })} placeholder="https://example.com/sub" /></Field>
          <Field label="User-Agent"><Input value={form.user_agent} onChange={(event) => setForm({ ...form, user_agent: event.target.value })} /></Field>
          <Field label="刷新间隔（秒）"><Input type="number" value={form.refresh_interval_seconds} onChange={(event) => setForm({ ...form, refresh_interval_seconds: Number(event.target.value) })} /></Field>
        </div>
      </Modal>
    </div>
  );
}
