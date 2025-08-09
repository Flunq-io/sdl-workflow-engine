import { ReactNode } from 'react';
import { Providers } from '@/components/providers';
import { Header } from '@/components/header';
import { NextIntlClientProvider } from 'next-intl';

interface LayoutProps {
  children: ReactNode;
  params: Promise<{ tenant: string; locale: string }>;
}

async function loadMessages(locale: string) {
  try {
    const messages = (await import(`../../../../messages/${locale}.json`)).default
    return messages
  } catch {
    const fallback = (await import(`../../../../messages/en.json`)).default
    return fallback
  }
}

export default async function TenantLayout({ children, params }: LayoutProps) {
  const { tenant, locale } = await params;
  const messages = await loadMessages(locale)

  return (
    <Providers>
      <NextIntlClientProvider locale={locale} messages={messages}>
        <div className="min-h-screen bg-background">
          <Header tenant={tenant} locale={locale} />
          <main className="container mx-auto py-6">
            {children}
          </main>
        </div>
      </NextIntlClientProvider>
    </Providers>
  );
}
