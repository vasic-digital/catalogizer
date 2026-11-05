import React, { useEffect, useRef, useState, useCallback } from 'react'
import { motion } from 'framer-motion'

interface PerformanceOptimizerProps {
  children: React.ReactNode
  itemCount: number
  threshold?: number
  loadingStrategy?: 'lazy' | 'virtual' | 'pagination'
  itemHeight?: number
  containerHeight?: number
  onVisibleItemsChange?: (visibleIndices: [number, number]) => void
}

export const PerformanceOptimizer: React.FC<PerformanceOptimizerProps> = ({
  children,
  itemCount,
  threshold = 100,
  loadingStrategy = 'lazy',
  itemHeight = 60,
  containerHeight = 400,
  onVisibleItemsChange
}) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const [visibleRange, setVisibleRange] = useState<[number, number]>([0, Math.min(20, itemCount)])
  const [_scrollTop, setScrollTop] = useState(0)
  const [_isIntersecting, setIsIntersecting] = useState<Map<number, boolean>>(new Map())
  const [loadedItems, setLoadedItems] = useState<Set<number>>(new Set())
  const loadedItemsRef = useRef(loadedItems)
  loadedItemsRef.current = loadedItems
  const observerRef = useRef<IntersectionObserver | null>(null)

  // Calculate visible items for virtualization
  const calculateVisibleRange = useCallback((scrollTop: number) => {
    const startIndex = Math.floor(scrollTop / itemHeight)
    const endIndex = Math.min(
      startIndex + Math.ceil(containerHeight / itemHeight) + 5, // Add buffer
      itemCount - 1
    )
    return [Math.max(0, startIndex), endIndex] as [number, number]
  }, [itemHeight, containerHeight, itemCount])

  // Handle scroll events
  const handleScroll = useCallback(() => {
    if (!containerRef.current) return
    
    const newScrollTop = containerRef.current.scrollTop
    setScrollTop(newScrollTop)
    
    if (loadingStrategy === 'virtual') {
      const newRange = calculateVisibleRange(newScrollTop)
      setVisibleRange(newRange)
      onVisibleItemsChange?.(newRange)
    }
  }, [loadingStrategy, calculateVisibleRange, onVisibleItemsChange])

  // Set up intersection observer for lazy loading
  useEffect(() => {
    if (loadingStrategy !== 'lazy') return

    const MAX_INTERSECTING = 1000
    const MAX_LOADED = 1000

    observerRef.current = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const index = parseInt(entry.target.getAttribute('data-index') || '0')
          setIsIntersecting(prev => {
            const next = new Map(prev)
            next.set(index, entry.isIntersecting)
            if (next.size > MAX_INTERSECTING) {
              const entries = Array.from(next.entries())
              return new Map(entries.slice(entries.length - MAX_INTERSECTING + 100))
            }
            return next
          })

          if (entry.isIntersecting && !loadedItemsRef.current.has(index)) {
            setLoadedItems(prev => {
              const next = new Set(prev)
              next.add(index)
              if (next.size > MAX_LOADED) {
                const values = Array.from(next)
                return new Set(values.slice(values.length - MAX_LOADED + 100))
              }
              return next
            })
          }
        })
      },
      {
        root: containerRef.current,
        rootMargin: '50px',
        threshold: 0.1
      }
    )

    return () => {
      observerRef.current?.disconnect()
    }
  }, [loadingStrategy])

  // Debounced scroll handler
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    let timeoutId: NodeJS.Timeout
    const debouncedScroll = () => {
      clearTimeout(timeoutId)
      timeoutId = setTimeout(handleScroll, 16) // ~60fps
    }

    container.addEventListener('scroll', debouncedScroll)
    return () => {
      container.removeEventListener('scroll', debouncedScroll)
      clearTimeout(timeoutId)
    }
  }, [handleScroll])

  // Render virtualized items
  const renderVirtualizedItems = () => {
    if (!Array.isArray(children)) return children

    const [startIndex, endIndex] = visibleRange
    const items = []
    const totalHeight = itemCount * itemHeight

    for (let i = startIndex; i <= endIndex; i++) {
      const child = children[i] as React.ReactElement
      if (!child) continue

      const top = i * itemHeight
      items.push(
        <motion.div
          key={i}
          style={{
            position: 'absolute',
            top: `${top}px`,
            left: 0,
            right: 0,
            height: `${itemHeight}px`
          }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2, delay: (i - startIndex) * 0.02 }}
        >
          {React.cloneElement(child, { index: i })}
        </motion.div>
      )
    }

    return (
      <div style={{ height: `${totalHeight}px`, position: 'relative' }}>
        {items}
      </div>
    )
  }

  // Render lazy loaded items
  const renderLazyItems = () => {
    if (!Array.isArray(children)) return children

    return children.map((child, index) => {
      const isLoaded = loadedItems.has(index) || index < 10 // Load first 10 items immediately
      
      return (
        <motion.div
          key={index}
          data-index={index}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: isLoaded ? 1 : 0, y: isLoaded ? 0 : 20 }}
          transition={{ duration: 0.3, delay: isLoaded ? 0 : index * 0.05 }}
          style={{
            minHeight: isLoaded ? 'auto' : `${itemHeight}px`
          }}
        >
          {isLoaded ? child : (
            <div className="animate-pulse bg-gray-100 dark:bg-gray-800 rounded" 
                 style={{ height: `${itemHeight}px` }} />
          )}
        </motion.div>
      )
    })
  }

  // Render paginated items
  const renderPaginatedItems = () => {
    if (!Array.isArray(children)) return children

    const [startIndex, endIndex] = visibleRange
    return children.slice(startIndex, endIndex + 1).map((child, index) => (
      <motion.div
        key={startIndex + index}
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.2, delay: index * 0.05 }}
      >
        {child}
      </motion.div>
    ))
  }

  // Determine rendering strategy
  const shouldOptimize = itemCount > threshold
  const renderItems = () => {
    if (!shouldOptimize) return children

    switch (loadingStrategy) {
      case 'virtual':
        return renderVirtualizedItems()
      case 'lazy':
        return renderLazyItems()
      case 'pagination':
        return renderPaginatedItems()
      default:
        return children
    }
  }

  if (!shouldOptimize) {
    return <>{children}</>
  }

  return (
    <div
      ref={containerRef}
      className="overflow-auto"
      style={{ height: `${containerHeight}px` }}
      onScroll={handleScroll}
    >
      {renderItems()}
      
      {/* Loading indicator for pagination */}
      {loadingStrategy === 'pagination' && visibleRange[1] < itemCount - 1 && (
        <div className="flex justify-center py-4">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full"></div>
        </div>
      )}
    </div>
  )
}

