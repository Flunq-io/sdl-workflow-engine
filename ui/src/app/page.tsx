import { redirect } from 'next/navigation'

export default function RootPage() {
  // Redirect to default tenant and locale
  const tenant = process.env.NEXT_PUBLIC_DEFAULT_TENANT || 'tenant'
  const locale = process.env.NEXT_PUBLIC_DEFAULT_LOCALE || 'en'
  redirect(`/${tenant}/${locale}`)
}
