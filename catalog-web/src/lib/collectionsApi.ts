import { SmartCollection, CollectionRule, CreateCollectionRequest, UpdateCollectionRequest, CollectionAnalytics, ShareCollectionRequest, CollectionShareInfo, CollectionTemplate } from '../types/collections';
import { api } from './api';
import { asList } from './apiShapes';

export interface CollectionItem {
  id: string;
  title: string;
  artist?: string;
  album?: string;
  duration?: number;
  media_type: string;
  file_size: number;
  date_added: string;
}

export interface CollectionItemsResponse {
  items: CollectionItem[];
  total: number;
  page: number;
  limit: number;
}

function normalizeCollection(raw: Record<string, unknown>): SmartCollection {
  const str = (v: unknown, fallback = '') => (typeof v === 'string' ? v : fallback);
  const count = typeof raw.item_count === 'number' ? raw.item_count : typeof raw.total_items === 'number' ? raw.total_items : 0;
  const type = str(raw.collection_type);
  const primary = (['music', 'video', 'image', 'document'] as const).find((t) => t === type) ?? 'mixed';
  const updated = str(raw.updated_at, str(raw.created_at));
  return {
    ...raw,
    id: String(raw.id ?? ''),
    name: str(raw.name, 'Untitled'),
    description: typeof raw.description === 'string' ? raw.description : undefined,
    smart_rules: Array.isArray(raw.smart_rules) ? (raw.smart_rules as CollectionRule[]) : [],
    // A stored collection without rules is a manual collection.
    is_smart: raw.is_smart === true || (Array.isArray(raw.smart_rules) && raw.smart_rules.length > 0),
    item_count: count,
    last_updated: str(raw.last_updated, updated),
    created_at: str(raw.created_at),
    updated_at: updated,
    cover_image: typeof raw.cover_image === 'string' ? raw.cover_image : typeof raw.cover_url === 'string' ? raw.cover_url : undefined,
    thumbnail_url: typeof raw.thumbnail_url === 'string' ? raw.thumbnail_url : undefined,
    owner_id: raw.owner_id != null ? String(raw.owner_id) : '',
    is_public: raw.is_public === true,
    primary_media_type: (raw.primary_media_type as SmartCollection['primary_media_type']) ?? primary,
  };
}

class CollectionsApi {
  private baseUrl = '/collections';

  // Wire shape: `{items:[{id:int, name, collection_type, description, total_items,
  // cover_url, created_at, updated_at}], total, limit, offset}` — an envelope,
  // not an array, and not the SmartCollection shape the UI renders.
  async getCollections(): Promise<SmartCollection[]> {
    const response = await api.get(`${this.baseUrl}`);
    return asList<Record<string, unknown>>(response.data, 'items').map(normalizeCollection);
  }

  async getCollection(id: string): Promise<SmartCollection> {
    const response = await api.get(`${this.baseUrl}/${id}`);
    return response.data;
  }

  async createCollection(collection: CreateCollectionRequest): Promise<SmartCollection> {
    const response = await api.post(`${this.baseUrl}`, collection);
    return response.data;
  }

  async updateCollection(id: string, updates: UpdateCollectionRequest): Promise<SmartCollection> {
    const response = await api.put(`${this.baseUrl}/${id}`, updates);
    return response.data;
  }

  async deleteCollection(id: string): Promise<void> {
    await api.delete(`${this.baseUrl}/${id}`);
  }

  async getCollectionItems(id: string, page = 1, limit = 50): Promise<CollectionItemsResponse> {
    const response = await api.get(`${this.baseUrl}/${id}/items`, {
      params: { page, limit }
    });
    return response.data;
  }

  async refreshCollection(id: string): Promise<SmartCollection> {
    const response = await api.post(`${this.baseUrl}/${id}/refresh`);
    return response.data;
  }

  async getCollectionAnalytics(id: string): Promise<CollectionAnalytics> {
    const response = await api.get(`${this.baseUrl}/${id}/analytics`);
    return response.data;
  }

  async shareCollection(id: string, shareRequest: ShareCollectionRequest): Promise<CollectionShareInfo> {
    const response = await api.post(`${this.baseUrl}/${id}/share`, shareRequest);
    return response.data;
  }

  async getSharedCollection(shareId: string): Promise<SmartCollection> {
    const response = await api.get(`${this.baseUrl}/shared/${shareId}`);
    return response.data;
  }

  async exportCollection(id: string, format: 'json' | 'csv' | 'm3u' = 'json'): Promise<Blob> {
    const response = await api.get(`${this.baseUrl}/${id}/export`, {
      params: { format },
      responseType: 'blob'
    });
    return response.data;
  }

  async importCollection(file: File): Promise<SmartCollection> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await api.post(`${this.baseUrl}/import`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return response.data;
  }

  async duplicateCollection(id: string, newName?: string): Promise<SmartCollection> {
    const response = await api.post(`${this.baseUrl}/${id}/duplicate`, {
      name: newName
    });
    return response.data;
  }

  async addItemsToCollection(id: string, itemIds: string[]): Promise<void> {
    await api.post(`${this.baseUrl}/${id}/items`, { item_ids: itemIds });
  }

  async removeItemsFromCollection(id: string, itemIds: string[]): Promise<void> {
    await api.delete(`${this.baseUrl}/${id}/items`, {
      data: { item_ids: itemIds }
    });
  }

  async getCollectionSuggestions(): Promise<string[]> {
    const response = await api.get(`${this.baseUrl}/suggestions`);
    return response.data;
  }

  async getTemplates(): Promise<CollectionTemplate[]> {
    const response = await api.get(`${this.baseUrl}/templates`);
    return response.data;
  }

  async testRules(rules: CollectionRule[]): Promise<{ valid: boolean; sample_count: number; errors?: string[] }> {
    const response = await api.post(`${this.baseUrl}/test-rules`, { rules });
    return response.data;
  }

  // Bulk operations
  async bulkDeleteCollections(collectionIds: string[]): Promise<void> {
    await api.delete(`${this.baseUrl}/bulk`, { data: { collection_ids: collectionIds } });
  }

  async bulkShareCollections(collectionIds: string[], shareRequest: ShareCollectionRequest): Promise<CollectionShareInfo[]> {
    const response = await api.post(`${this.baseUrl}/bulk/share`, {
      collection_ids: collectionIds,
      share_request: shareRequest
    });
    return response.data;
  }

  async bulkExportCollections(collectionIds: string[], format: 'json' | 'csv' | 'm3u' = 'json'): Promise<Blob> {
    const response = await api.post(`${this.baseUrl}/bulk/export`, {
      collection_ids: collectionIds,
      format
    }, {
      responseType: 'blob'
    });
    return response.data;
  }

  async bulkUpdateCollections(collectionIds: string[], action: string, updates?: UpdateCollectionRequest): Promise<SmartCollection[]> {
    const response = await api.put(`${this.baseUrl}/bulk`, {
      collection_ids: collectionIds,
      action,
      updates
    });
    return response.data;
  }
}

export const collectionsApi = new CollectionsApi();
