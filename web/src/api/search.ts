import { apiRequest } from './client';
import type { SearchResult } from './types';

export function globalSearch(query: string) {
  const params = new URLSearchParams();
  params.set('q', query);
  return apiRequest<SearchResult>(`/search?${params.toString()}`);
}
