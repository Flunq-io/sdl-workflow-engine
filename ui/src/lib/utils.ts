import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  if (ms < 3600000) return `${(ms / 60000).toFixed(1)}m`
  return `${(ms / 3600000).toFixed(1)}h`
}

export function formatRelativeTime(date: Date | string, locale?: string): string {
  // Handle both Date objects and date strings
  const parsedDate = typeof date === 'string' ? new Date(date) : date

  // Check if the date is valid
  if (isNaN(parsedDate.getTime())) {
    console.warn('Invalid date provided to formatRelativeTime:', date)
    return 'Invalid date'
  }

  const now = new Date()
  const diff = now.getTime() - parsedDate.getTime()

  // Handle future dates (shouldn't happen but just in case)
  if (diff < 0) {
    return 'in the future'
  }

  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`

  // For dates older than a week, show the actual date using browser locale
  return parsedDate.toLocaleDateString(locale || navigator.language, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

export function formatAbsoluteTime(date: Date | string, locale?: string): string {
  // Handle both Date objects and date strings
  const parsedDate = typeof date === 'string' ? new Date(date) : date

  // Check if the date is valid
  if (isNaN(parsedDate.getTime())) {
    console.warn('Invalid date provided to formatAbsoluteTime:', date)
    return 'Invalid date'
  }

  // Format using browser locale with full date and time
  return parsedDate.toLocaleString(locale || navigator.language, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  })
}
