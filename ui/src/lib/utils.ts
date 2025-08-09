import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Robust date parsing supporting ISO strings, numeric timestamps (ms or s), and Date objects
function parseDateInput(date: Date | string | number | null | undefined): Date | null {
  if (date instanceof Date) return isNaN(date.getTime()) ? null : date
  if (typeof date === 'number') {
    const d = new Date(date)
    return isNaN(d.getTime()) ? null : d
  }
  if (typeof date === 'string') {
    const trimmed = date.trim()
    // numeric string => epoch ms or seconds
    if (/^\d+$/.test(trimmed)) {
      const n = parseInt(trimmed, 10)
      const ms = trimmed.length <= 10 ? n * 1000 : n
      const d = new Date(ms)
      return isNaN(d.getTime()) ? null : d
    }
    // normalize "YYYY-MM-DD HH:MM:SS" to ISO
    const normalized = /^\d{4}-\d{2}-\d{2} /.test(trimmed) ? trimmed.replace(' ', 'T') : trimmed
    const d = new Date(normalized)
    return isNaN(d.getTime()) ? null : d
  }
  return null
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  if (ms < 3600000) return `${(ms / 60000).toFixed(1)}m`
  return `${(ms / 3600000).toFixed(1)}h`
}

export function formatDurationHMS(ms: number): string {
  if (!Number.isFinite(ms)) return '-'
  const totalSeconds = Math.floor(ms / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const hh = String(hours).padStart(2, '0')
  const mm = String(minutes).padStart(2, '0')
  const ss = String(seconds).padStart(2, '0')
  return `${hh}:${mm}:${ss}`
}

export function isDisplayableDate(date: Date | string | number | null | undefined, minYear = 1970): boolean {
  const d = parseDateInput(date) || new Date('0001-01-01T00:00:00Z')
  if (!(d instanceof Date) || isNaN(d.getTime())) return false
  if (d.getFullYear() < minYear) return false
  return true
}

export function formatRelativeTime(date: Date | string | number, locale?: string): string {
  const d = parseDateInput(date)

  if (!d || !isDisplayableDate(d)) {
    return '-'
  }

  const now = new Date()
  const diff = now.getTime() - d.getTime()

  if (diff < 0) {
    return 'in the future'
  }

  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`

  // For dates older than a week, show the actual date+time using browser locale
  return d.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

export function formatAbsoluteTime(date: Date | string | number | null | undefined, locale?: string): string {
  const parsedDate = parseDateInput(date)

  if (!parsedDate || !isDisplayableDate(parsedDate)) {
    return '-'
  }

  return parsedDate.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  })
}


export function formatAbsoluteTimeCompact(date: Date | string | number, locale?: string): string {
  const d = parseDateInput(date)
  if (!d || !isDisplayableDate(d)) return '-'
  return d.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
    dateStyle: 'medium',
    timeStyle: 'short'
  } as Intl.DateTimeFormatOptions)
}

export function formatDatePairCompact(date: Date | string | number | null | undefined, locale?: string): string {
  if (date === null || date === undefined) return '-'
  const abs = formatAbsoluteTimeCompact(date, locale)
  const rel = formatRelativeTime(date, locale)
  if (abs === '-' && rel === '-') return '-'
  if (abs === '-' && rel !== '-') return rel
  if (rel === '-' && abs !== '-') return abs
  return `${abs} (${rel})`
}


export function formatRelativeTimeShort(date: Date | string | number, locale?: string): string {
  const d = parseDateInput(date)
  if (!d || !isDisplayableDate(d)) return '-'
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 0) return 'soon'
  if (diff < 60000) return 'now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d`
  // older than a week -> show short date
  return d.toLocaleDateString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
    month: 'short',
    day: 'numeric'
  })
}

export function formatAbsoluteTimeUltraCompact(date: Date | string | number, locale?: string): string {
  const d = parseDateInput(date)
  if (!d || !isDisplayableDate(d)) return '-'
  const now = new Date()
  const sameYear = d.getFullYear() === now.getFullYear()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) {
    return d.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
    })
  }
  if (sameYear) {
    return d.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
    })
  }
  return d.toLocaleString(locale || (typeof navigator !== 'undefined' ? navigator.language : undefined), {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
  })
}

export function formatDatePairUltraCompact(date: Date | string | number | null | undefined, locale?: string): string {
  if (date === null || date === undefined) return '-'
  const abs = formatAbsoluteTimeUltraCompact(date, locale)
  const rel = formatRelativeTimeShort(date, locale)
  if (abs === '-' && rel === '-') return '-'
  if (abs === '-' && rel !== '-') return rel
  if (rel === '-' && abs !== '-') return abs
  return `${abs} (${rel})`
}
