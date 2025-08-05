"use client"

import * as React from "react"

interface GlobalDropdownContextType {
  openDropdown: string | null
  setOpenDropdown: (id: string | null) => void
}

const GlobalDropdownContext = React.createContext<GlobalDropdownContextType>({
  openDropdown: null,
  setOpenDropdown: () => {}
})

export function GlobalDropdownProvider({ children }: { children: React.ReactNode }) {
  const [openDropdown, setOpenDropdown] = React.useState<string | null>(null)

  React.useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Element
      if (!target.closest('[data-dropdown]')) {
        setOpenDropdown(null)
      }
    }

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpenDropdown(null)
      }
    }

    if (openDropdown) {
      document.addEventListener('click', handleClickOutside)
      document.addEventListener('keydown', handleEscape)
      return () => {
        document.removeEventListener('click', handleClickOutside)
        document.removeEventListener('keydown', handleEscape)
      }
    }
  }, [openDropdown])

  return (
    <GlobalDropdownContext.Provider value={{ openDropdown, setOpenDropdown }}>
      {children}
    </GlobalDropdownContext.Provider>
  )
}

export function useGlobalDropdown() {
  const context = React.useContext(GlobalDropdownContext)
  if (!context) {
    throw new Error('useGlobalDropdown must be used within a GlobalDropdownProvider')
  }
  return context
}
