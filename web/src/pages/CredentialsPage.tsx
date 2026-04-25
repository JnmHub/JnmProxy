import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, KeyRound, Plus, Terminal, Trash2 } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { createCredential, deleteCredential, listCredentials, resetCredentialPassword, updateCredential, type CredentialInput, type CredentialUpdateInput } from '../api/credentials';
import { listGroups } from '../api/groups';
import { listNodes } from '../api/nodes';
import type { BindMode, Credential, CredentialBinding, SelectionPolicy } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Field, Input, Select } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

type ScopeForm = {
  enabled: boolean;
  bind_mode: BindMode;
  selection_policy: SelectionPolicy;
  remark: string;
  bindings: CredentialBinding[];
};

type CreateForm = ScopeForm & {
  username: string;
  password: string;
};

const defaultCreateForm: CreateForm = {
  username: '',
  password: '',
  enabled: true,
  bind_mode: 'all',
  selection_policy: 'random',
  remark: '',
  bindings: [],
};

const defaultEditForm: ScopeForm = {
  enabled: true,
  bind_mode: 'all',
  selection_policy: 'random',
  remark: '',
  bindings: [],
};

export function CredentialsPage() {
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const search = params.get('search')?.trim().toLowerCase() ?? '';
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<CreateForm>(defaultCreateForm);
  const [resetID, setResetID] = useState<number>(0);
  const [editID, setEditID] = useState<number>(0);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [newPassword, setNewPassword] = useState('');
  const [editForm, setEditForm] = useState<ScopeForm>(defaultEditForm);
  const [createdCommand, setCreatedCommand] = useState('');
  const [commandCredential, setCommandCredential] = useState<Credential | null>(null);

  const credentialsQuery = useQuery({ queryKey: ['credentials'], queryFn: listCredentials });
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });
  const nodesQuery = useQuery({ queryKey: ['nodes'], queryFn: () => listNodes() });

  const groupNames = useMemo(() => new Map((groupsQuery.data ?? []).map((group) => [group.id, group.name])), [groupsQuery.data]);
  const nodeNames = useMemo(() => new Map((nodesQuery.data ?? []).map((node) => [node.id, node.name])), [nodesQuery.data]);
  const credentials = useMemo(() => {
    const items = credentialsQuery.data ?? [];
    if (!search) return items;
    return items.filter((credential) => [credential.username, credential.remark, credential.bind_mode, credential.selection_policy].some((value) => value.toLowerCase().includes(search)));
  }, [credentialsQuery.data, search]);

  const invalidateCredentials = () => { void queryClient.invalidateQueries({ queryKey: ['credentials'] }); };
  const createMutation = useMutation({
    mutationFn: (input: CredentialInput) => createCredential(input),
    onSuccess: (_credential, input) => { setCreatedCommand(proxyCommand(input.username, input.password)); setOpen(false); setForm(defaultCreateForm); invalidateCredentials(); },
  });
  const updateMutation = useMutation({ mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => updateCredential(id, { enabled }), onSuccess: invalidateCredentials });
  const editMutation = useMutation({
    mutationFn: ({ id, input }: { id: number; input: CredentialUpdateInput }) => updateCredential(id, input),
    onSuccess: () => { setEditID(0); setEditForm(defaultEditForm); invalidateCredentials(); },
  });
  const resetMutation = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) => resetCredentialPassword(id, password),
    onSuccess: () => { setResetID(0); setNewPassword(''); invalidateCredentials(); },
  });
  const deleteMutation = useMutation({ mutationFn: deleteCredential, onSuccess: invalidateCredentials });

  const openCreate = () => {
    setForm(defaultCreateForm);
    setOpen(true);
  };
  const openEdit = (credential: Credential) => {
    setEditID(credential.id);
    setEditForm(normalizeScope({
      enabled: credential.enabled,
      bind_mode: credential.bind_mode,
      selection_policy: credential.selection_policy,
      remark: credential.remark,
      bindings: credential.bindings ?? [],
    }));
  };
  const submitCreate = () => {
    const normalized = normalizeScope(form);
    createMutation.mutate({
      username: form.username,
      password: form.password,
      enabled: normalized.enabled,
      bind_mode: normalized.bind_mode,
      selection_policy: normalized.selection_policy,
      remark: normalized.remark,
      bindings: normalized.bindings,
    });
  };
  const submitEdit = () => {
    const normalized = normalizeScope(editForm);
    editMutation.mutate({
      id: editID,
      input: {
        enabled: normalized.enabled,
        bind_mode: normalized.bind_mode,
        selection_policy: normalized.selection_policy,
        remark: normalized.remark,
        bindings: normalized.bindings,
      },
    });
  };

  const createError = scopeError(form);
  const editError = scopeError(editForm);
  const bindingLabel = (binding: CredentialBinding) => {
    const name = binding.target_type === 'group' ? groupNames.get(binding.target_id) : nodeNames.get(binding.target_id);
    return `${binding.target_type === 'group' ? '分组' : '节点'}:${name ?? binding.target_id}`;
  };

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Credentials</p><h1 className="mt-2 text-3xl font-semibold text-white">凭证管理</h1></div>
      <Card>
        <CardHeader title="代理访问凭证" description={search ? `搜索「${params.get('search')}」的凭证结果。` : '客户端使用 HTTP/SOCKS5 代理时必须携带这里创建的账号密码。绑定范围现在直接显示目标名称。'} action={<Button variant="primary" onClick={openCreate}><Plus className="h-4 w-4" />创建凭证</Button>} />
        {credentialsQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['用户名', '状态', '绑定', '策略', '备注', '时间', '操作']} empty={!credentials.length}>
            {credentials.map((credential) => (
              <tr key={credential.id}>
                <td className="px-4 py-3 font-mono text-blue-200">{credential.username}</td>
                <td className="px-4 py-3"><Badge value={credential.enabled ? 'supported' : 'unsupported'}>{credential.enabled ? '启用' : '禁用'}</Badge></td>
                <td className="px-4 py-3"><CredentialBindingSummary credential={credential} groupNames={groupNames} nodeNames={nodeNames} /></td>
                <td className="px-4 py-3"><Badge value={credential.selection_policy} /></td>
                <td className="px-4 py-3 text-slate-400">{credential.remark || '—'}</td>
                <td className="px-4 py-3 text-xs text-slate-500">{formatTime(credential.updated_at)}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Button onClick={() => updateMutation.mutate({ id: credential.id, enabled: !credential.enabled })}>{credential.enabled ? '禁用' : '启用'}</Button>
                    <Button onClick={() => openEdit(credential)}>编辑</Button>
                    <Button onClick={() => setCommandCredential(credential)}><Terminal className="h-4 w-4" />命令</Button>
                    <Button onClick={() => setResetID(credential.id)}><KeyRound className="h-4 w-4" />重置</Button>
                    <Button variant="danger" onClick={() => setConfirm({ title: '删除凭证', message: `确定删除凭证「${credential.username}」吗？客户端将无法继续使用该账号。`, danger: true, confirmText: '删除', onConfirm: () => deleteMutation.mutate(credential.id) })}><Trash2 className="h-4 w-4" />删除</Button>
                  </div>
                </td>
              </tr>
            ))}
          </DataTable>
        )}
      </Card>

      <Modal open={open} title="创建凭证" onClose={() => setOpen(false)} footer={<><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button variant="primary" disabled={!form.username || !form.password || Boolean(createError) || createMutation.isPending} onClick={submitCreate}>保存</Button></>}>
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="用户名"><Input value={form.username} onChange={(event) => setForm({ ...form, username: event.target.value })} /></Field>
          <Field label="密码"><Input type="password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} /></Field>
          <ScopeFields
            form={form}
            setForm={setForm}
            groups={groupsQuery.data ?? []}
            nodes={nodesQuery.data ?? []}
            bindingLabel={bindingLabel}
          />
          <Field label="备注"><Input value={form.remark} onChange={(event) => setForm({ ...form, remark: event.target.value })} /></Field>
        </div>
        <ScopeHint error={createError} />
      </Modal>

      <Modal open={editID > 0} title="编辑凭证" onClose={() => { setEditID(0); setEditForm(defaultEditForm); }} footer={<><Button variant="ghost" onClick={() => { setEditID(0); setEditForm(defaultEditForm); }}>取消</Button><Button variant="primary" disabled={Boolean(editError) || editMutation.isPending} onClick={submitEdit}>保存</Button></>}>
        <div className="grid gap-4">
          <label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={editForm.enabled} onChange={(event) => setEditForm({ ...editForm, enabled: event.target.checked })} />启用凭证</label>
          <ScopeFields
            form={editForm}
            setForm={setEditForm}
            groups={groupsQuery.data ?? []}
            nodes={nodesQuery.data ?? []}
            bindingLabel={bindingLabel}
          />
          <Field label="备注"><Input value={editForm.remark} onChange={(event) => setEditForm({ ...editForm, remark: event.target.value })} /></Field>
        </div>
        <ScopeHint error={editError} />
      </Modal>

      <Modal open={resetID > 0} title="重置密码" onClose={() => setResetID(0)} footer={<><Button variant="ghost" onClick={() => setResetID(0)}>取消</Button><Button variant="primary" disabled={!newPassword} onClick={() => resetMutation.mutate({ id: resetID, password: newPassword })}>确认重置</Button></>}>
        <Field label="新密码"><Input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></Field>
      </Modal>
      <CommandModal
        open={Boolean(createdCommand)}
        title="凭证创建成功"
        command={createdCommand}
        description="这是唯一一次能直接展示明文密码的测试命令，关闭后前端不会保存密码。"
        onClose={() => setCreatedCommand('')}
      />
      <CommandModal
        open={Boolean(commandCredential)}
        title="测试命令"
        command={commandCredential ? proxyCommand(commandCredential.username, '<填写密码>') : ''}
        description="已有凭证不会返回旧密码明文，请把命令里的 <填写密码> 替换成你自己保存的密码。"
        onClose={() => setCommandCredential(null)}
      />
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}

