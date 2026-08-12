import type { NextConfig } from "next";
import { PHASE_DEVELOPMENT_SERVER } from "next/constants";

const createNextConfig = (phase: string): NextConfig => ({
  reactCompiler: true,
  ...(phase === PHASE_DEVELOPMENT_SERVER
    ? {
        allowedDevOrigins: ["localhost", "127.0.0.1", "::1"],
        async rewrites() {
          const apiBaseUrl = process.env.OCTOPUS_API_BASE_URL || "http://localhost:8080";
          return [
            {
              source: "/api/:path*",
              destination: `${apiBaseUrl}/api/:path*`,
            },
          ];
        },
      }
    : {
        output: "export",
        assetPrefix: "./",
      }),
});

export default createNextConfig;
