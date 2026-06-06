import type { HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function Alert({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      role="alert"
      className={cn('rounded-xl border border-red-200 bg-red-50/80 p-4 text-red-950', className)}
      {...props}
    />
  )
}
