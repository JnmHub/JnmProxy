import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Play, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { applyKeywordRules, createKeywordRule, deleteKeywordRule, listKeywordRules, type KeywordInput } from '../api/groups';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input, Textarea } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';

const defaultForm: KeywordInput = { name: '', keywords: '', case_sensitive: false, enabled: true };

export function KeywordGroupsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<KeywordInput>(defaultForm);
  const [selected, setSelected] = useState<number[]>([]);
  const [lastResult, setLastResult] = useState<string>('');
  const rulesQuery = useQuery({ queryKey: ['keyword-rules'], queryFn: listKeywordRules });
  const createMutation = useMutation({ mutationFn: createKeywordRule, onSuccess: () => { setOpen(false); setForm(defaultForm); void queryClient.invalidateQueries({ queryKey: ['keyword-rules'] }); } });
  const deleteMutation = useMutation({ mutationFn: deleteKeywordRule, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['keyword-rules'] }); } });
  const applyMutation = useMutation({ mutationFn: ({ all }: { all: boolean }) => applyKeywordRules(selected, all), onSuccess: (result) => { setLastResult(`规则 ${result.rules_scanned}，节点 ${result.nodes_scanned}，分组 ${result.groups_touched}，关系 ${result.relations_touched}`); } });
  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Keyword Groups</p><h1 className="mt-2 text-3xl font-semibold text-white">关键词分组</h1></div>
      <Card>
        <CardHeader title="规则列表" description="关键词用 | 分割，命中节点名称后自动创建/加入分组。" action={<><Button onClick={() => applyMutation.mutate({ all: true })}><Play className="h-4 w-4" />执行全部</Button><Button disabled={!selected.length} onClick={() => applyMutation.mutate({ all: false })}>执行选中</Button><Button variant="primary" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />新增规则</Button></>} />
        {lastResult ? <div className="mb-4 rounded-2xl border border-emerald-400/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-200">{lastResult}</div> : null}
        <DataTable columns={['选择', '名称', '关键词', '状态', '大小写', '操作']} empty={!rulesQuery.data?.length}>
          {(rulesQuery.data ?? []).map((rule) => <tr key={rule.id}><td className="px-4 py-3"><input type="checkbox" checked={selected.includes(rule.id)} onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, rule.id] : prev.filter((id) => id !== rule.id))} /></td><td className="px-4 py-3 font-medium text-blue-200">{rule.name}</td><td className="max-w-md truncate px-4 py-3 font-mono text-xs text-slate-400">{rule.keywords}</td><td className="px-4 py-3"><Badge value={rule.enabled ? 'supported' : 'unsupported'}>{rule.enabled ? '启用' : '禁用'}</Badge></td><td className="px-4 py-3 text-slate-400">{rule.case_sensitive ? '区分' : '不区分'}</td><td className="px-4 py-3"><Button variant="danger" onClick={() => window.confirm('确定删除该规则吗？') && deleteMutation.mutate(rule.id)}><Trash2 className="h-4 w-4" />删除</Button></td></tr>)}
        </DataTable>
      </Card>
      <Modal open={open} title="新增关键词规则" onClose={() => setOpen(false)} footer={<><Button variant="ghost" onClick={() => setOpen(false)}>取消</Button><Button variant="primary" disabled={!form.name || !form.keywords} onClick={() => createMutation.mutate(form)}>保存</Button></>}>
        <div className="grid gap-4"><Field label="规则名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></Field><Field label="关键词" hint="例如：香港|HK|日本|JP"><Textarea value={form.keywords} onChange={(event) => setForm({ ...form, keywords: event.target.value })} /></Field><label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={form.case_sensitive} onChange={(event) => setForm({ ...form, case_sensitive: event.target.checked })} />区分大小写</label></div>
      </Modal>
    </div>
  );
}
