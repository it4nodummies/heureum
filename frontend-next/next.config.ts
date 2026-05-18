import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Allow images from any origin for avatar URLs
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "**" },
      { protocol: "http", hostname: "**" },
    ],
  },
  // Proxy API calls to backend in development
  async rewrites() {
    return [
      {
        source: "/rest/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/rest/:path*`,
      },
    ];
  },
};

export default nextConfig;
