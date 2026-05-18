import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Open Jira",
  description: "Modern project management",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full">
      <body className="h-full bg-[#f8f9fc] text-[#1a1f36]">{children}</body>
    </html>
  );
}
