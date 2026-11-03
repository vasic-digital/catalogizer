import { useSearchParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { entityApi } from '@/lib/mediaApi'
import { Button } from '@/components/ui/Button'
import { Search, ArrowLeft } from 'lucide-react'
import { TypeSelectorGrid } from '@/components/entity/TypeSelector'
import { EntityGrid } from '@/components/entity/EntityGrid'
import type { MediaEntity } from '@/types/media'

export function EntityBrowser() {
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const selectedType = searchParams.get('type') || ''
  const searchQuery = searchParams.get('q') || ''
  const page = parseInt(searchParams.get('page') || '1', 10)
  const limit = 24
  const offset = (page - 1) * limit

  const { data: typesData } = useQuery({
    queryKey: ['entityTypes'],
    queryFn: () => entityApi.getEntityTypes(),
  })

  const { data: statsData } = useQuery({
    queryKey: ['entityStats'],
    queryFn: () => entityApi.getEntityStats(),
  })

  const {
    data: entitiesData,
    isLoading: entitiesLoading,
  } = useQuery({
    queryKey: ['entities', selectedType, searchQuery, page],
    queryFn: () =>
      selectedType
        ? entityApi.browseByType(selectedType, { limit, offset })
        : entityApi.getEntities({ query: searchQuery || undefined, limit, offset }),
    enabled: !!selectedType || !!searchQuery,
  })

  const types = typesData?.types || []
  const entities = entitiesData?.items || []
  const total = entitiesData?.total || 0

  const handleTypeSelect = (type: string) => {
    setSearchParams({ type })
  }

  const handleSearch = (q: string) => {
    if (q) {
      setSearchParams({ q })
    } else {
      setSearchParams({})
    }
  }

  const handleBack = () => {
    setSearchParams({})
  }

  const handleEntityClick = (entity: MediaEntity) => {
    navigate(`/entity/${entity.id}`)
  }

  const handlePageChange = (newPage: number) => {
    const params: Record<string, string> = {}
    if (selectedType) params.type = selectedType
    if (searchQuery) params.q = searchQuery
    params.page = String(newPage)
    setSearchParams(params)
  }

  const showTypeSelector = !selectedType && !searchQuery

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {!showTypeSelector && (
            <Button variant="ghost" size="icon" onClick={handleBack}>
              <ArrowLeft className="h-5 w-5" />
            </Button>
          )}
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              {showTypeSelector
                ? 'Browse Media'
                : selectedType
                  ? selectedType.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()) + 's'
                  : `Search: "${searchQuery}"`}
            </h1>
            {statsData && showTypeSelector && (
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                {statsData.total_entities} total entities across all types
              </p>
            )}
          </div>
        </div>

        <div className="relative w-64">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search entities..."
            defaultValue={searchQuery}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                handleSearch((e.target as HTMLInputElement).value)
              }
            }}
            className="w-full pl-10 pr-4 py-2 bg-gray-100 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-gray-800 dark:border-gray-600 dark:text-white"
          />
        </div>
      </div>

      {showTypeSelector && <TypeSelectorGrid types={types} onSelect={handleTypeSelect} />}

      {!showTypeSelector && (
        <>
          {entitiesLoading ? (
            <div className="flex items-center justify-center min-h-[200px]">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
            </div>
          ) : (
            <EntityGrid
              entities={entities}
              total={total}
              limit={limit}
              offset={offset}
              page={page}
              onEntityClick={handleEntityClick}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </div>
  )
}

export default EntityBrowser
