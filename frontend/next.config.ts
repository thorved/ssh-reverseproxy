import type { NextConfig } from "next";
import { PHASE_DEVELOPMENT_SERVER } from "next/constants";

const devApiUrl =
  process.env.NEXT_PUBLIC_DEV_API_URL?.trim() ?? "http://127.0.0.1:8080";

export default function nextConfig(phase: string): NextConfig {
  const config: NextConfig = {
    output: "export",
    trailingSlash: true,
    images: {
      unoptimized: true,
    },
  };

  if (phase === PHASE_DEVELOPMENT_SERVER) {
    config.rewrites = async () => [
      {
        source: "/api/:path*",
        destination: `${devApiUrl}/api/:path*`,
      },
    ];
  }

  return config;
}
