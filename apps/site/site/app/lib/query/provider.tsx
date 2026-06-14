import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';

import type { PropsWithChildren } from 'react';

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false
      }
    }
  });

export function QueryProvider({ children }: PropsWithChildren) {
  const [queryClient] = useState(createQueryClient);

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
