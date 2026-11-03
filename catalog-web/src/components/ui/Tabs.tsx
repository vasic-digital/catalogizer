import * as React from "react"
import { cn } from "@/lib/utils"

interface Tab {
  id: string
  label: string
}

interface TabsProps {
  tabs?: Tab[]
  activeTab?: string
  onChangeTab?: (tab: string) => void
  children?: React.ReactNode
  defaultValue?: string
  value?: string
  onValueChange?: (value: string) => void
  className?: string
}

interface TabsListProps {
  children: React.ReactNode
  className?: string
}

interface TabsTriggerProps {
  value: string
  children: React.ReactNode
  className?: string
  isActive?: boolean
  onClick?: () => void
}

interface TabsContentProps {
  value: string
  children: React.ReactNode
  className?: string
  isActive?: boolean
}

const TabsContext = React.createContext<{
  value?: string
  onValueChange?: (value: string) => void
}>({})

const Tabs: React.FC<TabsProps> = ({
  tabs,
  activeTab,
  onChangeTab,
  children,
  defaultValue,
  value: controlledValue,
  onValueChange
}) => {
  // Always call hooks at the top level
  const [internalValue, setInternalValue] = React.useState(defaultValue || '')
  const isControlled = controlledValue !== undefined
  const currentValue = isControlled ? controlledValue : internalValue

  const handleValueChange = React.useCallback((newValue: string) => {
    if (!isControlled) {
      setInternalValue(newValue)
    }
    onValueChange?.(newValue)
  }, [isControlled, onValueChange])

  // Support simple tabs mode
  if (tabs && activeTab && onChangeTab) {
    return (
      <div className="inline-flex min-h-[2.5rem] max-w-full overflow-x-auto items-center justify-start sm:justify-center rounded-md bg-gray-100 dark:bg-gray-800 p-1 text-gray-600 dark:text-gray-400 mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => onChangeTab(tab.id)}
            className={`inline-flex flex-shrink-0 items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 ${
              activeTab === tab.id
                ? "bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm"
                : "hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>
    );
  }

  // Support complex tabs with children
  return (
    <TabsContext.Provider value={{ value: currentValue, onValueChange: handleValueChange }}>
      {children}
    </TabsContext.Provider>
  )
}

const TabsList: React.FC<TabsListProps> = ({ children, className }) => (
  <div className={cn(
    "inline-flex min-h-[2.5rem] max-w-full overflow-x-auto items-center justify-start sm:justify-center rounded-md bg-gray-100 dark:bg-gray-800 p-1 text-gray-600 dark:text-gray-400",
    className
  )}>
    {children}
  </div>
)

const TabsTrigger: React.FC<TabsTriggerProps> = ({ 
  value, 
  children, 
  className, 
  onClick 
}) => {
  const context = React.useContext(TabsContext)
  const isActive = context.value === value

  const handleClick = () => {
    onClick?.()
    context.onValueChange?.(value)
  }

  return (
    <button
      className={cn(
        "inline-flex flex-shrink-0 items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-all",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2",
        "disabled:pointer-events-none disabled:opacity-50",
        isActive 
          ? "bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm" 
          : "hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white",
        className
      )}
      onClick={handleClick}
    >
      {children}
    </button>
  )
}

const TabsContent: React.FC<TabsContentProps> = ({ 
  value, 
  children, 
  className 
}) => {
  const context = React.useContext(TabsContext)
  const isActive = context.value === value

  if (!isActive) return null

  return (
    <div className={cn(
      "mt-2",
      className
    )}>
      {children}
    </div>
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent }