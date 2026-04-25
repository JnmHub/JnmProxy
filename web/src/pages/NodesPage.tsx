import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, Eye, Filter, RefreshCw, RotateCcw, Search, ShieldCheck } from 'lucide-react';
import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { batchNodes, checkAllNodes, checkNode, listNodePage, listRuntimeNodes, rebuildNodeAdapter, setNodeEnabled, type NodeBatchAction, type NodeFilter } from '../api/nodes';
import { listGroups } from '../api/groups';
import { listSubscriptions } from '../api/subscriptions';
import type { ProxyGroup, ProxyNode, RuntimeNodeState } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { ConfirmDialog, type ConfirmState } from '../components/ui/ConfirmDialog';
import { Drawer } from '../components/ui/Drawer';
import { Field, Input, Select } from '../components/ui/Input';
import { JsonBlock } from '../components/ui/JsonBlock';
import { LoadingState } from '../components/ui/LoadingState';
import { PaginationBar } from '../components/ui/Pagination';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';
import { statusLabel } from '../utils/status';

export function NodesPage() {
  const [params] = useSearchParams();
  const searchParam = params.get('search') ?? '';
  const [search, setSearch] = useState(searchParam);
  const deferredSearch = useDeferredValue(search.trim());
  const [region, setRegion] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [filter, setFilter] = useState<NodeFilter>({ subscription_id: Number(params.get('subscription_id')) || undefined });
  const [selected, setSelected] = useState<number[]>([]);
  const [detail, setDetail] = useState<ProxyNode | null>(null);
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const queryClient = useQueryClient();

  const pageFilter = useMemo(() => ({
    ...filter,
    search: deferredSearch || undefined,
    region: region || undefined,
    page,
    page_size: pageSize,
  }), [deferredSearch, filter, page, pageSize, region]);
  const nodesQuery = useQuery({ queryKey: ['nodes', 'page', pageFilter], queryFn: () => listNodePage(pageFilter) });
  const runtimeQuery = useQuery({ queryKey: ['runtime', 'nodes'], queryFn: listRuntimeNodes, refetchInterval: 5_000 });
  const subscriptionsQuery = useQuery({ queryKey: ['subscriptions'], queryFn: listSubscriptions });
  const groupsQuery = useQuery({ queryKey: ['groups'], queryFn: listGroups });

  const invalidate = () => { void queryClient.invalidateQueries({ queryKey: ['nodes'] }); };
  const setEnabledMutation = useMutation({ mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => setNodeEnabled(id, enabled), onSuccess: invalidate });
  const checkMutation = useMutation({ mutationFn: checkNode, onSuccess: invalidate });
  const checkAllMutation = useMutation({ mutationFn: checkAllNodes, onSuccess: invalidate });
  const checkSelectedMutation = useMutation({
    mutationFn: async () => Promise.all(selected.map((id) => checkNode(id))),
    onSuccess: () => { setSelected([]); invalidate(); },
  });
  const rebuildMutation = useMutation({ mutationFn: rebuildNodeAdapter, onSuccess: invalidate });
  const batchMutation = useMutation({
    mutationFn: ({ action, groupID }: { action: NodeBatchAction; groupID?: number }) => batchNodes(action, selected, groupID),
    onSuccess: () => { setSelected([]); invalidate(); },
  });

  const nodePage = nodesQuery.data;
  const nodes = nodePage?.items ?? [];
  const total = nodePage?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const protocolOptions = commonProtocols;
  const subscriptionNames = useMemo(() => new Map((subscriptionsQuery.data ?? []).map((item) => [item.id, item.name])), [subscriptionsQuery.data]);
  const groupNames = useMemo(() => new Map((groupsQuery.data ?? []).map((group) => [group.id, group.name])), [groupsQuery.data]);
  const runtimeByNodeID = useMemo(() => new Map((runtimeQuery.data ?? []).map((item) => [item.node_id, item])), [runtimeQuery.data]);
  const summary = useMemo(() => nodeSummary(nodes), [nodes]);
  const allSelected = selected.length > 0 && selected.length === nodes.length;

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  useEffect(() => {
    setSearch(searchParam);
    setPage(1);
    setSelected([]);
  }, [searchParam]);

  const updateFilter = (next: NodeFilter) => {
    setFilter(next);
    setPage(1);
    setSelected([]);
  };
  const clearFilters = () => {
    setSearch('');
    setRegion('');
    updateFilter({});
  };
  const updateSearch = (value: string) => {
    setSearch(value);
    setPage(1);
    setSelected([]);
  };
  const updateRegion = (value: string) => {
    setRegion(value);
    setPage(1);
    setSelected([]);
  };
  const toggleSelected = (nodeID: number, checked: boolean) => {
    setSelected((prev) => checked ? uniqueNumbers([...prev, nodeID]) : prev.filter((id) => id !== nodeID));
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
      title: '全部健康检查',
      message: '确定立即检查所有可检查节点吗？节点较多时可能需要等待一会儿。',
      confirmText: '开始检查',
      onConfirm: () => checkAllMutation.mutate(),
    });
  };
  const requestCheckSelected = () => {
    setConfirm({
      title: '检查已选节点',
      message: `确定立即检查已选 ${selected.length} 个节点吗？`,
      confirmText: '开始检查',
      onConfirm: () => checkSelectedMutation.mutate(),
    });
  };

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">Nodes</p>
        <h1 className="mt-2 text-3xl font-semibold text-white">节点管理</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">按订阅、分组、协议、存活状态和 sing-box 状态筛选节点，快速定位不能进入代理池的原因。</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <SummaryCard title="匹配总数" value={total} hint="服务端筛选后的总节点" />
        <SummaryCard title="当前页" value={nodes.length} hint={`第 ${page} / ${totalPages} 页`} />
        <SummaryCard title="本页可用" value={summary.alive} hint="alive_status = alive" tone="success" />
        <SummaryCard title="本页死亡" value={summary.dead} hint="alive_status = dead" tone="danger" />
        <SummaryCard title="本页适配异常" value={summary.singBoxError} hint="需要查看错误原因" tone="warning" />
      </div>

      <Card>
        <CardHeader
          title="筛选"
          description="搜索、协议和地区都走服务端筛选；节点很多时不会一次性拉全量表格。"
          action={<Button onClick={requestCheckAll} disabled={checkAllMutation.isPending}><RefreshCw className="h-4 w-4" />全部健康检查</Button>}
        />
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-7">
          <Field label="搜索">
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
              <Input className="pl-9" value={search} onChange={(event) => updateSearch(event.target.value)} placeholder="名称 / 服务器 / 协议" />
            </div>
          </Field>
          <Field label="订阅">
            <Select value={filter.subscription_id ?? ''} onChange={(event) => updateFilter({ ...filter, subscription_id: Number(event.target.value) || undefined })}>
              <option value="">全部订阅</option>
              {(subscriptionsQuery.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
            </Select>
          </Field>
          <Field label="分组">
            <Select value={filter.group_id ?? ''} onChange={(event) => updateFilter({ ...filter, group_id: Number(event.target.value) || undefined })}>
              <option value="">全部分组</option>
              {(groupsQuery.data ?? []).map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}
            </Select>
          </Field>
          <Field label="协议">
            <Select value={filter.protocol ?? ''} onChange={(event) => updateFilter({ ...filter, protocol: event.target.value || undefined })}>
              <option value="">全部协议</option>
              {protocolOptions.map((protocol) => <option key={protocol} value={protocol}>{protocol}</option>)}
            </Select>
          </Field>
          <Field label="健康">
            <Select value={filter.alive_status ?? ''} onChange={(event) => updateFilter({ ...filter, alive_status: event.target.value as NodeFilter['alive_status'] })}>
              <option value="">全部健康</option>
              <option value="alive">可用</option>
              <option value="dead">死亡</option>
              <option value="unknown">未知</option>
            </Select>
          </Field>
          <Field label="sing-box">
            <Select value={filter.sing_box_status ?? ''} onChange={(event) => updateFilter({ ...filter, sing_box_status: event.target.value as NodeFilter['sing_box_status'] })}>
              <option value="">全部状态</option>
              <option value="supported">支持</option>
              <option value="unsupported">不支持</option>
              <option value="error">错误</option>
            </Select>
          </Field>
          <Field label="启用">
            <Select value={filter.enabled === '' || filter.enabled === undefined ? '' : String(filter.enabled)} onChange={(event) => updateFilter({ ...filter, enabled: event.target.value === '' ? '' : event.target.value === 'true' })}>
              <option value="">全部</option>
              <option value="true">启用</option>
              <option value="false">禁用</option>
            </Select>
          </Field>
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          {regionOptions.map((item) => (
            <Button key={item.value || 'all'} variant={region === item.value ? 'primary' : 'secondary'} onClick={() => updateRegion(item.value)}>
              {item.label}
            </Button>
          ))}
          <Button variant="ghost" onClick={clearFilters}><Filter className="h-4 w-4" />清空筛选</Button>
          <Button disabled={!selected.length || checkSelectedMutation.isPending} onClick={requestCheckSelected}><Activity className="h-4 w-4" />检查已选</Button>
        </div>
      </Card>

      <Card>
        <CardHeader
          title="节点列表"
          description={`共匹配 ${total} 个节点，本页 ${nodes.length} 个，已选 ${selected.length} 个。`}
          action={<BatchActions selected={selected} groups={groupsQuery.data ?? []} onBatch={requestBatch} />}
        />
        {nodesQuery.isLoading ? <LoadingState /> : (
          <>
            <DataTable columns={['选择', '节点', '来源', '状态', '协议', '延迟', '服务器', '操作']} empty={!nodes.length}>
            {nodes.map((node) => {
              const runtime = runtimeByNodeID.get(node.id);
              return (
              <tr key={node.id} className="hover:bg-slate-900/50">
                <td className="px-4 py-3">
                  <input type="checkbox" checked={selected.includes(node.id)} onChange={(event) => toggleSelected(node.id, event.target.checked)} />
                </td>
                <td className="px-4 py-3">
                  <button className="text-left font-medium text-blue-200 hover:text-blue-100" onClick={() => setDetail(node)}>{node.name}</button>
                  <div className="mt-1 text-xs text-slate-500">{node.enabled ? '启用' : '禁用'} / DB失败 {node.fail_count} / 运行失败 {runtime?.failure_count ?? 0}</div>
                </td>
                <td className="px-4 py-3 text-xs text-slate-400">
                  <div>{subscriptionNames.get(node.subscription_id) ?? `订阅 ${node.subscription_id}`}</div>
                  <div className="mt-1">{groupLabel(node.group_ids, groupNames)}</div>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Badge value={node.alive_status} />
                    <Badge value={node.sing_box_status}>SB {statusLabel(node.sing_box_status)}</Badge>
                    {runtime ? <Badge value={runtime.circuit_open ? 'failed' : runtime.in_candidate_pool ? 'supported' : 'unknown'}>{runtime.circuit_open ? '熔断中' : runtime.in_candidate_pool ? '候选池' : '未入池'}</Badge> : null}
                  </div>
                </td>
                <td className="px-4 py-3 font-mono text-xs">{node.protocol}</td>
                <td className="px-4 py-3 font-mono text-xs">{node.latency_ms ? `${node.latency_ms}ms` : '—'}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-400">{node.server}:{node.port}</td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2">
                    <Button onClick={() => setDetail(node)}><Eye className="h-4 w-4" />详情</Button>
                    <Button disabled={checkMutation.isPending} onClick={() => checkMutation.mutate(node.id)}>检查</Button>
                    <Button disabled={setEnabledMutation.isPending} onClick={() => setEnabledMutation.mutate({ id: node.id, enabled: !node.enabled })}>{node.enabled ? '禁用' : '启用'}</Button>
                    <Button disabled={rebuildMutation.isPending} onClick={() => rebuildMutation.mutate(node.id)}><RotateCcw className="h-4 w-4" />重建</Button>
                  </div>
                </td>
              </tr>
              );
            })}
            {nodes.length ? (
              <tr>
                <td className="px-4 py-3">
                  <input type="checkbox" checked={allSelected} onChange={(event) => setSelected(event.target.checked ? nodes.map((node) => node.id) : [])} />
                </td>
                <td className="px-4 py-3 text-xs text-slate-500" colSpan={7}>全选当前页</td>
              </tr>
            ) : null}
            </DataTable>
            <PaginationBar
              page={page}
              pageSize={pageSize}
              total={total}
              totalPages={totalPages}
              onPageChange={(nextPage) => { setPage(nextPage); setSelected([]); }}
              onPageSizeChange={(nextPageSize) => { setPageSize(nextPageSize); setPage(1); setSelected([]); }}
            />
          </>
        )}
      </Card>

      <Drawer open={Boolean(detail)} title={detail?.name ?? '节点详情'} onClose={() => setDetail(null)}>
        {detail ? <NodeDetail node={detail} runtime={runtimeByNodeID.get(detail.id)} subscriptionName={subscriptionNames.get(detail.subscription_id)} groupNames={groupNames} /> : null}
      </Drawer>
      <ConfirmDialog state={confirm} onClose={() => setConfirm(null)} />
    </div>
  );
}