// Performance monitoring hook
export const usePerformanceMonitor = (_componentName: string) => {
  const renderCount = useRef(0)
  const startTime = useRef(Date.now())
  const lastRenderTime = useRef(Date.now())

  useEffect(() => {
    renderCount.current++
    const now = Date.now()

    // Performance metrics available via getMetrics()

    lastRenderTime.current = now

    // Track frequent rendering (less than 60fps threshold)
  })

  const getMetrics = () => ({
    renderCount: renderCount.current,
    totalTime: Date.now() - startTime.current,
    averageRenderTime: (Date.now() - startTime.current) / renderCount.current
  })

  return { getMetrics }
}

// Memory optimization hook
export const useMemoryOptimization = () => {
  const [cache, setCache] = useState<Map<string, unknown>>(new Map())
  const maxCacheSize = 100

  const addToCache = useCallback((key: string, data: unknown) => {
    setCache(prev => {
      const newCache = new Map(prev)
      
      // Remove oldest items if cache is full
      if (newCache.size >= maxCacheSize) {
        const firstKey = newCache.keys().next().value
        newCache.delete(firstKey)
      }
      
      newCache.set(key, data)
      return newCache
    })
  }, [])

  const getFromCache = useCallback((key: string) => {
    return cache.get(key)
  }, [cache])

  const clearCache = useCallback(() => {
    setCache(new Map())
  }, [])

  useEffect(() => {
    // Cleanup cache on unmount
    return () => {
      setCache(new Map())
    }
  }, [])

  return { addToCache, getFromCache, clearCache, cacheSize: cache.size }
}

