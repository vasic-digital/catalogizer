import { useState, useRef } from 'react'
import { Heart, Settings, Download, Upload } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { FavoritesGrid } from '@/components/favorites/FavoritesGrid'
import { PageHeader } from '@/components/layout/PageHeader'
import { useFavorites } from '@/hooks/useFavorites'
import { cn } from '@/lib/utils'
import toast from 'react-hot-toast'

const FavoritesPage: React.FC = () => {
  const [showBulkActions, setShowBulkActions] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { favorites, stats, refetchStats, refetchFavorites, bulkAddToFavorites } = useFavorites()

  const handleExportFavorites = () => {
    if (!favorites || favorites.length === 0) {
      toast.error('No favorites to export')
      return
    }
    const exportData = favorites.map(fav => ({
      media_id: fav.media_id,
      title: fav.media_item?.title,
      media_type: fav.media_item?.media_type,
      year: fav.media_item?.year,
      added_at: fav.created_at,
    }))
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `favorites-${new Date().toISOString().split('T')[0]}.json`
    link.click()
    URL.revokeObjectURL(url)
    toast.success(`Exported ${exportData.length} favorites`)
  }

  const handleImportFavorites = () => {
    fileInputRef.current?.click()
  }

  const handleFileSelected = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (e) => {
      try {
        const data = JSON.parse(e.target?.result as string)
        const mediaIds = Array.isArray(data)
          ? data.map((item: { media_id?: number }) => item.media_id).filter(Boolean) as number[]
          : []
        if (mediaIds.length === 0) {
          toast.error('No valid media IDs found in file')
          return
        }
        bulkAddToFavorites(mediaIds)
        refetchFavorites()
        refetchStats()
      } catch (error) {
      console.error("Error:", error);
        toast.error('Invalid JSON file')
      }
    }
    reader.readAsText(file)
    event.target.value = ''
  }

  const handleBulkOperations = () => {
    setShowBulkActions(!showBulkActions)
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <input
        ref={fileInputRef}
        type="file"
        accept=".json"
        className="hidden"
        onChange={handleFileSelected}
      />
      <PageHeader
        title="My Favorites"
        subtitle="Manage your favorite media items"
        icon={<Heart className="w-6 h-6" />}
        actions={
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={handleBulkOperations}
              className={cn(
                'transition-colors',
                showBulkActions && 'bg-blue-50 border-blue-300 text-blue-700 dark:bg-blue-900/20 dark:border-blue-600 dark:text-blue-300'
              )}
            >
              <Settings className="w-4 h-4 mr-2" />
              Bulk Actions
            </Button>
            <Button variant="outline" onClick={handleImportFavorites}>
              <Upload className="w-4 h-4 mr-2" />
              Import
            </Button>
            <Button variant="outline" onClick={handleExportFavorites}>
              <Download className="w-4 h-4 mr-2" />
              Export
            </Button>
          </div>
        }
      />

      <div className="container mx-auto px-4 py-6">
        <Tabs defaultValue="favorites" className="space-y-6">
          <TabsList>
            <TabsTrigger value="favorites">Favorites</TabsTrigger>
            <TabsTrigger value="recent">Recently Added</TabsTrigger>
            <TabsTrigger value="stats">Statistics</TabsTrigger>
          </TabsList>

          <TabsContent value="favorites">
            <FavoritesGrid
              showFilters={true}
              showStats={true}
              selectable={showBulkActions}
            />
          </TabsContent>

          <TabsContent value="recent">
            <Card>
              <CardHeader>
                <CardTitle>Recently Added Favorites</CardTitle>
              </CardHeader>
              <CardContent>
                <FavoritesGrid
                  showFilters={false}
                  showStats={false}
                  selectable={showBulkActions}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="stats">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <Card>
                <CardHeader>
                  <CardTitle>Favorite Statistics</CardTitle>
                </CardHeader>
                <CardContent>
                  {stats ? (
                    <div className="space-y-4">
                      <div className="flex justify-between items-center">
                        <span className="text-gray-600 dark:text-gray-400">Total Favorites</span>
                        <span className="text-2xl font-bold">{stats.total_count}</span>
                      </div>
                      
                      <div className="border-t pt-4">
                        <h4 className="font-medium mb-3">By Media Type</h4>
                        <div className="space-y-2">
                          {Object.entries(stats.media_type_breakdown).map(([type, count]) => (
                            <div key={type} className="flex justify-between items-center">
                              <span className="capitalize text-gray-600 dark:text-gray-400">
                                {type.replace('_', ' ')}
                              </span>
                              <span className="font-medium">{count}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="animate-pulse">
                      <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded mb-3"></div>
                      <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded mb-4"></div>
                      <div className="space-y-2">
                        <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded"></div>
                        <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-5/6"></div>
                        <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-4/6"></div>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Favorite Insights</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-4">
                    <div className="p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                      <h4 className="font-medium text-blue-900 dark:text-blue-100 mb-2">
                        Most Common Type
                      </h4>
                      {stats && (
                        <p className="text-blue-700 dark:text-blue-300">
                          {Object.entries(stats.media_type_breakdown).reduce((a, b) => 
                            a[1] > b[1] ? a : b
                          )[0].replace('_', ' ')}
                        </p>
                      )}
                    </div>

                    <div className="p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
                      <h4 className="font-medium text-green-900 dark:text-green-100 mb-2">
                        Recent Activity
                      </h4>
                      <p className="text-green-700 dark:text-green-300">
                        {stats?.recent_additions?.length || 0} items added in the last week
                      </p>
                    </div>

                    <div className="p-4 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
                      <h4 className="font-medium text-purple-900 dark:text-purple-100 mb-2">
                        Storage Impact
                      </h4>
                      <p className="text-purple-700 dark:text-purple-300">
                        Favorites help you quickly access your most-loved content
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}

export default FavoritesPage