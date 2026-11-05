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