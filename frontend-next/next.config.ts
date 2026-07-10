import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Produce a self-contained server.js for Docker
  output: "standalone",

  images: {
    remotePatterns: [
      { protocol: "https", hostname: "**" },
      { protocol: "http", hostname: "**" },
    ],
  },

  // Proxy /rest/* and /ws/* to the Go API.
  // In Docker:  API_URL=http://api:8080 (set by docker-compose, never sent to browser)
  // Local dev:  falls back to http://localhost:8080
  async rewrites() {
    const apiUrl =
      process.env.API_URL ??
      process.env.NEXT_PUBLIC_API_URL ??
      "http://localhost:8080";

    return [
      { source: "/rest/:path*", destination: `${apiUrl}/rest/:path*` },
      { source: "/ws/:path*",   destination: `${apiUrl}/ws/:path*`   },
    ];
  },
};

export default nextConfig;
