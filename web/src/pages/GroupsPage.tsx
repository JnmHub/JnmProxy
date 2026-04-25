import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Edit3, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { addNodesToGroup, createGroup, deleteGroup, listGroups, removeNodesFromGroup, updateGroup, type GroupInput } from '../api/groups';
import { listNodes } from '../api/nodes';
import type { ProxyGroup, ProxyNode } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Drawer } from '../components/ui/Drawer';
import { Field, Input, Select } from '../components/ui/Input';
import { LoadingState } from '../components/ui/LoadingState';
import { Modal } from '../components/ui/Modal';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

const defaultForm: GroupInput = { name: '', description: '', auto_created: false };

export function GroupsPage() {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [form, setForm] = useState<GroupInput>(defaultForm);
  const [detailGroup, setDetailGroup] = useState<ProxyGroup | null>(null);
  const [addNodeID, setAddNodeID] = useState<number>(0);
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });
  const groupNodesQuery = useQuery({ queryKey: ['nodes', { group_id: detailGroup?.id }], queryFn: () => listNodes({ group_id: detailGroup?.id }), enabled: Boolean(detailGroup?.id) });
  const allNodesQuery = useQuery({ queryKey: ['nodes', 'all-options'], queryFn: () => listNodes(), enabled: Boolean(detailGroup?.id) });
  const invalidate = () => { void queryClient.invalidateQueries({ queryKey: ['groups'] }); };
  const invalidateGroupNodes = () => {
    void queryClient.invalidateQueries({ queryKey: ['nodes'] });
    void queryClient.invalidateQueries({ queryKey: ['groups'] });
  };
  const createMutation = useMutation({ mutationFn: createGroup, onSuccess: () => { closeModal(); invalidate(); } });
  const updateMutation = useMutation({ mutationFn: ({ id, input }: { id: number; input: GroupInput }) => updateGroup(id, input), onSuccess: () => { closeModal(); invalidate(); } });
  const deleteMutation = useMutation({ mutationFn: deleteGroup, onSuccess: () => { setDetailGroup(null); invalidateGroupNodes(); } });
  const addNodeMutation = useMutation({ mutationFn: ({ groupID, nodeID }: { groupID: number; nodeID: number }) => addNodesToGroup(groupID, [nodeID]), onSuccess: () => { setAddNodeID(0); invalidateGroupNodes(); } });
  const removeNodeMutation = useMutation({ mutationFn: ({ groupID, nodeID }: { groupID: number; nodeID: number }) => removeNodesFromGroup(groupID, [nodeID]), onSuccess: invalidateGroupNodes });

  const openCreate = () => { setEditingID(null); setForm(defaultForm); setOpen(true); };
  const openEdit = (group: ProxyGroup) => { setEditingID(group.id); setForm({ name: group.name, description: group.description, auto_created: group.auto_created }); setOpen(true); };
  const closeModal = () => { setOpen(false); setEditingID(null); setForm(defaultForm); };
  const submit = () => editingID ? updateMutation.mutate({ id: editingID, input: form }) : createMutation.mutate(form);
  const groupNodes = groupNodesQuery.data ?? [];
  const availableNodes = (allNodesQuery.data ?? []).filter((node) => !groupNodes.some((groupNode) => groupNode.id === node.id));

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Groups</p><h1 className="mt-2 text-3xl font-semibold text-white">分组管理</h1></div>
      <Card>
        <CardHeader title="分组列表" description="节点可以属于多个分组，凭证也可以绑定分组。" action={<Button variant="primary" onClick={openCreate}><Plus className="h-4 w-4" />创建分组</Button>} />
        {groupsQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['名称', '描述', '来源', '时间', '操作']} empty={!groupsQuery.data?.length}>
            {(groupsQuery.data ?? []).map((group) => <tr key={group.id}><td className="px-4 py-3"><button className="font-medium text-blue-200 hover:text-blue-100" onClick={() => setDetailGroup(group)}>{group.name}</button></td><td className="px-4 py-3 text-slate-400">{group.description || '—'}</td><td className="px-4 py-3"><Badge value={group.auto_created ? 'supported' : 'unknown'}>{group.auto_created ? '自动创建' : '手动创建'}</Badge></td><td className="px-4 py-3 text-xs text-slate-500">{formatTime(group.updated_at)}</td><td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button onClick={() => setDetailGroup(group)}>节点</Button><Button onClick={() => openEdit(group)}><Edit3 className="h-4 w-4" />编辑</Button><Button variant="danger" onClick={() => setConfirm({ title: '删除分组', message: `确定删除分组「${group.name}」吗？节点不会删除，只会解除分组关系。`, danger: true, confirmText: '删除', onConfirm: () => deleteMutation.mutate(group.id) })}><Trash2 className="h-4 w-4" />删除</Button></div></td></tr>)}
          </DataTable>
        )}
      </Card>
      <Modal open={open} title={editingID ? '编辑分组' : '创建分组'} onClose={closeModal} footer={<><Button variant="ghost" onClick={closeModal}>取消</Button><Button variant="primary" disabled={!form.name || createMutation.isPending || updateMutation.isPending} onClick={submit}>保存</Button></>}>
        <div className="grid gap-4"><Field label="名称"><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></Field><Field label="描述"><Input value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></Field><label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={form.auto_created ?? false} onChange={(event) => setForm({ ...form, auto_created: event.target.checked })} />标记为自动创建</label></div>
      </Modal>
      <Drawer open={Boolean(detailGroup)} title={detailGroup?.name ?? '分组详情'} onClose={() => setDetailGroup(null)}>
        {detailGroup ? (
          <GroupNodesPanel
            group={detailGroup}
            nodes={groupNodes}
            availableNodes={availableNodes}
            addNodeID={addNodeID}
            loading={groupNodesQuery.isLoading}
            onAddNodeIDChange={setAddNodeID}
            onAddNode={() => addNodeMutation.mutate({ groupID: detailGroup.id, nodeID: addNodeID })}
            onRemoveNode={(node) => setConfirm({
              title: '移出分组',
              message: `确定把节点「${node.name}」从分组「${detailGroup.name}」移出吗？`,
              danger: true,
              confirmText: '移出',
              onConfirm: () => removeNodeMutation.mutate({ groupID: detailGroup.id, nodeID: node.id }),
            })}
          />
        ) : null}
      </Drawer>
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}

