import { apiRequest } from './client';
import type { BindMode, Credential, CredentialBinding, SelectionPolicy } from './types';

export interface CredentialInput {
  username: string;
  password: string;
  enabled?: boolean;
  bind_mode: BindMode;
  selection_policy: SelectionPolicy;
  remark?: string;
  bindings?: CredentialBinding[];
}

export interface CredentialUpdateInput {
  enabled?: boolean;
  bind_mode?: BindMode;
  selection_policy?: SelectionPolicy;
  remark?: string;
  bindings?: CredentialBinding[];
}

export function listCredentials() {
  return apiRequest<Credential[]>('/credentials');
}

export function getCredential(id: number) {
  return apiRequest<Credential>(`/credentials/${id}`);
}

export function createCredential(input: CredentialInput) {
  return apiRequest<Credential>('/credentials', { method: 'POST', body: input });
}

export function updateCredential(id: number, input: CredentialUpdateInput) {
  return apiRequest<Credential>(`/credentials/${id}`, { method: 'PUT', body: input });
}

export function resetCredentialPassword(id: number, password: string) {
  return apiRequest<void>(`/credentials/${id}/reset-password`, { method: 'POST', body: { password } });
}

export function deleteCredential(id: number) {
  return apiRequest<void>(`/credentials/${id}`, { method: 'DELETE' });
}
