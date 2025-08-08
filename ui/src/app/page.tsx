import { redirect } from 'next/navigation'

export default function RootPage() {
  // Redirect to default tenant and locale
  redirect('/acme-inc/en')
}
