import type {NextConfig} from "next";
import path from "path";

const isProduction = process.env.NODE_ENV === 'production';

// Where `next dev` forwards /v1.0/* to. Deployed environments do NOT do this any
// more — the frontend is static on Cloudflare and the browser calls the API host
// directly, with CORS. The dev rewrite stays only so local work needs no CORS
// configuration; it is the one place where dev and prod differ on purpose.
const DEV_API_ORIGIN = process.env.DEV_API_ORIGIN || 'http://localhost:8000';

// rewrites() is unsupported by `output: 'export'` and only ever runs under
// `next dev`. Keeping the two mutually exclusive is what lets the dev server
// proxy the API while the production build stays a pure static export.
const nextConfig: NextConfig = {
  turbopack: {
    root: path.join(__dirname),
  },
  allowedDevOrigins: ['127.0.0.1'],
  ...(isProduction
    ? {
      output: 'export', images: {unoptimized: true,}
    }
    : {
      async rewrites() {
        return [
          {source: '/v1.0/:path*', destination: `${DEV_API_ORIGIN}/v1.0/:path*`},
          // OpenAPI spec + Stoplight page. Served by the API outside /v1.0.
          // Deployed environments reach them at dfe-api[-env].aoctech.app/docs;
          // this rewrite is a local convenience only.
          ...['/docs', '/openapi.json', '/openapi.yaml'].map((source) => ({
            source,
            destination: `${DEV_API_ORIGIN}${source}`,
          })),
        ];
      },
    }),
};

export default nextConfig;
