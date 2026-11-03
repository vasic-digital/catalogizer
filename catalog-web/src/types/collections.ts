export interface Collection {
  id: string;
  name: string;
  description?: string;
  item_count: number;
  is_public: boolean;
  is_smart: boolean;
  primary_media_type: 'music' | 'video' | 'image' | 'document' | 'mixed';
  created_at: string;
  updated_at: string;
  thumbnail_url?: string;
  cover_image?: string;
  owner_id: string;
}

export interface SmartCollection {
  id: string;
  name: string;
  description?: string;
  is_smart: boolean;
  smart_rules: CollectionRule[];
  item_count: number;
  last_updated: string;
  created_at: string;
  updated_at: string;
  thumbnail_url?: string;
  cover_image?: string;
  owner_id: string;
  is_public: boolean;
  primary_media_type: 'music' | 'video' | 'image' | 'document' | 'mixed';
}

export interface CollectionRule {
  id: string;
  field: string;
  operator: string;
  value: string | number | boolean | string[] | null;
  condition?: 'AND' | 'OR';
  nested_rules?: CollectionRule[];
  field_type: 'text' | 'number' | 'date' | 'boolean' | 'select' | 'multiselect';
  label: string;
}

export interface CollectionTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  rules: Omit<CollectionRule, 'id'>[];
  icon?: string;
}

export type CollectionTemplateRule = Omit<CollectionRule, 'id'>;

export interface CollectionAnalytics {
  collection_id: string;
  total_items: number;
  media_type_distribution: {
    music: number;
    video: number;
    image: number;
    document: number;
  };
  size_distribution: {
    total_size_bytes: number;
    average_size_bytes: number;
    largest_item_size_bytes: number;
  };
  quality_distribution: {
    hd: number;
    sd: number;
    uhd: number;
  };
  time_based_stats: {
    items_added_today: number;
    items_added_this_week: number;
    items_added_this_month: number;
    oldest_item_date: string;
    newest_item_date: string;
  };
  genre_distribution?: Record<string, number>;
  artist_distribution?: Record<string, number>;
  decade_distribution?: Record<string, number>;
  engagement_stats: {
    total_views: number;
    unique_viewers: number;
    total_plays: number;
    average_completion_rate: number;
    last_accessed: string;
  };
}

export interface CreateCollectionRequest {
  name: string;
  description?: string;
  is_public?: boolean;
  is_smart: true;
  smart_rules: CollectionRule[];
}

export interface UpdateCollectionRequest {
  name?: string;
  description?: string;
  is_public?: boolean;
  smart_rules?: CollectionRule[];
}

export interface ShareCollectionRequest {
  can_view: boolean;
  can_comment: boolean;
  can_download: boolean;
  expires_at?: string;
  allow_reshare?: boolean;
}

export interface CollectionShareInfo {
  share_url: string;
  share_id: string;
  expires_at?: string;
  permissions: ShareCollectionRequest;
  created_at: string;
  access_count: number;
}