import type {NextConfig} from "next";
import path from "path";

const nextConfig: NextConfig = {
  turbopack: {
    root: path.join(__dirname),
  },
  output: 'export',
  allowedDevOrigins: ['127.0.0.1'],
};

export default nextConfig;
