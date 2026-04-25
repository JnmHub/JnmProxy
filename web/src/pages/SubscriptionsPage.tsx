import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Edit3, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { useMemo, useState } from 'react';
import { createSubscription, deleteSubscription, listSubscriptions, refreshSubscription, updateSubscription, type SubscriptionInput } from '../api/subscriptions';
import type { Subscription } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Field, Input } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatBytes, usagePercent } from '../utils/bytes';
import { formatTime, maskURL } from '../utils/format';

const defaultInput: SubscriptionInput = { name: '', url: '', user_agent: 'clash.meta', refresh_interval_seconds: 3600, enabled: true };

export function SubscriptionsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [form, setForm] = useState<SubscriptionInput>(defaultInput);
  const subscriptionsQuery = useQuery({ queryKey: ['subscriptions'], queryFn: listSubscriptions });
  const invalidate = () => { void queryClient.invalidateQueries({ queryKey: ['subscriptions'] }); };
  const createMutation = useMutation({ mutationFn: createSubscription, onSuccess: () => { closeModal(); invalidate(); } });
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: number; input: SubscriptionInput }) => updateSubscription(id, input), onSuccess: () => { closeModal(); invalidate(); } });
  const refreshMutation = useMutation({ mutationFn: refreshSubscription, onSuccess: invalidate });
  const deleteMutation = useMutation({ mutationFn: deleteSubscription, onSuccess: invalidate });
  const subscriptions = subscriptionsQuery.data ?? [];
  const summary = useMemo(() => ({
    total: subscriptions.length,
    enabled: subscriptions.filter((item) => item.enabled).length,
    failed: subscriptions.filter((item) => item.last_status === 'failed').length,
    nodes: subscriptions.reduce((total, item) => total + (item.node_count ?? 0), 0),
  }), [subscriptions]);

  const openCreate = () => { setEditingID(null); setForm(defaultInput); setOpen(true); };
  const openEdit = (subscription: Subscription) => {
    setEditingID(subscription.id);
    setForm({
      name: subscription.name,
      url: subscription.url,
      user_agent: subscription.user_agent,
      refresh_interval_seconds: subscription.refresh_interval_seconds,
      enabled: subscription.enabled,
    });
    setOpen(true);
  };
  const closeModal = () => { setOpen(false); setEditingID(null); setForm(defaultInput); };
  const submit = () => {
    if (editingID) updateMutation.mutate({ id: editingID, input: form });
    else createMutation.mutate(form);
  };

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Subscriptions</p><h1 className="mt-2 text-3xl font-semibold text-white">订阅管理</h1><p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">查看订阅刷新结果、流量用量、到期时间和节点转换统计。</p></div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <SummaryCard title="订阅数量" value={summary.total} hint="已保存订阅链接" />
        <SummaryCard title="启用订阅" value={summary.enabled} hint="会参与定时刷新" tone="success" />
        <SummaryCard title="刷新失败" value={summary.failed} hint="需要查看错误原因" tone="danger" />
        <SummaryCard title="节点总数" value={summary.nodes} hint="所有订阅累计节点" />
      </div>
      <Card>
        <CardHeader title="订阅列表" description="管理机场订阅链接、刷新节点和查看用量。" action={<Button variant="primary" onClick={openCreate}><Plus className="h-4 w-4" />新增订阅</Button>} />
        {subscriptionsQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['名称', '状态', '流量', '节点统计', '到期/刷新', '最近错误', '操作']} empty={!subscriptions.length}>
            {subscriptions.map((item) => {
              const used = (item.upload_bytes ?? 0) + (item.download_bytes ?? 0);
              return (
                <tr key={item.id} className="hover:bg-slate-900/50">
                  <td className="px-4 py-3"><Link className="font-medium text-blue-200 hover:text-blue-100" to={`/subscriptions/${item.id}`}>{item.name}</Link><div className="mt-1 font-mono text-xs text-slate-500">{maskURL(item.url)}</div><div className="mt-1 text-xs text-slate-500">间隔 {item.refresh_interval_seconds}s / {item.enabled ? '启用' : '禁用'}</div></td>
                  <td className="px-4 py-3"><Badge value={item.last_status} /></td>
                  <td className="px-4 py-3"><TrafficUsage used={used} total={item.total_bytes} /></td>
                  <td className="px-4 py-3"><NodeStats subscription={item} /></td>
                  <td className="px-4 py-3 text-xs text-slate-400"><div>到期 {formatTime(item.expire_at)}</div><div className="mt-1">最近 {formatTime(item.last_refresh_at)}</div><div className="mt-1">下次 {formatTime(item.next_refresh_at)}</div></td>
                  <td className="max-w-xs px-4 py-3 text-xs text-red-300">{item.last_error ? <span title={item.last_error}>{item.last_error}</span> : <span className="text-slate-500">—</span>}</td>
                  <td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button disabled={refreshMutation.isPending} onClick={() => refreshMutation.mutate(item.id)}><RefreshCw className="h-4 w-4" />刷新</Button><Button onClick={() => openEdit(item)}><Edit3 className="h-4 w-4" />编辑</Button><Button variant="danger" onClick={() => setConfirm({ title: '删除订阅', message: `确定删除订阅「${item.name}」吗？该订阅下节点也会删除。`, danger: true, confirmText: '删除', onConfirm: () => deleteMutation.mutate(item.id) })}><Trash2 className="h-4 w-4" />删除</Button></div></td>
                </tr>
              );
            })}
          </DataTable>
        )}
      </Card>
      <Modal open={open} title={editingID ? '编辑订阅' : '新增订阅'} onClose={closeModal} footer={<><Button variant="ghost" onClick={closeModal}>取消</Button><Button variant="primary" disabled={!form.name || !form.url || createMutation.isPending || updateMutation.isPending} onClick={submit}>保存</Button></>}>
        <div className="grid gap-4">
          <Field label="名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="示例订阅" /></Field>
          <Field label="订阅 URL"><Input value={form.url} onChange={(event) => setForm({ ...form, url: event.target.value })} placeholder="https://example.com/sub" /></Field>
          <Field label="User-Agent"><Input value={form.user_agent} onChange={(event) => setForm({ ...form, user_agent: event.target.value })} /></Field>
          <Field label="刷新间隔（秒）"><Input type="number" value={form.refresh_interval_seconds} onChange={(event) => setForm({ ...form, refresh_interval_seconds: Number(event.target.value) })} /></Field>
          <label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={form.enabled ?? true} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} />启用订阅</label>
        </div>
      </Modal>
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}

