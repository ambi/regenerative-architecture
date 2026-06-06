import type { InputHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function Input({ className, type, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      type={type}
      className={cn(
        'h-11 w-full rounded-md border bg-white px-3 py-2 text-sm outline-none transition',
        'placeholder:text-slate-400 focus:border-indigo-500 focus:ring-3 focus:ring-indigo-500/10',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
}
