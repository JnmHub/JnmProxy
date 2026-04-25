import { apiRequest } from './client';
import type { GroupKeyword, ProxyGroup } from './types';

export interface GroupInput {
  name: string;
  description?: string;
  auto_created?: boolean;
}

export interface KeywordInput {
  name: string;
  keywords: string;
  case_sensitive: boolean;
  enabled?: boolean;
}

export interface ApplyKeywordsResult {
  rules_scanned: number;
  nodes_scanned: number;
  groups_touched: number;
  relations_touched: number;
}

export function listGroups() {
  return apiRequest<ProxyGroup[]>('/groups');
}

export function createGroup(input: GroupInput) {
  return apiRequest<ProxyGroup>('/groups', { method: 'POST', body: input });
}

export function updateGroup(id: number, input: Partial<GroupInput>) {
  return apiRequest<ProxyGroup>(`/groups/${id}`, { method: 'PUT', body: input });
}

export function deleteGroup(id: number) {
  return apiRequest<void>(`/groups/${id}`, { method: 'DELETE' });
}

export function addNodesToGroup(id: number, nodeIDs: number[]) {
  return apiRequest<void>(`/groups/${id}/nodes`, { method: 'POST', body: { node_ids: nodeIDs } });
}

export function removeNodesFromGroup(id: number, nodeIDs: number[]) {
  return apiRequest<void>(`/groups/${id}/nodes`, { method: 'DELETE', body: { node_ids: nodeIDs } });
}

export function listKeywordRules() {
  return apiRequest<GroupKeyword[]>('/group-keywords');
}

export function createKeywordRule(input: KeywordInput) {
  return apiRequest<GroupKeyword>('/group-keywords', { method: 'POST', body: input });
}

export function updateKeywordRule(id: number, input: Partial<KeywordInput>) {
  return apiRequest<GroupKeyword>(`/group-keywords/${id}`, { method: 'PUT', body: input });
}

export function deleteKeywordRule(id: number) {
  return apiRequest<void>(`/group-keywords/${id}`, { method: 'DELETE' });
}

export function applyKeywordRules(ruleIDs: number[], all: boolean) {
  return apiRequest<ApplyKeywordsResult>('/group-keywords/apply', { method: 'POST', body: { rule_ids: ruleIDs, all } });
}
