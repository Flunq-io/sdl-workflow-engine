'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { Workflow, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { LanguageThemeSwitcher } from './language-theme-switcher'

interface HeaderProps {
  tenant?: string;
  locale?: string;
}

export function Header({ tenant, locale }: HeaderProps = {}) {
  const pathname = usePathname()

  const isWorkflowsActive = pathname?.includes('/workflows')
  const isExecutionsActive = pathname?.includes('/executions')

  return (
    <header className="border-b border-border bg-card">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <Link href={tenant ? `/${tenant}/${locale}` : `/${locale}`} className="flex items-center space-x-3">
            <div className="flex items-center justify-center w-8 h-8 bg-primary rounded-lg">
              <Workflow className="h-5 w-5 text-primary-foreground" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-foreground">
                Flunq.io {tenant && `- ${tenant}`}
              </h1>
              <p className="text-xs text-muted-foreground">Execution Monitor</p>
            </div>
          </Link>

          {tenant && (
            <nav className="hidden md:flex items-center space-x-4">
              <Link
                href={`/${tenant}/${locale}/workflows`}
                className={`text-sm font-medium transition-colors ${
                  isWorkflowsActive
                    ? 'text-foreground border-b-2 border-primary pb-1'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                Workflows
              </Link>
              <Link
                href={`/${tenant}/${locale}/executions`}
                className={`text-sm font-medium transition-colors ${
                  isExecutionsActive
                    ? 'text-foreground border-b-2 border-primary pb-1'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                Executions
              </Link>
            </nav>
          )}

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
