import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import type { UserAttributeDef } from '../types'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// attributeLabel は属性を「日本語表示名 (key)」で表す。label が無ければ key のみ。
export function attributeLabel(def: Pick<UserAttributeDef, 'key' | 'label'>): string {
  return def.label ? `${def.label} (${def.key})` : def.key
}
