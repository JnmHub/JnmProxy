import { apiRequest } from './client';
import type { AliveStatus, ProxyNode } from './types';

export interface NodeFilter {
  subscription_id?: number;
  group_id?: number;
  protocol?: string;
  alive_status?: AliveStatus | '';
  enabled?: boolean | '';
}

export type NodeBatchAction = 'enable' | 'disable' | 'add_group' | 'remove_group';

function queryString(filter: NodeFilter = {}) {
  const params = new URLSearchParams();
  Object.entries(filter).forEach(([key, value]) => {
    if (value === undefined || value === '' || value === 0) return;
    params.set(key, String(value));
  });
  const text = params.toString();
  return text ? `?${text}` : '';
}

export function listNodes(filter?: NodeFilter) {
  return apiRequest<ProxyNode[]>(`/nodes${queryString(filter)}`);
}

export function getNode(id: number) {
  return apiRequest<ProxyNode>(`/nodes/${id}`);
}

export function setNodeEnabled(id: number, enabled: boolean) {
  return apiRequest<void>(`/nodes/${id}`, { method: 'PUT', body: { enabled } });
}

export function checkNode(id: number) {
  return apiRequest<void>(`/nodes/${id}/check`, { method: 'POST', body: {} });
}

export function checkAllNodes() {
  return apiRequest<{ checked: number }>('/nodes/check', { method: 'POST', body: {} });
}

export function rebuildNodeAdapter(id: number) {
  return apiRequest<{ node_id: number; status: string }>(`/nodes/${id}/rebuild-adapter`, { method: 'POST', body: {} });
}

export function batchNodes(action: NodeBatchAction, nodeIDs: number[], groupID = 0) {
  return apiRequest<void>('/nodes/batch', { method: 'POST', body: { action, node_ids: nodeIDs, group_id: groupID } });
}
