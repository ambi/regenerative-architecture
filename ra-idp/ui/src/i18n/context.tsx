import { createContext, useContext, useMemo, type ReactNode } from 'react'
import { pickCatalog, type Locale, type Messages } from './messages'
import { readMeta } from '@/lib/page-context'

interface I18nValue {
  locale: Locale
  m: Messages
}

const I18nContext = createContext<I18nValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const value = useMemo<I18nValue>(() => {
    const fromMeta = readMeta('ra-idp:locale')
    const fromHtml = typeof document !== 'undefined' ? document.documentElement.lang : null
    const locale: Locale = (fromMeta ?? fromHtml) === 'en' ? 'en' : 'ja'
    return { locale, m: pickCatalog(locale) }
  }, [])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useMessages(): Messages {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useMessages must be used within I18nProvider')
  return ctx.m
}

export function useLocale(): Locale {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useLocale must be used within I18nProvider')
  return ctx.locale
}
