import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw, RotateCcw, Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { batchNodes, checkAllNodes, checkNode, listNodes, rebuildNodeAdapter, setNodeEnabled, type NodeBatchAction, type NodeFilter } from '../api/nodes';
import { listGroups } from '../api/groups';
import { listSubscriptions } from '../api/subscriptions';
import type { ProxyNode } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Drawer } from '../components/ui/Drawer';
import { Field, Input, Select } from '../components/ui/Input';
import { JsonBlock } from '../components/ui/JsonBlock';
import { LoadingState } from '../components/ui/LoadingState';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function NodesPage() {
  const [params] = useSearchParams();
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<NodeFilter>({ subscription_id: Number(params.get('subscription_id')) || undefined });
  const [selected, setSelected] = useState<number[]>([]);
  const [detail, setDetail] = useState<ProxyNode | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const queryClient = useQueryClient();
  const nodesQuery = useQuery({ queryKey: ['nodes', filter], queryFn: () => listNodes(filter) });
  const subscriptionsQuery = useQuery({ queryKey: ['subscriptions'], queryFn: listSubscriptions });
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });
  const invalidate = () => { void queryClient.invalidateQueries({ queryKey: ['nodes'] }); };
  const setEnabledMutation = useMutation({ mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => setNodeEnabled(id, enabled), onSuccess: invalidate });
  const checkMutation = useMutation({ mutationFn: checkNode, onSuccess: invalidate });
  const checkAllMutation = useMutation({ mutationFn: checkAllNodes, onSuccess: invalidate });
  const rebuildMutation = useMutation({ mutationFn: rebuildNodeAdapter, onSuccess: invalidate });
  const batchMutation = useMutation({ mutationFn: ({ action, groupID }: { action: 'enable' | 'disable' | 'add_group' | 'remove_group'; groupID?: number }) => batchNodes(action, selected, groupID), onSuccess: () => { setSelected([]); invalidate(); } });

  const nodes = useMemo(() => (nodesQuery.data ?? []).filter((node) => node.name.toLowerCase().includes(search.toLowerCase())), [nodesQuery.data, search]);
  const allSelected = selected.length > 0 && selected.length === nodes.length;
  const updateFilter = (next: NodeFilter) => {
    setFilter(next);
    setSelected([]);
  };
  const requestBatch = (action: NodeBatchAction, groupID?: number) => {
    const labels: Record<NodeBatchAction, string> = {
      enable: '批量启用',
      disable: '批量禁用',
      add_group: '批量加入分组',
      remove_group: '批量移出分组',
    };
    setConfirm({
      title: labels[action],
      message: `确定对已选 ${selected.length} 个节点执行「${labels[action]}」吗？`,
      danger: action === 'disable' || action === 'remove_group',
      confirmText: '确认执行',
      onConfirm: () => batchMutation.mutate({ action, groupID }),
    });
  };
  const requestCheckAll = () => {
    setConfirm({
      title: '批量健康检查',
      message: '确定立即检查所有可检查节点吗？节点较多时可能需要等待一会儿。',
      confirmText: '开始检查',
      onConfirm: () => checkAllMutation.mutate(),
    });
  };

  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Nodes</p><h1 className="mt-2 text-3xl font-semibold text-white">节点管理</h1></div>
      <Card>
        <CardHeader title="筛选" description="按订阅、分组、协议、健康状态和名称过滤节点。" action={<Button onClick={requestCheckAll} disabled={checkAllMutation.isPending}><RefreshCw className="h-4 w-4" />批量健康检查</Button>} />
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
          <Field label="搜索"><div className="relative"><Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" /><Input className="pl-9" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="节点名称" /></div></Field>
          <Field label="订阅"><Select value={filter.subscription_id ?? ''} onChange={(event) => updateFilter({ ...filter, subscription_id: Number(event.target.value) || undefined })}><option value="">全部</option>{(subscriptionsQuery.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</Select></Field>
          <Field label="分组"><Select value={filter.group_id ?? ''} onChange={(event) => updateFilter({ ...filter, group_id: Number(event.target.value) || undefined })}><option value="">全部</option>{(groupsQuery.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</Select></Field>
          <Field label="协议"><Input value={filter.protocol ?? ''} onChange={(event) => updateFilter({ ...filter, protocol: event.target.value || undefined })} placeholder="vmess" /></Field>
          <Field label="健康"><Select value={filter.alive_status ?? ''} onChange={(event) => updateFilter({ ...filter, alive_status: event.target.value as NodeFilter['alive_status'] })}><option value="">全部</option><option value="alive">可用</option><option value="dead">死亡</option><option value="unknown">未知</option></Select></Field>
          <Field label="启用"><Select value={filter.enabled === '' || filter.enabled === undefined ? '' : String(filter.enabled)} onChange={(event) => updateFilter({ ...filter, enabled: event.target.value === '' ? '' : event.target.value === 'true' })}><option value="">全部</option><option value="true">启用</option><option value="false">禁用</option></Select></Field>
        </div>
      </Card>
      <Card>
        <CardHeader title="节点列表" description={`当前 ${nodes.length} 个节点，已选 ${selected.length} 个。`} action={<BatchActions selected={selected} groups={groupsQuery.data ?? []} onBatch={requestBatch} />} />
        {nodesQuery.isLoading ? <LoadingState /> : (
          <DataTable columns={['选择', '名称', '协议', '健康', 'sing-box', '延迟', '服务器', '操作']} empty={!nodes.length}>
            {nodes.map((node) => (
              <tr key={node.id} className="hover:bg-slate-900/50">
                <td className="px-4 py-3"><input type="checkbox" checked={selected.includes(node.id)} onChange={(event) => setSelected((prev) => event.target.checked ? [...prev, node.id] : prev.filter((id) => id !== node.id))} /></td>
                <td className="px-4 py-3"><button className="text-left font-medium text-blue-200 hover:text-blue-100" onClick={() => setDetail(node)}>{node.name}</button><div className="mt-1 text-xs text-slate-500">{node.enabled ? '启用' : '禁用'} / 失败 {node.fail_count}</div></td>
                <td className="px-4 py-3 font-mono text-xs">{node.protocol}</td>
                <td className="px-4 py-3"><Badge value={node.alive_status} /></td>
                <td className="px-4 py-3"><Badge value={node.sing_box_status} /></td>
                <td className="px-4 py-3 font-mono text-xs">{node.latency_ms ? `${node.latency_ms}ms` : '—'}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-400">{node.server}:{node.port}</td>
                <td className="px-4 py-3"><div className="flex flex-wrap gap-2"><Button disabled={checkMutation.isPending} onClick={() => checkMutation.mutate(node.id)}>检查</Button><Button disabled={setEnabledMutation.isPending} onClick={() => setEnabledMutation.mutate({ id: node.id, enabled: !node.enabled })}>{node.enabled ? '禁用' : '启用'}</Button><Button disabled={rebuildMutation.isPending} onClick={() => rebuildMutation.mutate(node.id)}><RotateCcw className="h-4 w-4" />重建</Button></div></td>
              </tr>
            ))}
            {nodes.length ? <tr><td className="px-4 py-3"><input type="checkbox" checked={allSelected} onChange={(event) => setSelected(event.target.checked ? nodes.map((node) => node.id) : [])} /></td><td className="px-4 py-3 text-xs text-slate-500" colSpan={7}>全选当前筛选结果</td></tr> : null}
          </DataTable>
        )}
      </Card>
      <Drawer open={Boolean(detail)} title={detail?.name ?? '节点详情'} onClose={() => setDetail(null)}>
        {detail ? <NodeDetail node={detail} /> : null}
      </Drawer>
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}

function BatchActions({ selected, groups, onBatch }: { selected: number[]; groups: Array<{ id: number; name: string }>; onBatch: (action: NodeBatchAction, groupID?: number) => void }) {
  const [groupID, setGroupID] = useState<number>(0);
  const disabled = selected.length === 0;
  return <><Button disabled={disabled} onClick={() => onBatch('enable')}>批量启用</Button><Button disabled={disabled} onClick={() => onBatch('disable')}>批量禁用</Button><Select className="w-40" value={groupID} onChange={(event) => setGroupID(Number(event.target.value))}><option value={0}>选择分组</option>{groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}</Select><Button disabled={disabled || !groupID} onClick={() => onBatch('add_group', groupID)}>加入分组</Button><Button disabled={disabled || !groupID} onClick={() => onBatch('remove_group', groupID)}>移出分组</Button></>;
}

function NodeDetail({ node }: { node: ProxyNode }) {
  return <div className="space-y-5"><div className="grid gap-3 md:grid-cols-2"><Detail label="协议" value={node.protocol} /><Detail label="服务器" value={`${node.server}:${node.port}`} /><Detail label="健康" value={node.alive_status} /><Detail label="sing-box" value={node.sing_box_status} /><Detail label="传输" value={node.transport_type || '—'} /><Detail label="最后检查" value={formatTime(node.last_checked_at)} /></div>{node.sing_box_error ? <div className="rounded-2xl border border-red-400/30 bg-red-500/10 p-4 text-sm text-red-200">{node.sing_box_error}</div> : null}<div><h3 className="mb-2 text-sm font-semibold text-slate-300">Raw Config</h3><JsonBlock value={node.raw_config_json} /></div><div><h3 className="mb-2 text-sm font-semibold text-slate-300">sing-box Outbound</h3><JsonBlock value={node.sing_box_outbound_json} /></div></div>;
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl border border-slate-800 bg-slate-900/40 p-4"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 font-mono text-sm text-slate-200">{value}</div></div>;
}
