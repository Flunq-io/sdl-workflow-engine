"use client"

import * as React from "react"
import { cn } from "@/lib/utils"
import { useGlobalDropdown } from "./dropdown-context"

interface DropdownMenuProps {
  children: React.ReactNode
  id: string // Unique identifier for this dropdown
}

interface DropdownMenuTriggerProps {
  children: React.ReactNode
  asChild?: boolean
  className?: string
  onClick?: () => void
}

interface DropdownMenuContentProps {
  children: React.ReactNode
  align?: 'start' | 'center' | 'end'
  className?: string
}

interface DropdownMenuItemProps {
  children: React.ReactNode
  onClick?: () => void
  className?: string
  disabled?: boolean
}

interface DropdownMenuLabelProps {
  children: React.ReactNode
  className?: string
}

interface DropdownMenuSeparatorProps {
  className?: string
}

const DropdownMenuContext = React.createContext<{
  isOpen: boolean
  setIsOpen: (open: boolean) => void
  dropdownId: string
}>({
  isOpen: false,
  setIsOpen: () => {},
  dropdownId: ''
})

export function DropdownMenu({ children, id }: DropdownMenuProps) {
  const { openDropdown, setOpenDropdown } = useGlobalDropdown()
  const isOpen = openDropdown === id

  const setIsOpen = (open: boolean) => {
    setOpenDropdown(open ? id : null)
  }

  return (
    <DropdownMenuContext.Provider value={{ isOpen, setIsOpen, dropdownId: id }}>
      <div className="relative" data-dropdown>
        {children}
      </div>
    </DropdownMenuContext.Provider>
  )
}

export function DropdownMenuTrigger({ children, asChild, className, onClick }: DropdownMenuTriggerProps) {
  const { isOpen, setIsOpen } = React.useContext(DropdownMenuContext)

  const handleClick = () => {
    setIsOpen(!isOpen)
    onClick?.()
  }

  if (asChild && React.isValidElement(children)) {
    type ChildProps = (React.ComponentProps<'a'> | React.ComponentProps<'button'>) & { className?: string; onClick?: (e: React.MouseEvent) => void }
    const child = children as React.ReactElement<ChildProps>
    return React.cloneElement(child, {
      ...(child.props as ChildProps),
      onClick: (e: React.MouseEvent) => {
        child.props?.onClick?.(e)
        handleClick()
      },
      'aria-expanded': isOpen,
      'aria-haspopup': true
    })
  }

  return (
    <button
      onClick={handleClick}
      className={className}
      aria-expanded={isOpen}
      aria-haspopup={true}
    >
      {children}
    </button>
  )
}

export function DropdownMenuContent({ children, align = 'end', className }: DropdownMenuContentProps) {
  const { isOpen } = React.useContext(DropdownMenuContext)

  if (!isOpen) return null

  const alignmentClasses = {
    start: 'left-0',
    center: 'left-1/2 -translate-x-1/2',
    end: 'right-0'
  }

  return (
    <div
      className={cn(
        "absolute top-full mt-1 z-50 min-w-[8rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md",
        alignmentClasses[align],
        className
      )}
    >
      {children}
    </div>
  )
}

export function DropdownMenuItem({ children, onClick, className, disabled }: DropdownMenuItemProps) {
  const { setIsOpen } = React.useContext(DropdownMenuContext)

  const handleClick = () => {
    if (!disabled) {
      onClick?.()
      setIsOpen(false)
    }
  }

  return (
    <div
      onClick={handleClick}
      className={cn(
        "relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground",
        disabled && "pointer-events-none opacity-50",
        className
      )}
    >
      {children}
    </div>
  )
}

export function DropdownMenuLabel({ children, className }: DropdownMenuLabelProps) {
  return (
    <div className={cn("px-2 py-1.5 text-sm font-semibold", className)}>
      {children}
    </div>
  )
}

export function DropdownMenuSeparator({ className }: DropdownMenuSeparatorProps) {
  return <div className={cn("-mx-1 my-1 h-px bg-muted", className)} />
}
