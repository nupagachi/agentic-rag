import createNextIntlPlugin from "next-intl/plugin";
import type { NextConfig } from "next";

// SR-5: khong dat allowedDevOrigins. SR-14: khong dung khoa `env`;
// bien moi truong chi duoc doc trong lib/env.ts.
const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
};

const withNextIntl = createNextIntlPlugin("./i18n/request.ts");

export default withNextIntl(nextConfig);
