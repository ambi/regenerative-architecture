import type { HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'rounded-lg border border-slate-200/80 bg-white/90 shadow-[0_18px_50px_-36px_rgb(15_23_42/42%)] backdrop-blur-sm',
        className,
      )}
      {...props}
    />
  )
}
