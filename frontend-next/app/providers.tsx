"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { ThemeProvider } from "@/components/layout/ThemeProvider";
import { I18nProvider } from "@/lib/i18n/I18nProvider";
import type { Locale } from "@/lib/i18n/types";

export function Providers({ children, initialLocale }: { children: React.ReactNode; initialLocale: Locale }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { staleTime: 30_000, retry: 1 },
        },
      })
  );
  return (
    <QueryClientProvider client={queryClient}>
      <I18nProvider initialLocale={initialLocale}>
        <ThemeProvider>{children}</ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>
  );
}
