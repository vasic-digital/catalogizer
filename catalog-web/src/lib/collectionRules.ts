import { CollectionRule, CollectionTemplate } from '../types/collections';

export const COLLECTION_FIELD_OPTIONS = [
  { value: 'title', label: 'Title', type: 'text' },
  { value: 'artist', label: 'Artist', type: 'text' },
  { value: 'album', label: 'Album', type: 'text' },
  { value: 'genre', label: 'Genre', type: 'select' },
  { value: 'year', label: 'Year', type: 'number' },
  { value: 'decade', label: 'Decade', type: 'select' },
  { value: 'duration', label: 'Duration', type: 'number' },
  { value: 'file_size', label: 'File Size', type: 'number' },
  { value: 'media_type', label: 'Media Type', type: 'select' },
  { value: 'quality', label: 'Quality', type: 'select' },
  { value: 'rating', label: 'Rating', type: 'number' },
  { value: 'play_count', label: 'Play Count', type: 'number' },
  { value: 'last_played', label: 'Last Played', type: 'date' },
  { value: 'date_added', label: 'Date Added', type: 'date' },
  { value: 'date_modified', label: 'Date Modified', type: 'date' },
  { value: 'file_extension', label: 'File Extension', type: 'text' },
  { value: 'bitrate', label: 'Bitrate', type: 'number' },
  { value: 'resolution', label: 'Resolution', type: 'select' },
  { value: 'fps', label: 'Frame Rate', type: 'number' },
  { value: 'channels', label: 'Audio Channels', type: 'number' },
  { value: 'language', label: 'Language', type: 'select' },
  { value: 'subtitles', label: 'Has Subtitles', type: 'boolean' },
  { value: 'is_favorite', label: 'Is Favorite', type: 'boolean' },
  { value: 'tags', label: 'Tags', type: 'multiselect' },
];

export const COLLECTION_OPERATORS = {
  text: [
    { value: 'contains', label: 'contains' },
    { value: 'equals', label: 'equals' },
    { value: 'starts_with', label: 'starts with' },
    { value: 'ends_with', label: 'ends with' },
    { value: 'not_contains', label: 'does not contain' },
    { value: 'not_equals', label: 'does not equal' },
    { value: 'is_empty', label: 'is empty' },
    { value: 'is_not_empty', label: 'is not empty' },
  ],
  number: [
    { value: 'equals', label: 'equals' },
    { value: 'not_equals', label: 'does not equal' },
    { value: 'greater_than', label: 'greater than' },
    { value: 'less_than', label: 'less than' },
    { value: 'greater_or_equal', label: 'greater than or equal to' },
    { value: 'less_or_equal', label: 'less than or equal to' },
    { value: 'between', label: 'between' },
    { value: 'not_between', label: 'not between' },
    { value: 'is_empty', label: 'is empty' },
    { value: 'is_not_empty', label: 'is not empty' },
  ],
  date: [
    { value: 'equals', label: 'equals' },
    { value: 'not_equals', label: 'does not equal' },
    { value: 'after', label: 'after' },
    { value: 'before', label: 'before' },
    { value: 'between', label: 'between' },
    { value: 'not_between', label: 'not between' },
    { value: 'today', label: 'is today' },
    { value: 'yesterday', label: 'is yesterday' },
    { value: 'this_week', label: 'is this week' },
    { value: 'this_month', label: 'is this month' },
    { value: 'this_year', label: 'is this year' },
    { value: 'last_7_days', label: 'in last 7 days' },
    { value: 'last_30_days', label: 'in last 30 days' },
    { value: 'last_90_days', label: 'in last 90 days' },
  ],
  select: [
    { value: 'equals', label: 'equals' },
    { value: 'not_equals', label: 'does not equal' },
    { value: 'is_any', label: 'is any of' },
    { value: 'is_not_any', label: 'is not any of' },
    { value: 'is_empty', label: 'is empty' },
    { value: 'is_not_empty', label: 'is not empty' },
  ],
  multiselect: [
    { value: 'contains_all', label: 'contains all' },
    { value: 'contains_any', label: 'contains any' },
    { value: 'contains_none', label: 'contains none' },
    { value: 'is_empty', label: 'is empty' },
    { value: 'is_not_empty', label: 'is not empty' },
  ],
  boolean: [
    { value: 'is_true', label: 'is true' },
    { value: 'is_false', label: 'is false' },
  ],
};

export const MEDIA_TYPE_OPTIONS = [
  { value: 'music', label: 'Music' },
  { value: 'video', label: 'Video' },
  { value: 'image', label: 'Image' },
  { value: 'document', label: 'Document' },
];

