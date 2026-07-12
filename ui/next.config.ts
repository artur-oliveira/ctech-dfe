import type {NextConfig} from "next";
import path from "path";

const isProduction = process.env.NODE_ENV === 'production';

// Where `next dev` forwards /v1.0/* to. Mirrors what CloudFront does in front of
// the ALB in deployed environments, so the browser is same-origin in dev too and
// CORS never applies.
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
    ? {output: 'export' as const}
    : {
      async rewrites() {
        return [
          {source: '/v1.0/:path*', destination: `${DEV_API_ORIGIN}/v1.0/:path*`},
        ];
      },
    }),
};

export default nextConfig;
