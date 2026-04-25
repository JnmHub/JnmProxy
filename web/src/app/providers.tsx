import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { PropsWithChildren } from 'react';
import { BrowserRouter } from 'react-router-dom';
import { ToastViewport, notifyToast } from '../components/ui/Toast';

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return '请求失败，请稍后重试';
}

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => notifyToast('数据加载失败', errorMessage(error), 'error'),
  }),
  mutationCache: new MutationCache({
    onError: (error) => notifyToast('操作失败', errorMessage(error), 'error'),
    onSuccess: () => notifyToast('操作成功', undefined, 'success'),
  }),
  defaultOptions: {
    queries: {
      staleTime: 20_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
      <ToastViewport />
    </QueryClientProvider>
  );
}