function CredentialBindingSummary({ credential, groupNames, nodeNames }: { credential: Credential; groupNames: Map<number, string>; nodeNames: Map<number, string> }) {
  const bindings = credential.bindings ?? [];
  if (credential.bind_mode === 'all') {
    return (
      <div>
        <Badge value="all">全部节点</Badge>
        <div className="mt-2 text-xs leading-5 text-slate-400">全部可用节点随机，使用洗牌袋尽量一轮不重复。</div>
      </div>
    );
  }
  if (credential.bind_mode === 'node') {
    const node = bindings.find((binding) => binding.target_type === 'node');
    return (
      <div>
        <Badge value="fixed">固定节点</Badge>
        <div className="mt-2 text-xs leading-5 text-blue-200">{node ? nodeNames.get(node.target_id) ?? `节点 ${node.target_id}` : '未绑定节点'}</div>
      </div>
    );
  }
  const names = bindings
    .filter((binding) => binding.target_type === 'group')
    .map((binding) => groupNames.get(binding.target_id) ?? `分组 ${binding.target_id}`);
  return (
    <div>
      <Badge value="group">分组随机</Badge>
      <div className="mt-2 flex max-w-sm flex-wrap gap-1.5">
        {names.length ? names.map((name) => <span key={name} className="rounded-full border border-blue-400/20 bg-blue-500/10 px-2 py-1 text-xs text-blue-100">{name}</span>) : <span className="text-xs text-slate-500">未绑定分组</span>}
      </div>
    </div>
  );
}

