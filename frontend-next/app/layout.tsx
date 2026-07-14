import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "Heureum",
  description: "Modern project management",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full">
      <body className="h-full bg-[#f8f9fc] text-[#1a1f36]">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
