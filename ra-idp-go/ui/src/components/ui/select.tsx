import { IconCheck, IconChevronDown } from '@tabler/icons-react'
import { cn } from '../../lib/utils'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from './dropdown-menu'

export type SelectOption = { value: string; label: string }

type SelectProps = {
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  className?: string
  id?: string
  disabled?: boolean
  'aria-label'?: string
}

// Radix DropdownMenu ベースのスタイル付き Select。native select の見た目の粗さを避け、
// 他の管理画面 (dropdown-menu 採用箇所) と統一する。
export function Select({
  value,
  onValueChange,
  options,
  placeholder,
  className,
  id,
  disabled,
  'aria-label': ariaLabel,
}: SelectProps) {
  const current = options.find((option) => option.value === value)
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        id={id}
        type="button"
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn(
          'inline-flex h-10 items-center justify-between gap-2 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 transition hover:border-slate-400 focus:border-blue-600 focus:outline-none focus:ring-3 focus:ring-blue-600/10 disabled:cursor-not-allowed disabled:opacity-60',
          className,
        )}
      >
        <span className={cn('truncate', current ? '' : 'text-slate-400')}>
          {current?.label ?? placeholder ?? '選択'}
        </span>
        <IconChevronDown size={16} className="shrink-0 text-slate-400" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[10rem]">
        {options.map((option) => (
          <DropdownMenuItem key={option.value} onSelect={() => onValueChange(option.value)}>
            <span className="flex-1">{option.label}</span>
            {option.value === value ? (
              <IconCheck size={16} className="text-blue-600" aria-hidden="true" />
            ) : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
