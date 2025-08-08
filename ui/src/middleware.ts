import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'
import { locales } from './i18n'

// List of valid tenants (you can make this dynamic by fetching from your API)
const VALID_TENANTS = ['acme-inc', 'demo-tenant', 'test-tenant']

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
    return NextResponse.redirect(new URL('/acme-inc/en', request.url))
  }

  // Parse the pathname to extract tenant and locale
  const pathSegments = pathname.split('/').filter(Boolean)

  if (pathSegments.length < 2) {
    // If no tenant/locale specified, redirect to default
    return NextResponse.redirect(new URL('/acme-inc/en', request.url))
  }

  const [tenant, locale] = pathSegments

  // Validate tenant
  if (!VALID_TENANTS.includes(tenant)) {
    // Redirect to default tenant if invalid
    const newPath = pathname.replace(`/${tenant}`, '/acme-inc')
    return NextResponse.redirect(new URL(newPath, request.url))
  }

  // Validate locale
  if (!locales.includes(locale as any)) {
    // Redirect to default locale if invalid
    const newPath = pathname.replace(`/${tenant}/${locale}`, `/${tenant}/en`)
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
