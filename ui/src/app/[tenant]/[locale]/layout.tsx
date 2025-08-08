import { ReactNode } from 'react';
import { Providers } from '@/components/providers';
import { Header } from '@/components/header';

interface LayoutProps {
  children: ReactNode;
  params: {
    tenant: string;
    locale: string;
  };
}

export default function TenantLayout({ children, params }: LayoutProps) {
  const { tenant, locale } = params;

  return (
    <Providers>
      <div className="min-h-screen bg-background">
        <Header tenant={tenant} locale={locale} />
        <main className="container mx-auto py-6">
          {children}
        </main>
      </div>
    </Providers>
  );
}
