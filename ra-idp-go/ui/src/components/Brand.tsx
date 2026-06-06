import { IconFingerprint } from '@tabler/icons-react'

export function Brand() {
  return (
    <div className="flex flex-nowrap items-center gap-3">
      <div className="flex size-[42px] items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-cyan-500 text-white">
        <IconFingerprint size={25} stroke={1.8} />
      </div>
      <div className="flex flex-col">
        <span className="text-lg font-bold leading-tight">RA Identity</span>
        <span className="text-xs font-semibold uppercase tracking-[0.08em] opacity-60">
          Secure access
        </span>
      </div>
    </div>
  )
}
