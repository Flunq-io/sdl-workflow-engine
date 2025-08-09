/** @type {import('next').NextConfig} */
const withNextIntl = require('next-intl/plugin')()

const nextConfig = {
  images: {
    domains: ['localhost'],
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
  },
  turbopack: {
    // Silence deprecation notice; Turbopack is stable in Next 15
  },
}

module.exports = withNextIntl(nextConfig)
