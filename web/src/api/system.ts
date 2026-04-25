import { apiRequest } from './client';
import type { SingBoxStatus, SystemHealth, SystemProxyStatus } from './types';

export function getSystemHealth() {
  return apiRequest<SystemHealth>('/system/health');
}

export function getSingBoxStatus() {
  return apiRequest<SingBoxStatus>('/system/sing-box');
}

export function getProxyStatus() {
  return apiRequest<SystemProxyStatus>('/system/proxy');
}
