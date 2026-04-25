import { apiRequest } from './client';
import type { SingBoxStatus, SystemHealth } from './types';

export function getSystemHealth() {
  return apiRequest<SystemHealth>('/system/health');
}

export function getSingBoxStatus() {
  return apiRequest<SingBoxStatus>('/system/sing-box');
}
