import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { entityApi } from '@/lib/mediaApi'
import { recommendationsApi } from '@/lib/recommendationsApi'
import { downloadApi } from '@/lib/downloadApi'
import { analyticsApi } from '@/lib/analyticsApi'
import { Button } from '@/components/ui/Button'
import { ArrowLeft, ChevronRight, Globe, Star, Download, Sparkles } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { EntityHero, ChildrenList, FilesList, DuplicatesList } from '@/components/entity/EntityDetailView'
import toast from 'react-hot-toast'
import { useAuth } from '@/contexts/AuthContext'

export function EntityDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const entityId = parseInt(id || '0', 10)

  const {
    data: entity,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['entity', entityId],
    queryFn: () => entityApi.getEntity(entityId),
    enabled: entityId > 0,
    // A 4xx (not-found, forbidden, bad id) is a definitive answer: retrying
    // it only keeps the spinner up. Transient failures keep the default retry.
    retry: (failureCount, err) => {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status !== undefined && status >= 400 && status < 500) return false
      return failureCount < 3
    },
  })
  const errorStatus = (error as { response?: { status?: number } } | null)?.response?.status

  const { data: childrenData } = useQuery({
    queryKey: ['entityChildren', entityId],
    queryFn: () => entityApi.getEntityChildren(entityId),
    enabled: entityId > 0 && (entity?.children_count || 0) > 0,
  })

  const { data: filesData } = useQuery({
    queryKey: ['entityFiles', entityId],
    queryFn: () => entityApi.getEntityFiles(entityId),
    enabled: entityId > 0,
  })

  const { data: duplicatesData } = useQuery({
    queryKey: ['entityDuplicates', entityId],
    queryFn: () => entityApi.getEntityDuplicates(entityId),
    enabled: entityId > 0,
  })

  const { data: similarItems } = useQuery({
    queryKey: ['recommendations-similar', entityId],
    queryFn: () => recommendationsApi.getSimilar(entityId),
    enabled: entityId > 0,
  })

  // Track page view via analytics
  useQuery({
    queryKey: ['analytics-track-view', entityId],
    queryFn: () => analyticsApi.trackAccess({ media_id: entityId, action: 'view' }),
    enabled: entityId > 0,
    staleTime: Infinity,
    retry: false,
  })

  const downloadMutation = useMutation({
    mutationFn: () => downloadApi.downloadEntity(entityId, entity?.title),
    onSuccess: () => toast.success('Download started'),
    onError: () => toast.error('Download failed'),
  })

  const refreshMutation = useMutation({
    mutationFn: () => entityApi.refreshEntityMetadata(entityId),
    onSuccess: () => {
      toast.success('Metadata refresh queued')
      queryClient.invalidateQueries({ queryKey: ['entity', entityId] })
    },
    onError: () => toast.error('Failed to refresh metadata'),
  })

  const favoriteMutation = useMutation({
    mutationFn: (favorite: boolean) =>
      entityApi.updateUserMetadata(entityId, { favorite }),
    onSuccess: () => {
      toast.success('Updated')
      queryClient.invalidateQueries({ queryKey: ['entity', entityId] })
    },
  })

  // v4: a disabled query (invalid id) reports isLoading, so gate on a valid id.
  if (isLoading && entityId > 0) {
    return (
      <div className="flex items-center justify-center min-h-[400px]" data-testid="entity-loading">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    )
  }

  if (isError && !(errorStatus && errorStatus >= 400 && errorStatus < 500)) {
    return (
      <div role="alert" className="text-center py-12">
        <p className="text-gray-700 dark:text-gray-300">Failed to load this item. Please try again.</p>
        <Button variant="outline" className="mt-4" onClick={() => refetch()}>
          Try again
        </Button>
      </div>
    )
  }

  if (!entity) {
    const denied = errorStatus === 401 || errorStatus === 403
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 dark:text-gray-400">
          {denied ? 'You do not have access to this item' : 'Entity not found'}
        </p>
        <Button variant="outline" className="mt-4" onClick={() => navigate('/browse')}>
          Back to Browse
        </Button>
      </div>
    )
  }

  const children = childrenData?.items || []
  const files = filesData?.files || []
  const duplicates = duplicatesData?.duplicates || []

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
          <ArrowLeft className="h-5 w-5" />
        </Button>
        <span className="text-sm text-gray-500 dark:text-gray-400 capitalize">
          {entity.media_type.replace(/_/g, ' ')}
        </span>
        <ChevronRight className="h-4 w-4 text-gray-400" />
        <span className="text-sm font-medium text-gray-900 dark:text-white">
          {entity.title}
        </span>
      </div>

      <EntityHero
        entity={entity}
        files={files}
        duplicateCount={duplicates.length}
        onFavorite={() => favoriteMutation.mutate(true)}
        onRefresh={() => refreshMutation.mutate()}
        refreshPending={refreshMutation.isPending}
        canRefresh={isAdmin}
      />

      <ChildrenList
        items={children}
        mediaType={entity.media_type}
        onChildClick={(childId) => navigate(`/entity/${childId}`)}
      />

      <FilesList files={files} />

      {entity.external_metadata && entity.external_metadata.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="h-5 w-5" />
              External Metadata
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {entity.external_metadata.map((meta) => (
                <div key={meta.id} className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
                  <div className="flex-1">
                    <span className="font-medium text-gray-900 dark:text-white capitalize">{meta.provider}</span>
                    <span className="text-sm text-gray-500 dark:text-gray-400 ml-2">ID: {meta.external_id}</span>
                  </div>
                  {meta.rating != null && (
                    <span className="flex items-center gap-1 text-sm">
                      <Star className="h-3 w-3 text-yellow-500" /> {meta.rating}
                    </span>
                  )}
                  {meta.review_url && (
                    <a href={meta.review_url} target="_blank" rel="noopener noreferrer" className="text-sm text-blue-600 hover:underline">
                      View
                    </a>
                  )}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <DuplicatesList
        duplicates={duplicates}
        onDuplicateClick={(dupId) => navigate(`/entity/${dupId}`)}
      />

      {/* Download Button */}
      <Card>
        <CardContent className="p-4 flex items-center justify-between">
          <div>
            <span className="font-medium text-gray-900 dark:text-white">Download</span>
            <span className="text-sm text-gray-500 dark:text-gray-400 ml-2">
              {files.length} file{files.length !== 1 ? 's' : ''} available
            </span>
          </div>
          <Button
            onClick={() => downloadMutation.mutate()}
            loading={downloadMutation.isPending}
            disabled={files.length === 0}
          >
            <Download className="h-4 w-4 mr-2" />
            Download
          </Button>
        </CardContent>
      </Card>

      {/* Similar Items / Recommendations */}
      {similarItems && similarItems.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Sparkles className="h-5 w-5" />
              Similar Items
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {similarItems.slice(0, 6).map((item) => (
                <button
                  key={item.id}
                  onClick={() => navigate(`/entity/${item.id}`)}
                  className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors text-left"
                >
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-gray-900 dark:text-white truncate">
                      {item.title}
                    </div>
                    <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                      <span className="capitalize">{item.media_type.replace(/_/g, ' ')}</span>
                      {item.year && <span>({item.year})</span>}
                      {item.similarity_score != null && (
                        <span className="text-blue-600 dark:text-blue-400">
                          {Math.round(item.similarity_score * 100)}% match
                        </span>
                      )}
                    </div>
                    {item.reason && (
                      <div className="text-xs text-gray-600 dark:text-gray-400 mt-1 truncate">{item.reason}</div>
                    )}
                  </div>
                  {item.rating != null && (
                    <div className="flex items-center gap-1 text-sm text-yellow-600">
                      <Star className="h-3 w-3" />
                      {item.rating}
                    </div>
                  )}
                </button>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

export default EntityDetail