function BatchActions({ selected, groups, onBatch }: { selected: number[]; groups: ProxyGroup[]; onBatch: (action: NodeBatchAction, groupID?: number) => void }) {
  const [groupID, setGroupID] = useState<number>(0);
  const disabled = selected.length === 0;
  return (
    <>
      <Button disabled={disabled} onClick={() => onBatch('enable')}>批量启用</Button>
      <Button disabled={disabled} onClick={() => onBatch('disable')}>批量禁用</Button>
      <Select className="w-40" value={groupID} onChange={(event) => setGroupID(Number(event.target.value))}>
        <option value={0}>选择分组</option>
        {groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}
      </Select>
      <Button disabled={disabled || !groupID} onClick={() => onBatch('add_group', groupID)}>加入分组</Button>
      <Button disabled={disabled || !groupID} onClick={() => onBatch('remove_group', groupID)}>移出分组</Button>
    </>
  );
}

function NodeDetail({ node, runtime, subscriptionName, groupNames }: { node: ProxyNode; runtime?: RuntimeNodeState; subscriptionName?: string; groupNames: Map<number, string> }) {
  const checks = runtimeChecks(node, runtime);
  return (
    <div className="space-y-5">
      <div className="grid gap-3 md:grid-cols-2">
        <Detail label="协议" value={node.protocol} />
        <Detail label="服务器" value={`${node.server}:${node.port}`} />
        <Detail label="订阅" value={subscriptionName ?? `订阅 ${node.subscription_id}`} />
        <Detail label="分组" value={groupLabel(node.group_ids, groupNames)} />
        <Detail label="健康" value={statusLabel(node.alive_status)} />
        <Detail label="sing-box" value={statusLabel(node.sing_box_status)} />
        <Detail label="adapter" value={statusLabel(node.adapter_status)} />
        <Detail label="传输" value={node.transport_type || '—'} />
        <Detail label="延迟" value={node.latency_ms ? `${node.latency_ms}ms` : '—'} />
        <Detail label="最后检查" value={formatTime(node.last_checked_at)} />
      </div>

      <div className="rounded-2xl border border-slate-800 bg-slate-950/60 p-4">
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-white"><Activity className="h-4 w-4 text-emerald-300" />运行态诊断</div>
        <div className="grid gap-3 md:grid-cols-2">
          <Detail label="内存候选池" value={runtime ? runtime.in_candidate_pool ? '在候选池' : '不在候选池' : '未加载'} />
          <Detail label="内存熔断" value={runtime?.circuit_open ? '熔断中' : '未熔断'} />
          <Detail label="连续失败" value={String(runtime?.failure_count ?? 0)} />
          <Detail label="熔断恢复" value={formatTime(runtime?.circuit_until)} />
          <Detail label="最近失败时间" value={formatTime(runtime?.last_failed_at)} />
          <Detail label="最近失败原因" value={runtime?.last_failure || '—'} />
        </div>
      </div>

      <div className="rounded-2xl border border-slate-800 bg-slate-950/60 p-4">
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-white"><ShieldCheck className="h-4 w-4 text-blue-300" />进入随机池检查</div>
        <div className="grid gap-2 md:grid-cols-2">
          {checks.map((check) => (
            <div key={check.label} className="flex items-start gap-2 rounded-xl border border-slate-800 bg-slate-900/40 px-3 py-2 text-xs">
              <Badge value={check.ok ? 'supported' : 'error'}>{check.ok ? '通过' : '阻塞'}</Badge>
              <span className="leading-5 text-slate-300">{check.label}</span>
            </div>
          ))}
        </div>
      </div>

      {node.sing_box_error ? <div className="rounded-2xl border border-red-400/30 bg-red-500/10 p-4 text-sm leading-6 text-red-200">{node.sing_box_error}</div> : null}
      <div>
        <h3 className="mb-2 text-sm font-semibold text-slate-300">Raw Config</h3>
        <JsonBlock value={node.raw_config_json} />
      </div>
      <div>
        <h3 className="mb-2 text-sm font-semibold text-slate-300">sing-box Outbound</h3>
        <JsonBlock value={node.sing_box_outbound_json} />
      </div>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/40 p-4">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-1 font-mono text-sm text-slate-200">{value}</div>
    </div>
  );
}

