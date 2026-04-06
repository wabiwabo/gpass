/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/auth/:path*",
        destination: `${process.env.BFF_URL || "http://localhost:4000"}/auth/:path*`,
      },
      {
        source: "/api/:path*",
        destination: `${process.env.BFF_URL || "http://localhost:4000"}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
