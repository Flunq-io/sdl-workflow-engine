import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'
import { locales } from './i18n'

// Default tenant and locale can be customized via env; do not hardcode tenants
const DEFAULT_TENANT = process.env.NEXT_PUBLIC_DEFAULT_TENANT || 'tenant'
const DEFAULT_LOCALE = process.env.NEXT_PUBLIC_DEFAULT_LOCALE || 'en'

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Skip middleware for static files, API routes, and Next.js internals
  if (
    pathname.startsWith('/_next') ||
    pathname.startsWith('/api') ||
    pathname.startsWith('/favicon.ico') ||
    pathname.includes('.')
  ) {
    return NextResponse.next()
  }

  // Handle root path - redirect to default tenant/locale
  if (pathname === '/') {
    return NextResponse.redirect(new URL(`/${DEFAULT_TENANT}/${DEFAULT_LOCALE}`, request.url))
  }

  // Parse the pathname to extract tenant and locale
  const pathSegments = pathname.split('/').filter(Boolean)

  if (pathSegments.length < 2) {
    // If no tenant/locale specified, redirect to default
    return NextResponse.redirect(new URL(`/${DEFAULT_TENANT}/${DEFAULT_LOCALE}`, request.url))
  }

  const [tenant, locale] = pathSegments

  // Do not enforce a fixed tenant allowlist here — allow any tenant string

  // Validate locale only
  if (!(locales as readonly string[]).includes(locale)) {
    // Redirect to default locale if invalid
    const newPath = pathname.replace(`/${tenant}/${locale}`, `/${tenant}/${DEFAULT_LOCALE}`)
    return NextResponse.redirect(new URL(newPath, request.url))
  }

  // Add tenant and locale to headers for use in components
  const response = NextResponse.next()
  response.headers.set('x-tenant', tenant)
  response.headers.set('x-locale', locale)

  return response
}

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     */
    '/((?!api|_next/static|_next/image|favicon.ico).*)',
  ],
}