export const QUALITY_OPTIONS = [
  { value: 'sd', label: 'SD (Standard Definition)' },
  { value: 'hd', label: 'HD (High Definition)' },
  { value: 'fhd', label: 'FHD (Full HD)' },
  { value: 'uhd', label: 'UHD (4K)' },
  { value: '8k', label: '8K' },
];

export const GENRE_OPTIONS = [
  { value: 'rock', label: 'Rock' },
  { value: 'pop', label: 'Pop' },
  { value: 'jazz', label: 'Jazz' },
  { value: 'classical', label: 'Classical' },
  { value: 'electronic', label: 'Electronic' },
  { value: 'hip_hop', label: 'Hip Hop' },
  { value: 'country', label: 'Country' },
  { value: 'blues', label: 'Blues' },
  { value: 'reggae', label: 'Reggae' },
  { value: 'metal', label: 'Metal' },
];

export const DECADE_OPTIONS = [
  { value: '1950s', label: '1950s' },
  { value: '1960s', label: '1960s' },
  { value: '1970s', label: '1970s' },
  { value: '1980s', label: '1980s' },
  { value: '1990s', label: '1990s' },
  { value: '2000s', label: '2000s' },
  { value: '2010s', label: '2010s' },
  { value: '2020s', label: '2020s' },
];

export const RESOLUTION_OPTIONS = [
  { value: '480p', label: '480p' },
  { value: '720p', label: '720p' },
  { value: '1080p', label: '1080p' },
  { value: '1440p', label: '1440p' },
  { value: '2160p', label: '2160p (4K)' },
  { value: '4320p', label: '4320p (8K)' },
];

export const LANGUAGE_OPTIONS = [
  { value: 'en', label: 'English' },
  { value: 'es', label: 'Spanish' },
  { value: 'fr', label: 'French' },
  { value: 'de', label: 'German' },
  { value: 'it', label: 'Italian' },
  { value: 'pt', label: 'Portuguese' },
  { value: 'ru', label: 'Russian' },
  { value: 'ja', label: 'Japanese' },
  { value: 'zh', label: 'Chinese' },
  { value: 'ko', label: 'Korean' },
];

export const COLLECTION_TEMPLATES: CollectionTemplate[] = [
  {
    id: 'recently_added',
    name: 'Recently Added',
    description: 'Items added in the last 30 days',
    category: 'Time-based',
    rules: [
      {
        field: 'date_added',
        operator: 'last_30_days',
        value: null,
        field_type: 'date',
        label: 'Date Added',
      },
    ],
    icon: 'Clock',
  },
  {
    id: 'high_rated',
    name: 'High Rated',
    description: 'Items with rating 4 stars or higher',
    category: 'Quality-based',
    rules: [
      {
        field: 'rating',
        operator: 'greater_or_equal',
        value: 4,
        field_type: 'number',
        label: 'Rating',
      },
    ],
    icon: 'Star',
  },
  {
    id: 'hd_movies',
    name: 'HD Movies',
    description: 'High definition video content',
    category: 'Quality-based',
    rules: [
      {
        field: 'media_type',
        operator: 'equals',
        value: 'video',
        field_type: 'select',
        label: 'Media Type',
      },
      {
        field: 'quality',
        operator: 'is_any',
        value: ['hd', 'fhd', 'uhd', '8k'],
        field_type: 'select',
        label: 'Quality',
        condition: 'AND',
      },
    ],
    icon: 'Film',
  },
  {
    id: 'music_classics',
    name: 'Music Classics',
    description: 'Classic music from before the year 2000',
    category: 'Genre-based',
    rules: [
      {
        field: 'media_type',
        operator: 'equals',
        value: 'music',
        field_type: 'select',
        label: 'Media Type',
      },
      {
        field: 'year',
        operator: 'less_than',
        value: 2000,
        field_type: 'number',
        label: 'Year',
        condition: 'AND',
      },
    ],
    icon: 'Music',
  },
  {
    id: 'favorites',
    name: 'My Favorites',
    description: 'All items marked as favorites',
    category: 'Personal',
    rules: [
      {
        field: 'is_favorite',
        operator: 'is_true',
        value: null,
        field_type: 'boolean',
        label: 'Is Favorite',
      },
    ],
    icon: 'Heart',
  },
  {
    id: 'large_files',
    name: 'Large Files',
    description: 'Files larger than 1GB',
    category: 'Size-based',
    rules: [
      {
        field: 'file_size',
        operator: 'greater_than',
        value: 1073741824, // 1GB in bytes
        field_type: 'number',
        label: 'File Size',
      },
    ],
    icon: 'HardDrive',
  },
  {
    id: 'rock_music',
    name: 'Rock Music Collection',
    description: 'All rock music tracks',
    category: 'Genre-based',
    rules: [
      {
        field: 'media_type',
        operator: 'equals',
        value: 'music',
        field_type: 'select',
        label: 'Media Type',
      },
      {
        field: 'genre',
        operator: 'equals',
        value: 'rock',
        field_type: 'select',
        label: 'Genre',
        condition: 'AND',
      },
    ],
    icon: 'Guitar',
  },
  {
    id: 'recently_played',
    name: 'Recently Played',
    description: 'Items played in the last 7 days',
    category: 'Activity-based',
    rules: [
      {
        field: 'last_played',
        operator: 'last_7_days',
        value: null,
        field_type: 'date',
        label: 'Last Played',
      },
    ],
    icon: 'Play',
  },
];

