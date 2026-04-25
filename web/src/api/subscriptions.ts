import { apiRequest } from './client';
import type { ProxyNode, Subscription, SubscriptionRefreshLog, SubscriptionRefreshResult } from './types';

export interface SubscriptionInput {
  name: string;
  url: string;
  user_agent?: string;
  refresh_interval_seconds: number;
  enabled?: boolean;
}

export type SubscriptionUpdateInput = Partial<SubscriptionInput>;

export function listSubscriptions() {
  return apiRequest<Subscription[]>('/subscriptions');
}

export function getSubscription(id: number) {
  return apiRequest<Subscription>(`/subscriptions/${id}`);
}

export function createSubscription(input: SubscriptionInput) {
  return apiRequest<Subscription>('/subscriptions', { method: 'POST', body: input });
}

export function updateSubscription(id: number, input: SubscriptionUpdateInput) {
  return apiRequest<Subscription>(`/subscriptions/${id}`, { method: 'PUT', body: input });
}

export function deleteSubscription(id: number) {
  return apiRequest<void>(`/subscriptions/${id}`, { method: 'DELETE' });
}

export function refreshSubscription(id: number) {
  return apiRequest<SubscriptionRefreshResult>(`/subscriptions/${id}/refresh`, { method: 'POST', body: {} });
}

export function listSubscriptionLogs(id: number) {
  return apiRequest<SubscriptionRefreshLog[]>(`/subscriptions/${id}/refresh-logs`);
}

export function listSubscriptionNodes(id: number) {
  return apiRequest<ProxyNode[]>(`/subscriptions/${id}/nodes`);
}
