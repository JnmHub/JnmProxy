import { apiRequest } from './client';
import type { TrafficOverview } from './types';

export function getTrafficOverview() {
  return apiRequest<TrafficOverview>('/stats/overview');
}