// Debounced search hook
export const useDebouncedSearch = (searchTerm: string, delay = 300) => {
  const [debouncedTerm, setDebouncedTerm] = useState(searchTerm)

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedTerm(searchTerm)
    }, delay)

    return () => {
      clearTimeout(handler)
    }
  }, [searchTerm, delay])

  return debouncedTerm
}

// Infinite scroll hook
export const useInfiniteScroll = (
  hasMore: boolean,
  isLoading: boolean,
  onLoadMore: () => void
) => {
  const observerRef = useRef<IntersectionObserver | null>(null)
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const onLoadMoreRef = useRef(onLoadMore)
  onLoadMoreRef.current = onLoadMore

  useEffect(() => {
    if (!hasMore || isLoading) return

    observerRef.current = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          onLoadMoreRef.current()
        }
      },
      {
        threshold: 0.1,
        rootMargin: '100px'
      }
    )

    if (loadMoreRef.current) {
      observerRef.current.observe(loadMoreRef.current)
    }

    return () => {
      observerRef.current?.disconnect()
    }
  }, [hasMore, isLoading])

  return loadMoreRef
}

// Dev-mode performance overlay — shows live FPS, memory, and DOM stats
export const PerformanceOverlay: React.FC = () => {
  const [visible, setVisible] = useState(true)
  const [metrics, setMetrics] = useState({
    fps: 0,
    domNodes: 0,
    heapUsedMB: 0,
    heapTotalMB: 0,
  })
  const frameCountRef = useRef(0)
  const lastTimeRef = useRef(performance.now())
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const measure = () => {
      frameCountRef.current++
      const now = performance.now()
      const elapsed = now - lastTimeRef.current

      if (elapsed >= 1000) {
        const fps = Math.round((frameCountRef.current / elapsed) * 1000)
        const domNodes = document.querySelectorAll('*').length

        let heapUsedMB = 0
        let heapTotalMB = 0
        const perf = performance as Performance & { memory?: { usedJSHeapSize: number; totalJSHeapSize: number } }
        if (perf.memory) {
          heapUsedMB = Math.round(perf.memory.usedJSHeapSize / (1024 * 1024))
          heapTotalMB = Math.round(perf.memory.totalJSHeapSize / (1024 * 1024))
        }

        setMetrics({ fps, domNodes, heapUsedMB, heapTotalMB })
        frameCountRef.current = 0
        lastTimeRef.current = now
      }

      rafRef.current = requestAnimationFrame(measure)
    }

    rafRef.current = requestAnimationFrame(measure)
    return () => cancelAnimationFrame(rafRef.current)
  }, [])

  if (!visible) {
    return (
      <button
        onClick={() => setVisible(true)}
        className="fixed bottom-2 right-2 z-[9999] bg-gray-900 text-green-400 text-xs px-2 py-1 rounded opacity-60 hover:opacity-100 font-mono"
      >
        perf
      </button>
    )
  }

  return (
    <div className="fixed bottom-2 right-2 z-[9999] bg-gray-900/90 text-green-400 text-xs p-3 rounded-lg shadow-lg font-mono min-w-[180px]">
      <div className="flex items-center justify-between mb-1">
        <span className="text-gray-400 font-bold">Performance</span>
        <button
          onClick={() => setVisible(false)}
          className="text-gray-400 hover:text-gray-200 ml-2"
        >
          x
        </button>
      </div>
      <div className={`${metrics.fps < 30 ? 'text-red-400' : metrics.fps < 50 ? 'text-yellow-400' : 'text-green-400'}`}>
        FPS: {metrics.fps}
      </div>
      <div>DOM: {metrics.domNodes} nodes</div>
      {metrics.heapTotalMB > 0 && (
        <div>Heap: {metrics.heapUsedMB}/{metrics.heapTotalMB} MB</div>
      )}
    </div>
  )
}