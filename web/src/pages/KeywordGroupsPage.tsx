import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Edit3, Play, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { applyKeywordRules, createKeywordRule, deleteKeywordRule, listKeywordRules, updateKeywordRule, type KeywordInput } from '../api/groups';
import type { GroupKeyword } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Field, Input, Textarea } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';

const defaultForm: KeywordInput = { name: '', keywords: '', case_sensitive: false, enabled: true };

export function KeywordGroupsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [form, setForm] = useState<KeywordInput>(defaultForm);
  const [selected, setSelected] = useState<number[]>([]);
  const [lastResult, setLastResult] = useState<string>('');
  const rulesQuery = useQuery({ queryKey: ['keyword-rules'], queryFn: listKeywordRules });
  const invalidate = () => { void queryClient.invalidateQueries({ queryKey: ['keyword-rules'] }); };
  const createMutation = useMutation({ mutationFn: createKeywordRule, onSuccess: () => { closeModal(); invalidate(); } });
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: number; input: KeywordInput }) => updateKeywordRule(id, input), onSuccess: () => { closeModal(); invalidate(); } });
  const deleteMutation = useMutation({ mutationFn: deleteKeywordRule, onSuccess: invalidate });
  const applyMutation = useMutation({ mutationFn: ({ all }: { all: boolean }) => applyKeywordRules(selected, all), onSuccess: (result) => { setLastResult(`规则 ${result.rules_scanned}，节点 ${result.nodes_scanned}，分组 ${result.groups_touched}，关系 ${result.relations_touched}`); } });

  const openCreate = () => { setEditingID(null); setForm(defaultForm); setOpen(true); };
  const openEdit = (rule: GroupKeyword) => { setEditingID(rule.id); setForm({ name: rule.name, keywords: rule.keywords, case_sensitive: rule.case_sensitive, enabled: rule.enabled }); setOpen(true); };
  const closeModal = () => { setOpen(false); setEditingID(null); setForm(defaultForm); };
  const submit = () => editingID ? updateMutation.mutate({ id: editingID, input: form }) : createMutation.mutate(form);

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Keyword Groups</p><h1 className="mt-2 text-3xl font-semibold text-white">关键词分组</h1></div>
      <Card>
        <CardHeader title="规则列表" description="关键词用 | 分割，命中节点名称后自动创建/加入分组。" action={<><Button onClick={() => applyMutation.mutate({ all: true })}><Play className="h-4 w-4" />执行全部</Button><Button disabled={!selected.length} onClick={() => applyMutation.mutate({ all: false })}>执行选中</Button><Button variant="primary" onClick={openCreate}><Plus className="h-4 w-4" />新增规则</Button></>} />
        {lastResult ? <div className="mb-4 rounded-2xl border border-emerald-400/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-200">{lastResult}</div> : null}
        {rulesQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['选择', '名称', '关键词', '状态', '大小写', '操作']} empty={!rulesQuery.data?.length}>
            {(rulesQuery.data ?? []).map((rule) => <tr key={rule.id}><td className="px-4 py-3"><input type="checkbox" checked={selected.includes(rule.id)} onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, rule.id] : prev.filter((id) => id !== rule.id))} /></td><td className="px-4 py-3 font-medium text-blue-200">{rule.name}</td><td className="max-w-md truncate px-4 py-3 font-mono text-xs text-slate-400">{rule.keywords}</td><td className="px-4 py-3"><Badge value={rule.enabled ? 'supported' : 'unsupported'}>{rule.enabled ? '启用' : '禁用'}</Badge></td><td className="px-4 py-3 text-slate-400">{rule.case_sensitive ? '区分' : '不区分'}</td><td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button onClick={() => openEdit(rule)}><Edit3 className="h-4 w-4" />编辑</Button><Button variant="danger" onClick={() => setConfirm({ title: '删除关键词规则', message: `确定删除规则「${rule.name}」吗？`, danger: true, confirmText: '删除', onConfirm: () => deleteMutation.mutate(rule.id) })}><Trash2 className="h-4 w-4" />删除</Button></div></td></tr>)}
          </DataTable>
        )}
      </Card>
      <Modal open={open} title={editingID ? '编辑关键词规则' : '新增关键词规则'} onClose={closeModal} footer={<><Button variant="ghost" onClick={closeModal}>取消</Button><Button variant="primary" disabled={!form.name || !form.keywords || createMutation.isPending || updateMutation.isPending} onClick={submit}>保存</Button></>}>
        <div className="grid gap-4"><Field label="规则名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></Field><Field label="关键词" hint="例如：香港|HK|日本|JP"><Textarea value={form.keywords} onChange={(event) => setForm({ ...form, keywords: event.target.value })} /></Field><label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={form.case_sensitive} onChange={(event) => setForm({ ...form, case_sensitive: event.target.checked })} />区分大小写</label><label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={form.enabled ?? true} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} />启用规则</label></div>
      </Modal>
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}
