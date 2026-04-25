import { apiRequest } from './client';
import type { ProxyRequestLogPage } from './types';

export interface ProxyRequestLogFilter {
  search?: string;
  status?: string;
  entry_protocol?: string;
  page?: number;
  page_size?: number;
}

function queryString(filter: ProxyRequestLogFilter = {}) {
  const params = new URLSearchParams();
  Object.entries(filter).forEach(([key, value]) => {
    if (value === undefined || value === '' || value === 0) return;
    params.set(key, String(value));
  });
  const text = params.toString();
  return text ? `?${text}` : '';
}

export function listProxyRequestLogs(filter?: ProxyRequestLogFilter) {
  return apiRequest<ProxyRequestLogPage>(`/proxy-request-logs${queryString(filter)}`);
}