function GroupNodesPanel({
  group,
  nodes,
  availableNodes,
  addNodeID,
  loading,
  onAddNodeIDChange,
  onAddNode,
  onRemoveNode,
}: {
  group: ProxyGroup;
  nodes: ProxyNode[];
  availableNodes: ProxyNode[];
  addNodeID: number;
  loading: boolean;
  onAddNodeIDChange: (id: number) => void;
  onAddNode: () => void;
  onRemoveNode: (node: ProxyNode) => void;
}) {
  return (
    <div className="space-y-5">
      <div className="rounded-2xl border border-slate-800 bg-slate-900/40 p-4 text-sm text-slate-400">
        <div className="font-medium text-slate-200">{group.description || '暂无描述'}</div>
        <div className="mt-2">当前分组内有 <span className="font-mono text-blue-200">{nodes.length}</span> 个节点。</div>
      </div>
      <div className="grid gap-3 md:grid-cols-[1fr_auto]">
        <Field label="添加节点">
          <Select value={addNodeID} onChange={(event) => onAddNodeIDChange(Number(event.target.value))}>
            <option value={0}>选择节点</option>
            {availableNodes.map((node) => <option key={node.id} value={node.id}>{node.name}</option>)}
          </Select>
        </Field>
        <div className="flex items-end">
          <Button variant="primary" disabled={!addNodeID} onClick={onAddNode}>加入分组</Button>
        </div>
      </div>
      {loading ? <LoadingState /> : (
        <DataTable columns={['节点', '协议', '健康', 'sing-box', '操作']} empty={!nodes.length}>
          {nodes.map((node) => (
            <tr key={node.id}>
              <td className="px-4 py-3 text-blue-200">{node.name}</td>
              <td className="px-4 py-3 font-mono text-xs text-slate-400">{node.protocol}</td>
              <td className="px-4 py-3"><Badge value={node.alive_status} /></td>
              <td className="px-4 py-3"><Badge value={node.sing_box_status} /></td>
              <td className="px-4 py-3"><Button variant="danger" onClick={() => onRemoveNode(node)}>移出</Button></td>
            </tr>
          ))}
        </DataTable>
      )}
    </div>
  );
}
