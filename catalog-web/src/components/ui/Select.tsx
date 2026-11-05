import React from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'

interface Option {
  value: string
  label: string
}

interface SelectProps extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'onChange'> {
  value?: string
  onChange?: (value: string) => void
  onValueChange?: (value: string) => void
  children?: React.ReactNode
  options?: Option[]
  className?: string
}

function nodeText(node: React.ReactNode): string {
  if (node == null || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(nodeText).join('')
  if (React.isValidElement(node)) return nodeText((node.props as { children?: React.ReactNode }).children)
  return ''
}

function selectedOptionLabel(
  value: string | undefined,
  options: Option[] | undefined,
  children: React.ReactNode
): string | undefined {
  if (options && options.length > 0) {
    return (options.find((o) => o.value === value) ?? options[0]).label
  }
  const opts: React.ReactElement<{ value?: string; children?: React.ReactNode }>[] = []
  React.Children.forEach(children, (c) => {
    if (React.isValidElement(c)) opts.push(c as React.ReactElement<{ value?: string; children?: React.ReactNode }>)
  })
  const pick = opts.find((o) => o.props.value === value) ?? opts[0]
  const text = pick ? nodeText(pick.props.children).trim() : ''
  return text || undefined
}

export const Select: React.FC<SelectProps> = ({
  value,
  onChange,
  onValueChange,
  children,
  options,
  className,
  ...props
}) => {
  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newValue = e.target.value;
    if (onChange) {
      onChange(newValue);
    }
    if (onValueChange) {
      onValueChange(newValue);
    }
  }

  // Accessible-name fallback (WCAG 4.1.2 / axe select-name): a select whose call site gave no
  // aria-label, aria-labelledby, title or id (an id may be associated with a <label>) is named from
  // its currently selected option, so no page ends up with an unnamed combobox.
  const hasOwnName = Boolean(
    props['aria-label'] || props['aria-labelledby'] || props.title || props.id
  )
  const fallbackName = hasOwnName ? undefined : selectedOptionLabel(value, options, children)

  // Generate options if provided
  const optionsChildren = options?.map((option) => (
    <option key={option.value} value={option.value}>
      {option.label}
    </option>
  ));

  return (
    <div className="relative">
      <select
        value={value}
        onChange={handleChange}
        aria-label={fallbackName}
        className={cn(
          'w-full px-3 py-2 bg-white border border-gray-300 rounded-lg shadow-sm',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:border-transparent dark:focus-visible:ring-offset-gray-900',
          'pr-8 appearance-none cursor-pointer',
          'text-gray-900',
          'dark:bg-gray-800 dark:border-gray-600 dark:text-white',
          className
        )}
        {...props}
      >
        {options ? optionsChildren : children}
      </select>
      <div className="absolute inset-y-0 right-0 flex items-center pr-2 pointer-events-none">
        <ChevronDown className="h-4 w-4 text-gray-400" />
      </div>
    </div>
  )
}

export { Select as SelectPrimitive }