export const getFieldOptions = (fieldType: string) => {
  switch (fieldType) {
    case 'media_type':
      return MEDIA_TYPE_OPTIONS;
    case 'quality':
      return QUALITY_OPTIONS;
    case 'genre':
      return GENRE_OPTIONS;
    case 'decade':
      return DECADE_OPTIONS;
    case 'resolution':
      return RESOLUTION_OPTIONS;
    case 'language':
      return LANGUAGE_OPTIONS;
    case 'tags':
      return [
        { value: 'favorite', label: 'Favorite' },
        { value: 'downloaded', label: 'Downloaded' },
        { value: 'watched', label: 'Watched' },
        { value: 'listened', label: 'Listened' },
      ];
    default:
      return [];
  }
};

export const getFieldLabel = (fieldValue: string) => {
  const field = COLLECTION_FIELD_OPTIONS.find(f => f.value === fieldValue);
  return field?.label || fieldValue;
};

export const getFieldType = (fieldValue: string) => {
  const field = COLLECTION_FIELD_OPTIONS.find(f => f.value === fieldValue);
  return field?.type || 'text';
};

// Operators that take a relative date and therefore need no value.
const RELATIVE_DATE_OPERATORS = new Set([
  'today', 'yesterday', 'this_week', 'this_month', 'this_year',
  'last_7_days', 'last_30_days', 'last_90_days',
]);

// Operators that take no value at all.
const VALUELESS_OPERATORS = new Set(['is_empty', 'is_not_empty', 'is_true', 'is_false', ...RELATIVE_DATE_OPERATORS]);

const hasNoValue = (value: unknown): boolean => value === null || value === undefined || value === '';

const validateDateValue = (rule: CollectionRule): string[] => {
  if (rule.operator === 'between' || rule.operator === 'not_between') {
    if (!Array.isArray(rule.value) || rule.value.length !== 2) {
      return ['Date range must have 2 values'];
    }
    return rule.value.some((v: string) => Number.isNaN(Date.parse(v))) ? ['Invalid date format in range'] : [];
  }
  if (RELATIVE_DATE_OPERATORS.has(rule.operator)) return [];
  return typeof rule.value === 'string' && Number.isNaN(Date.parse(rule.value)) ? ['Invalid date format'] : [];
};

export const validateRule = (rule: CollectionRule): string[] => {
  const errors: string[] = [];

  if (!rule.field) {
    errors.push('Field is required');
  }

  if (!rule.operator) {
    errors.push('Operator is required');
  }

  const fieldType = getFieldType(rule.field);

  // Check value requirements based on operator
  if (!VALUELESS_OPERATORS.has(rule.operator) && hasNoValue(rule.value)) {
    errors.push('Value is required for this operator');
  }

  // Type-specific validation
  if (rule.value !== null && rule.value !== undefined) {
    if (fieldType === 'number' && Number.isNaN(Number(rule.value))) {
      errors.push('Value must be a number');
    }
    if (fieldType === 'date') {
      errors.push(...validateDateValue(rule));
    }
  }

  return errors;
};

export const validateRules = (rules: CollectionRule[]): string[] => {
  const allErrors: string[] = [];
  
  if (rules.length === 0) {
    allErrors.push('At least one rule is required');
    return allErrors;
  }
  
  rules.forEach((rule, index) => {
    const ruleErrors = validateRule(rule);
    if (ruleErrors.length > 0) {
      allErrors.push(`Rule ${index + 1}: ${ruleErrors.join(', ')}`);
    }
  });
  
  return allErrors;
};