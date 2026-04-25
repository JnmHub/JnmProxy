import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { createGroup, deleteGroup, listGroups } from '../api/groups';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function GroupsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ name: '', description: '' });
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });
  const createMutation = useMutation({ mutationFn: createGroup, onSuccess: () => { setOpen(false); setForm({ name: '', description: '' }); void queryClient.invalidateQueries({ queryKey: ['groups'] }); } });
  const deleteMutation = useMutation({ mutationFn: deleteGroup, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['groups'] }); } });
  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Groups</p><h1 className="mt-2 text-3xl font-semibold text-white">分组管理</h1></div>
      <Card>
        <CardHeader title="分组列表" description="节点可以属于多个分组，凭证也可以绑定分组。" action={<Button variant="primary" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />创建分组</Button>} />
        <DataTable columns={['名称', '描述', '来源', '时间', '操作']} empty={!groupsQuery.data?.length}>
          {(groupsQuery.data ?? []).map((group) => <tr key={group.id}><td className="px-4 py-3 font-medium text-blue-200">{group.name}</td><td className="px-4 py-3 text-slate-400">{group.description || '—'}</td><td className="px-4 py-3"><Badge value={group.auto_created ? 'supported' : 'unknown'}>{group.auto_created ? '自动创建' : '手动创建'}</Badge></td><td className="px-4 py-3 text-xs text-slate-500">{formatTime(group.updated_at)}</td><td className="px-4 py-3"><Button variant="danger" onClick={() => window.confirm('确定删除该分组吗？') && deleteMutation.mutate(group.id)}><Trash2 className="h-4 w-4" />删除</Button></td></tr>)}
        </DataTable>
      </Card>
      <Modal open={open} title="创建分组" onClose={() => setOpen(false)} footer={<><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button variant="primary" disabled={!form.name} onClick={() => createMutation.mutate(form)}>保存</Button></>}>
        <div className="grid gap-4"><Field label="名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></Field><Field label="描述"><Input value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></Field></div>
      </Modal>
    </div>
  );
}
