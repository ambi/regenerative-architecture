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
          'relative flex size-11 items-center justify-center rounded-lg border shadow-sm',
          inverse
            ? 'border-white/15 bg-white/10 text-white backdrop-blur-sm'
            : 'border-slate-200 bg-white text-blue-700 shadow-blue-950/5',
        )}
      >
        <IconFingerprint size={25} stroke={1.7} aria-hidden="true" />
        <span
          className={cn(
            'absolute -bottom-0.5 -right-0.5 size-3 rounded-full border-2 bg-teal-400',
            inverse ? 'border-[#0a1020]' : 'border-white',
          )}
        />
      </div>
      <div className="flex flex-col">
        <span className="text-[1.05rem] font-semibold leading-tight tracking-normal">
          RA Identity
        </span>
        {!compact ? (
          <span className="mt-0.5 text-[0.68rem] font-semibold uppercase tracking-normal opacity-60">
            Identity & Access
          </span>
        ) : null}
      </div>
    </div>
  )
}