function SummaryCard({ title, value, hint, tone = 'default' }: { title: string; value: number; hint: string; tone?: 'default' | 'success' | 'warning' | 'danger' }) {
  const color = {
    default: 'text-white',
    success: 'text-emerald-200',
    warning: 'text-amber-200',
    danger: 'text-red-200',
  }[tone];
  return (
    <Card>
      <div className={`font-mono text-3xl font-semibold ${color}`}>{value}</div>
      <div className="mt-2 text-sm font-medium text-slate-200">{title}</div>
      <div className="mt-1 text-xs text-slate-500">{hint}</div>
    </Card>
  );
}

function nodeSummary(nodes: ProxyNode[]) {
  return {
    alive: nodes.filter((node) => node.alive_status === 'alive').length,
    dead: nodes.filter((node) => node.alive_status === 'dead').length,
    singBoxSupported: nodes.filter((node) => node.sing_box_status === 'supported').length,
    singBoxError: nodes.filter((node) => node.sing_box_status === 'error').length,
  };
}

function runtimeChecks(node: ProxyNode, runtime?: RuntimeNodeState) {
  return [
    { label: '节点已启用', ok: node.enabled },
    { label: 'adapter 或 sing-box 状态为支持', ok: node.adapter_status === 'supported' || node.sing_box_status === 'supported' },
    { label: '健康状态不是死亡', ok: node.alive_status !== 'dead' },
    { label: '当前没有内存熔断', ok: !runtime?.circuit_open },
    { label: '当前在运行候选池', ok: Boolean(runtime?.in_candidate_pool) },
  ];
}

function groupLabel(groupIDs: number[] | undefined, groupNames: Map<number, string>) {
  if (!groupIDs?.length) return '未分组';
  return groupIDs.map((id) => groupNames.get(id) ?? `分组 ${id}`).join('、');
}

function uniqueNumbers(values: number[]) {
  return [...new Set(values)];
}

const commonProtocols = ['http', 'https', 'socks', 'socks5', 'socks5h', 'ss', 'shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'hy2', 'tuic'];

const regionOptions = [
  { label: '全部地区', value: '' },
  { label: '香港', value: 'hk' },
  { label: '日本', value: 'jp' },
  { label: '美国', value: 'us' },
  { label: '新加坡', value: 'sg' },
  { label: '台湾', value: 'tw' },
  { label: '韩国', value: 'kr' },
];
