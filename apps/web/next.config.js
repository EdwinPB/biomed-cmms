/** @type {import('next').NextConfig} */
if (process.env.NODE_ENV === "production" && !process.env.NEXT_PUBLIC_API_URL) {
  throw new Error(
    "NEXT_PUBLIC_API_URL is required for production builds. " +
      "Set it in the build environment, e.g. " +
      "NEXT_PUBLIC_API_URL=https://api.example.com npm run build",
  );
}

const nextConfig = {
  reactStrictMode: true,
  output: "standalone",
};

module.exports = nextConfig;