function TrafficUsage({ used, total }: { used: number; total?: number }) {
  return (
    <div>
      <div className="text-sm text-slate-200">{formatBytes(used)} / {formatBytes(total)}</div>
      <div className="mt-2 h-1.5 w-36 overflow-hidden rounded-full bg-slate-800">
        <div className="h-full bg-blue-500" style={{ width: `${usagePercent(used, total)}%` }} />
      </div>
      <div className="mt-1 text-xs text-slate-500">{usagePercent(used, total)}%</div>
    </div>
  );
}

function NodeStats({ subscription }: { subscription: Subscription }) {
  return (
    <div className="space-y-1 text-xs text-slate-400">
      <div>节点 {subscription.node_count ?? 0}</div>
      <div className="text-emerald-300">支持 {subscription.sing_box_supported_count ?? 0}</div>
      <div className="text-red-300">错误 {subscription.sing_box_error_count ?? 0}</div>
      <div>不支持 {subscription.unsupported_count ?? 0}</div>
    </div>
  );
}

function SummaryCard({ title, value, hint, tone = 'default' }: { title: string; value: number; hint: string; tone?: 'default' | 'success' | 'danger' }) {
  const color = tone === 'success' ? 'text-emerald-200' : tone === 'danger' ? 'text-red-200' : 'text-white';
  return (
    <Card>
      <div className={`font-mono text-3xl font-semibold ${color}`}>{value}</div>
      <div className="mt-2 text-sm font-medium text-slate-200">{title}</div>
      <div className="mt-1 text-xs text-slate-500">{hint}</div>
    </Card>
  );
}
