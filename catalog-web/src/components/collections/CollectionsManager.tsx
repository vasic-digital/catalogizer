import React, { useState } from 'react';
import { Plus, Search, Grid, List, PlayCircle, Edit2, Trash2 } from 'lucide-react';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/Card';
import { Badge } from '../ui/Badge';

interface Collection {
  id: string;
  name: string;
  description?: string;
  mediaCount: number;
  duration: number;
  thumbnail?: string;
  isSmart: boolean;
  criteria?: {
    genres?: string[];
    yearRange?: [number, number];
    ratingRange?: [number, number];
    tags?: string[];
  };
  createdAt: string;
  updatedAt: string;
}

interface CollectionsManagerProps {
  collections: Collection[];
  onCreateCollection?: (collection: Omit<Collection, 'id' | 'createdAt' | 'updatedAt'>) => void;
  onUpdateCollection?: (id: string, collection: Partial<Collection>) => void;
  onDeleteCollection?: (id: string) => void;
  onPlayCollection?: (id: string) => void;
}

/**
 * CollectionsManager provides CRUD controls for media collections with
 * search, grid/list view switching, and smart collection support.
 */
export const CollectionsManager: React.FC<CollectionsManagerProps> = ({
  collections,
  onCreateCollection,
  onUpdateCollection,
  onDeleteCollection,
  onPlayCollection
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingCollection, setEditingCollection] = useState<Collection | null>(null);

  const filteredCollections = collections.filter(collection =>
    collection.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    collection.description?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const formatDuration = (seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }
    return `${minutes}m`;
  };

  const handleCreateCollection = (collectionData: Omit<Collection, 'id' | 'createdAt' | 'updatedAt'>) => {
    onCreateCollection?.(collectionData);
    setShowCreateModal(false);
  };

  const handleUpdateCollection = (collectionData: Partial<Collection>) => {
    if (editingCollection) {
      onUpdateCollection?.(editingCollection.id, collectionData);
      setEditingCollection(null);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-4 h-4" />
            <Input
              placeholder="Search collections..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10 w-64"
            />
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant={viewMode === 'grid' ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setViewMode('grid')}
            >
              <Grid className="w-4 h-4" />
            </Button>
            <Button
              variant={viewMode === 'list' ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setViewMode('list')}
            >
              <List className="w-4 h-4" />
            </Button>
          </div>
        </div>
        <Button onClick={() => setShowCreateModal(true)}>
          <Plus className="w-4 h-4 mr-2" />
          New Collection
        </Button>
      </div>

      {/* Collections Display */}
      {viewMode === 'grid' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
          {filteredCollections.map(collection => (
            <Card key={collection.id} className="group hover:shadow-lg transition-shadow">
              <CardHeader className="p-0">
                {collection.thumbnail ? (
                  <img 
                    src={collection.thumbnail} 
                    alt={collection.name}
                    className="w-full h-48 object-cover rounded-t-lg"
                  />
                ) : (
                  <div className="w-full h-48 bg-gradient-to-br from-blue-400 to-purple-600 rounded-t-lg flex items-center justify-center">
                    <PlayCircle className="w-16 h-16 text-white/80" />
                  </div>
                )}
              </CardHeader>
              <CardContent className="p-4">
                <div className="space-y-2">
                  <div className="flex items-start justify-between">
                    <h3 className="font-medium text-gray-900 group-hover:text-blue-600 transition-colors">
                      {collection.name}
                    </h3>
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditingCollection(collection)}
                      >
                        <Edit2 className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => onDeleteCollection?.(collection.id)}
                        className="text-red-500 hover:text-red-600"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                  
                  {collection.description && (
                    <p className="text-sm text-gray-600 line-clamp-2">
                      {collection.description}
                    </p>
                  )}
                  
                  <div className="flex items-center gap-4 text-sm text-gray-500">
                    <span>{collection.mediaCount} items</span>
                    <span>{formatDuration(collection.duration)}</span>
                    {collection.isSmart && (
                      <Badge variant="secondary">Smart</Badge>
                    )}
                  </div>
                  
                  <div className="flex items-center justify-between pt-2">
                    <span className="text-xs text-gray-600 dark:text-gray-400">
                      Updated {new Date(collection.updatedAt).toLocaleDateString()}
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onPlayCollection?.(collection.id)}
                    >
                      <PlayCircle className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          {filteredCollections.map(collection => (
            <Card key={collection.id} className="group">
              <CardContent className="p-4">
                <div className="flex items-center gap-4">
                  {collection.thumbnail ? (
                    <img 
                      src={collection.thumbnail} 
                      alt={collection.name}
                      className="w-16 h-16 object-cover rounded-lg"
                    />
                  ) : (
                    <div className="w-16 h-16 bg-gradient-to-br from-blue-400 to-purple-600 rounded-lg flex items-center justify-center">
                      <PlayCircle className="w-8 h-8 text-white/80" />
                    </div>
                  )}
                  
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between">
                      <div className="min-w-0 flex-1">
                        <h3 className="font-medium text-gray-900 group-hover:text-blue-600 transition-colors truncate">
                          {collection.name}
                        </h3>
                        {collection.description && (
                          <p className="text-sm text-gray-600 line-clamp-1 mt-1">
                            {collection.description}
                          </p>
                        )}
                      </div>
                      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity ml-4">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setEditingCollection(collection)}
                        >
                          <Edit2 className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => onDeleteCollection?.(collection.id)}
                          className="text-red-500 hover:text-red-600"
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-4 mt-2">
                      <span className="text-sm text-gray-500">
                        {collection.mediaCount} items
                      </span>
                      <span className="text-sm text-gray-500">
                        {formatDuration(collection.duration)}
                      </span>
                      {collection.isSmart && (
                        <Badge variant="secondary">Smart Collection</Badge>
                      )}
                      <span className="text-sm text-gray-600 dark:text-gray-400">
                        Updated {new Date(collection.updatedAt).toLocaleDateString()}
                      </span>
                    </div>
                  </div>
                  
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onPlayCollection?.(collection.id)}
                  >
                    <PlayCircle className="w-5 h-5" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Empty State */}
      {filteredCollections.length === 0 && (
        <Card className="text-center py-12">
          <CardContent>
            <div className="space-y-4">
              <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mx-auto">
                <Plus className="w-8 h-8 text-gray-400" />
              </div>
              <div>
                <h3 className="text-lg font-medium text-gray-900 mb-2">
                  {searchQuery ? 'No collections found' : 'No collections yet'}
                </h3>
                <p className="text-gray-600 mb-4">
                  {searchQuery 
                    ? 'Try adjusting your search terms' 
                    : 'Create your first collection to organize your media'
                  }
                </p>
                {!searchQuery && (
                  <Button onClick={() => setShowCreateModal(true)}>
                    <Plus className="w-4 h-4 mr-2" />
                    Create Collection
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Create/Edit Modal (placeholder) */}
      {(showCreateModal || editingCollection) && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>
                {editingCollection ? 'Edit Collection' : 'Create New Collection'}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Input
                placeholder="Collection name"
                defaultValue={editingCollection?.name}
              />
              <textarea
                placeholder="Description (optional)"
                className="w-full p-3 border border-gray-300 rounded-lg resize-none"
                rows={3}
                defaultValue={editingCollection?.description}
              />
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowCreateModal(false);
                    setEditingCollection(null);
                  }}
                  className="flex-1"
                >
                  Cancel
                </Button>
                <Button
                  onClick={() => {
                    if (editingCollection) {
                      handleUpdateCollection({
                        name: 'Updated Collection Name',
                        description: 'Updated description'
                      });
                    } else {
                      handleCreateCollection({
                        name: 'New Collection',
                        description: 'Collection description',
                        mediaCount: 0,
                        duration: 0,
                        isSmart: false
                      });
                    }
                  }}
                  className="flex-1"
                >
                  {editingCollection ? 'Update' : 'Create'}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};