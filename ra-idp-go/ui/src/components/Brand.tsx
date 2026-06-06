import { IconFingerprint } from '@tabler/icons-react'
import { cn } from '../lib/utils'

type BrandProps = {
  compact?: boolean
  inverse?: boolean
}

export function Brand({ compact = false, inverse = false }: BrandProps) {
  return (
    <div
      className={cn(
        'flex flex-nowrap items-center gap-3.5',
        inverse ? 'text-white' : 'text-slate-950',
      )}
    >
      <div
        className={cn(
          'relative flex size-11 items-center justify-center rounded-xl border shadow-sm',
          inverse
            ? 'border-white/15 bg-white/10 text-white backdrop-blur-sm'
            : 'border-blue-100 bg-blue-50 text-blue-700',
        )}
      >
        <IconFingerprint size={25} stroke={1.7} aria-hidden="true" />
        <span
          className={cn(
            'absolute -bottom-0.5 -right-0.5 size-3 rounded-full border-2 bg-emerald-400',
            inverse ? 'border-[#0d1b36]' : 'border-white',
          )}
        />
      </div>
      <div className="flex flex-col">
        <span className="text-[1.05rem] font-semibold leading-tight tracking-[-0.01em]">
          RA Identity
        </span>
        {!compact ? (
          <span className="mt-0.5 text-[0.68rem] font-semibold uppercase tracking-[0.12em] opacity-60">
            Identity & Access
          </span>
        ) : null}
      </div>
    </div>
  )
}
