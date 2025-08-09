import {getRequestConfig} from 'next-intl/server'

export const locales = ['en', 'fr', 'de', 'es', 'zh', 'nl'] as const
export type Locale = typeof locales[number]

export default getRequestConfig(async () => ({
  locale: 'en',
  messages: (await import(`../../messages/en.json`)).default
}))

