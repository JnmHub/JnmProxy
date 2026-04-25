import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { createCredential, deleteCredential, listCredentials, resetCredentialPassword, updateCredential, type CredentialInput } from '../api/credentials';
import { listGroups } from '../api/groups';
import { listNodes } from '../api/nodes';
import type { BindMode, CredentialBinding, SelectionPolicy } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input, Select } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

const defaultForm: CredentialInput = { username: '', password: '', enabled: true, bind_mode: 'all', selection_policy: 'random', remark: '', bindings: [] };

export function CredentialsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<CredentialInput>(defaultForm);
  const [resetID, setResetID] = useState<number>(0);
  const [newPassword, setNewPassword] = useState('');
  const credentialsQuery = useQuery({ queryKey: ['credentials'], queryFn: listCredentials });
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });
  const nodesQuery = useQuery({ queryKey: ['nodes'], queryFn: () => listNodes() });
  const createMutation = useMutation({ mutationFn: createCredential, onSuccess: () => { setOpen(false); setForm(defaultForm); void queryClient.invalidateQueries({ queryKey: ['credentials'] }); } });
  const updateMutation = useMutation({ mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => updateCredential(id, { enabled }), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['credentials'] }); } });
  const resetMutation = useMutation({ mutationFn: ({ id, password }: { id: number; password: string }) => resetCredentialPassword(id, password), onSuccess: () => { setResetID(0); setNewPassword(''); void queryClient.invalidateQueries({ queryKey: ['credentials'] }); } });
  const deleteMutation = useMutation({ mutationFn: deleteCredential, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['credentials'] }); } });

  const bindingOptions = form.bind_mode === 'group' ? (groupsQuery.data ?? []).map((group) => ({ id: group.id, label: group.name, type: 'group' as const })) : form.bind_mode === 'node' ? (nodesQuery.data ?? []).map((node) => ({ id: node.id, label: node.name, type: 'node' as const })) : [];

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Credentials</p><h1 className="mt-2 text-3xl font-semibold text-white">凭证管理</h1></div>
      <Card>
        <CardHeader title="代理访问凭证" description="客户端使用 HTTP/SOCKS5 代理时必须携带这里创建的账号密码。" action={<Button variant="primary" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />创建凭证</Button>} />
        <DataTable columns={['用户名', '状态', '绑定', '策略', '备注', '时间', '操作']} empty={!credentialsQuery.data?.length}>
          {(credentialsQuery.data ?? []).map((credential) => <tr key={credential.id}><td className="px-4 py-3 font-mono text-blue-200">{credential.username}</td><td className="px-4 py-3"><Badge value={credential.enabled ? 'supported' : 'unsupported'}>{credential.enabled ? '启用' : '禁用'}</Badge></td><td className="px-4 py-3"><Badge value={credential.bind_mode} /></td><td className="px-4 py-3"><Badge value={credential.selection_policy} /></td><td className="px-4 py-3 text-slate-400">{credential.remark || '—'}</td><td className="px-4 py-3 text-xs text-slate-500">{formatTime(credential.updated_at)}</td><td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button onClick={() => updateMutation.mutate({ id: credential.id, enabled: !credential.enabled })}>{credential.enabled ? '禁用' : '启用'}</Button><Button onClick={() => setResetID(credential.id)}><KeyRound className="h-4 w-4" />重置</Button><Button variant="danger" onClick={() => window.confirm('确定删除该凭证吗？') && deleteMutation.mutate(credential.id)}><Trash2 className="h-4 w-4" />删除</Button></div></td></tr>)}
        </DataTable>
      </Card>
      <Modal open={open} title="创建凭证" onClose={() => setOpen(false)} footer={<><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button variant="primary" disabled={!form.username || !form.password} onClick={() => createMutation.mutate(form)}>保存</Button></>}>
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="用户名"><Input value={form.username} onChange={(event) => setForm({ ...form, username: event.target.value })} /></Field>
          <Field label="密码"><Input type="password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} /></Field>
          <Field label="绑定模式"><Select value={form.bind_mode} onChange={(event) => setForm({ ...form, bind_mode: event.target.value as BindMode, bindings: [] })}><option value="all">全部节点</option><option value="group">指定分组</option><option value="node">指定节点</option></Select></Field>
          <Field label="选择策略"><Select value={form.selection_policy} onChange={(event) => setForm({ ...form, selection_policy: event.target.value as SelectionPolicy })}><option value="random">随机</option><option value="fixed">固定</option></Select></Field>
          <Field label="备注"><Input value={form.remark} onChange={(event) => setForm({ ...form, remark: event.target.value })} /></Field>
          {form.bind_mode !== 'all' ? <Field label="绑定目标"><Select value="" onChange={(event) => addBinding(event.target.value, form.bind_mode, form, setForm)}><option value="">选择后加入</option>{bindingOptions.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}</Select><div className="mt-2 flex flex-wrap gap-2">{(form.bindings ?? []).map((binding) => <button key={`${binding.target_type}-${binding.target_id}`} className="rounded-full border border-blue-400/30 bg-blue-500/10 px-3 py-1 font-mono text-xs text-blue-200" onClick={() => setForm({ ...form, bindings: form.bindings?.filter((item) => item !== binding) })}>{binding.target_type}:{binding.target_id} ×</button>)}</div></Field> : null}
        </div>
      </Modal>
      <Modal open={resetID > 0} title="重置密码" onClose={() => setResetID(0)} footer={<><Button variant="ghost" onClick={() => setResetID(0)}>取消</Button><Button variant="primary" disabled={!newPassword} onClick={() => resetMutation.mutate({ id: resetID, password: newPassword })}>确认重置</Button></>}>
        <Field label="新密码"><Input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></Field>
      </Modal>
    </div>
  );
}

function addBinding(value: string, bindMode: BindMode, form: CredentialInput, setForm: (value: CredentialInput) => void) {
  const id = Number(value);
  if (!id || bindMode === 'all') return;
  const binding: CredentialBinding = { target_type: bindMode === 'group' ? 'group' : 'node', target_id: id };
  if ((form.bindings ?? []).some((item) => item.target_type === binding.target_type && item.target_id === binding.target_id)) return;
  setForm({ ...form, bindings: [...(form.bindings ?? []), binding] });
}
