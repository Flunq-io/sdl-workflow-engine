import createMiddleware from 'next-intl/middleware'
import { locales } from './i18n'

export default createMiddleware({
  // A list of all locales that are supported
  locales,

  // Used when no locale matches
  defaultLocale: 'en',

  // Automatically detect the locale from the browser
  localeDetection: true
})

export const config = {
  // Match only internationalized pathnames
  matcher: ['/', '/(de|en|es|fr|nl|zh)/:path*']
}
