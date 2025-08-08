"use client"

import { useTheme } from "next-themes"
import { useRouter, usePathname } from "next/navigation"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { ClientOnly } from "@/components/client-only"
import {
  Globe,
  Moon,
  Sun,
  Monitor,
  Settings
} from "lucide-react"

const languages = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'fr', name: 'Français', flag: '🇫🇷' },
  { code: 'de', name: 'Deutsch', flag: '🇩🇪' },
  { code: 'es', name: 'Español', flag: '🇪🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' },
  { code: 'nl', name: 'Nederlands', flag: '🇳🇱' },
]

export function LanguageThemeSwitcher() {
  const { theme, setTheme } = useTheme()
  const router = useRouter()
  const pathname = usePathname()

  // Extract locale from pathname (format: /tenant/locale/...)
  const pathSegments = pathname.split('/').filter(Boolean)
  const locale = pathSegments.length >= 2 ? pathSegments[1] : 'en'
  const tenant = pathSegments.length >= 1 ? pathSegments[0] : 'acme-inc'

  const currentLanguage = languages.find(lang => lang.code === locale)

  const switchLanguage = (newLocale: string) => {
    // Update the locale in the URL (format: /tenant/locale/...)
    const segments = pathname.split('/').filter(Boolean)
    if (segments.length >= 2) {
      segments[1] = newLocale // Replace locale
    } else if (segments.length === 1) {
      segments.push(newLocale) // Add locale
    } else {
      segments.push(tenant, newLocale) // Add both tenant and locale
    }
    router.push('/' + segments.join('/'))
  }

  const getThemeIcon = () => {
    switch (theme) {
      case 'light':
        return <Sun className="h-4 w-4" />
      case 'dark':
        return <Moon className="h-4 w-4" />
      default:
        return <Monitor className="h-4 w-4" />
    }
  }

  return (
    <div className="flex items-center gap-2">
      {/* Language Switcher */}
      <DropdownMenu id="language-switcher">
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-2">
            <Globe className="h-4 w-4" />
            <span className="hidden sm:inline">{currentLanguage?.flag}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>Select language</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {languages.map((language) => (
            <DropdownMenuItem
              key={language.code}
              onClick={() => switchLanguage(language.code)}
              className={locale === language.code ? 'bg-accent' : ''}
            >
              <span className="mr-2">{language.flag}</span>
              {language.name}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Theme Switcher */}
      <ClientOnly fallback={
        <Button variant="ghost" size="sm">
          <Monitor className="h-4 w-4" />
        </Button>
      }>
        <DropdownMenu id="theme-switcher">
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm">
              {getThemeIcon()}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Toggle theme</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => setTheme('light')}>
              <Sun className="mr-2 h-4 w-4" />
              Light
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme('dark')}>
              <Moon className="mr-2 h-4 w-4" />
              Dark
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setTheme('system')}>
              <Monitor className="mr-2 h-4 w-4" />
              System
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </ClientOnly>
    </div>
  )
}