function CommandModal({ open, title, command, description, onClose }: { open: boolean; title: string; command: string; description: string; onClose: () => void }) {
  return (
    <Modal open={open} title={title} onClose={onClose} footer={<><Button variant="ghost" onClick={onClose}>关闭</Button><Button variant="primary" disabled={!command} onClick={() => copyCommand(command)}><Copy className="h-4 w-4" />复制命令</Button></>}>
      <div className="space-y-4">
        <p className="text-sm leading-6 text-slate-400">{description}</p>
        <div className="rounded-2xl border border-slate-800 bg-slate-950 p-4">
          <div className="mb-3 flex items-center gap-2 text-sm text-slate-300"><Terminal className="h-4 w-4 text-blue-300" />SOCKS5 测试</div>
          <code className="block overflow-x-auto whitespace-nowrap rounded-xl bg-slate-900 px-3 py-3 font-mono text-xs text-blue-100">{command || '—'}</code>
        </div>
      </div>
    </Modal>
  );
}

function ScopeFields<T extends ScopeForm>({ form, setForm, groups, nodes, bindingLabel }: {
  form: T;
  setForm: (value: T) => void;
  groups: Array<{ id: number; name: string }>;
  nodes: Array<{ id: number; name: string }>;
  bindingLabel: (binding: CredentialBinding) => string;
}) {
  const options = form.bind_mode === 'group'
    ? groups.map((group) => ({ id: group.id, label: group.name }))
    : form.bind_mode === 'node'
      ? nodes.map((node) => ({ id: node.id, label: node.name }))
      : [];
  const setBindMode = (bindMode: BindMode) => setForm({ ...form, bind_mode: bindMode, selection_policy: policyForBindMode(bindMode), bindings: [] });
  const setBinding = (value: string) => {
    const binding = bindingFromValue(value, form.bind_mode);
    if (!binding) return;
    setForm({ ...form, bindings: nextBindings(form.bindings, binding, form.bind_mode) });
  };

  return (
    <>
      <Field label="绑定模式">
        <Select value={form.bind_mode} onChange={(event) => setBindMode(event.target.value as BindMode)}>
          <option value="all">全部节点（随机）</option>
          <option value="group">指定分组（组内随机）</option>
          <option value="node">指定单个节点（固定）</option>
        </Select>
      </Field>
      <Field label="选择策略">
        <Input value={form.selection_policy === 'fixed' ? '固定：只使用一个指定节点' : '随机：从可用范围内随机选择'} readOnly />
      </Field>
      {form.bind_mode !== 'all' ? (
        <Field label="绑定目标">
          <Select value="" onChange={(event) => setBinding(event.target.value)}>
            <option value="">选择后加入</option>
            {options.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </Select>
          <BindingChips bindings={form.bindings} onRemove={(binding) => setForm({ ...form, bindings: removeBinding(form.bindings, binding) })} bindingLabel={bindingLabel} />
        </Field>
      ) : null}
    </>
  );
}

function ScopeHint({ error }: { error?: string }) {
  return (
    <div className={`mt-4 rounded-2xl border px-4 py-3 text-xs ${error ? 'border-red-400/30 bg-red-500/10 text-red-200' : 'border-blue-400/20 bg-blue-500/10 text-blue-200'}`}>
      {error ?? '规则：全部节点和分组只能随机；固定策略只用于指定一个节点。'}
    </div>
  );
}

function BindingChips({ bindings, onRemove, bindingLabel }: { bindings: CredentialBinding[]; onRemove: (binding: CredentialBinding) => void; bindingLabel: (binding: CredentialBinding) => string }) {
  if (!bindings.length) return <div className="mt-2 text-xs text-slate-500">还没有选择绑定目标。</div>;
  return (
    <div className="mt-2 flex flex-wrap gap-2">
      {bindings.map((binding) => (
        <button key={`${binding.target_type}-${binding.target_id}`} className="rounded-full border border-blue-400/30 bg-blue-500/10 px-3 py-1 font-mono text-xs text-blue-200 hover:bg-blue-500/20" onClick={() => onRemove(binding)}>
          {bindingLabel(binding)} ×
        </button>
      ))}
    </div>
  );
}

function policyForBindMode(bindMode: BindMode): SelectionPolicy {
  return bindMode === 'node' ? 'fixed' : 'random';
}

function normalizeScope<T extends ScopeForm>(form: T): T {
  const bindMode = form.bind_mode;
  const bindings = bindMode === 'all'
    ? []
    : bindMode === 'node'
      ? form.bindings.filter((binding) => binding.target_type === 'node').slice(0, 1)
      : uniqueBindings(form.bindings.filter((binding) => binding.target_type === 'group'));
  return { ...form, selection_policy: policyForBindMode(bindMode), bindings };
}

function scopeError(form: ScopeForm) {
  if (form.bind_mode === 'group' && !form.bindings.some((binding) => binding.target_type === 'group')) return '指定分组时至少选择一个分组。';
  if (form.bind_mode === 'node' && form.bindings.filter((binding) => binding.target_type === 'node').length !== 1) return '固定策略必须且只能选择一个节点。';
  return '';
}

function bindingFromValue(value: string, bindMode: BindMode): CredentialBinding | null {
  const id = Number(value);
  if (!id || bindMode === 'all') return null;
  return { target_type: bindMode === 'group' ? 'group' : 'node', target_id: id };
}

function nextBindings(bindings: CredentialBinding[], binding: CredentialBinding, bindMode: BindMode) {
  if (bindMode === 'node') return [binding];
  if (bindings.some((item) => item.target_type === binding.target_type && item.target_id === binding.target_id)) return bindings;
  return [...bindings, binding];
}

function removeBinding(bindings: CredentialBinding[], binding: CredentialBinding) {
  return bindings.filter((item) => item.target_type !== binding.target_type || item.target_id !== binding.target_id);
}

function uniqueBindings(bindings: CredentialBinding[]) {
  const seen = new Set<string>();
  return bindings.filter((binding) => {
    const key = `${binding.target_type}:${binding.target_id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function proxyCommand(username: string, password: string) {
  const safeUsername = encodeURIComponent(username);
  const safePassword = password.startsWith('<') ? password : encodeURIComponent(password);
  return `curl --proxy socks5h://${safeUsername}:${safePassword}@127.0.0.1:1080 https://httpbin.org/ip`;
}

function copyCommand(command: string) {
  if (!command || !navigator.clipboard) return;
  void navigator.clipboard.writeText(command);
}
