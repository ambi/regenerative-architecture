import type { InputHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function Input({ className, type, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      type={type}
      className={cn(
        'h-12 w-full rounded-lg border border-slate-300 bg-white/92 px-3.5 py-2 text-[0.925rem] text-slate-950 shadow-xs outline-none transition-[border-color,box-shadow,background-color]',
        'placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-600 focus:bg-white focus:ring-3 focus:ring-blue-600/10',
        'disabled:cursor-not-allowed disabled:bg-slate-100 disabled:opacity-60',
        className,
      )}
      {...props}
    />
  )
}
