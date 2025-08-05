'use client'

import Link from 'next/link'
import { Workflow, ExternalLink } from 'lucide-react'
import { useTranslations, useLocale } from 'next-intl'
import { Button } from '@/components/ui/button'
import { LanguageThemeSwitcher } from './language-theme-switcher'

export function Header() {
  const t = useTranslations()
  const locale = useLocale()

  return (
    <header className="border-b border-border bg-card">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <Link href={`/${locale}`} className="flex items-center space-x-3">
            <div className="flex items-center justify-center w-8 h-8 bg-primary rounded-lg">
              <Workflow className="h-5 w-5 text-primary-foreground" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-foreground">{t('header.title')}</h1>
              <p className="text-xs text-muted-foreground">{t('header.subtitle')}</p>
            </div>
          </Link>

          <div className="flex items-center space-x-4">
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.open('http://localhost:8080/docs', '_blank')}
              className="hidden sm:flex"
            >
              <ExternalLink className="h-4 w-4 mr-2" />
              API
            </Button>

            <LanguageThemeSwitcher />
          </div>
        </div>
      </div>
    </header>
  )
}
