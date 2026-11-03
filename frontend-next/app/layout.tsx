import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";
import { requestLocale } from "@/lib/i18n/server";
import { translate } from "@/lib/i18n/translate";

// Server-side locale for the first paint: see requestLocale (cookie →
// Accept-Language → en); the client re-resolves once /myself loads.

export async function generateMetadata(): Promise<Metadata> {
  const locale = await requestLocale();
  return {
    title: "Heureum",
    description: translate(locale, "layout.meta.description"),
  };
}

const noFlashScript = `(function(){try{var t=localStorage.getItem('theme');if(t!=='light'&&t!=='dark'){t=window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}if(t==='dark'){document.documentElement.classList.add('dark');}}catch(e){}})();`;

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const locale = await requestLocale();
  return (
    <html lang={locale} className="h-full" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashScript }} />
      </head>
      <body className="h-full bg-[#f8f9fc] text-[#1a1f36] dark:bg-[#0e1116] dark:text-[#e6e8eb]">
        <Providers initialLocale={locale}>{children}</Providers>
      </body>
    </html>
  );
}
