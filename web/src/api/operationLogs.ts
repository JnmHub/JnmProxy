import { apiRequest } from './client';
import type { OperationLogPage } from './types';

export interface OperationLogFilter {
  action?: string;
  target_type?: string;
  search?: string;
  page?: number;
  page_size?: number;
}

function queryString(filter: OperationLogFilter = {}) {
  const params = new URLSearchParams();
  Object.entries(filter).forEach(([key, value]) => {
    if (value === undefined || value === '' || value === 0) return;
    params.set(key, String(value));
  });
  const text = params.toString();
  return text ? `?${text}` : '';
}

export function listOperationLogs(filter?: OperationLogFilter) {
  return apiRequest<OperationLogPage>(`/operation-logs${queryString(filter)}`);
}
