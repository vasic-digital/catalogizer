package services

import (
	"context"
	"math"
	"regexp"
	"strings"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

// Duplicate detection and deduplication service
type DuplicateDetectionService struct {
	db           *database.DB
	logger       *zap.Logger
	cacheService *CacheService
}

// Duplicate detection result
type DuplicateGroup struct {
	ID              string          `json:"id"`
	MediaType       MediaType       `json:"media_type"`
	PrimaryItem     DuplicateItem   `json:"primary_item"`
	DuplicateItems  []DuplicateItem `json:"duplicate_items"`
	Confidence      float64         `json:"confidence"`
	DetectionMethod string          `json:"detection_method"`
	MatchTypes      []string        `json:"match_types"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Status          string          `json:"status"` // pending, confirmed, dismissed
	AutoResolved    bool            `json:"auto_resolved"`
}

type DuplicateItem struct {
	MediaID      string                 `json:"media_id"`
	FilePath     string                 `json:"file_path"`
	FileName     string                 `json:"file_name"`
	FileSize     int64                  `json:"file_size"`
	FileHash     string                 `json:"file_hash"`
	Title        string                 `json:"title"`
	Artist       string                 `json:"artist,omitempty"`
	Album        string                 `json:"album,omitempty"`
	Director     string                 `json:"director,omitempty"`
	Author       string                 `json:"author,omitempty"`
	Year         int                    `json:"year,omitempty"`
	Duration     int64                  `json:"duration,omitempty"`
	Quality      string                 `json:"quality,omitempty"`
	Format       string                 `json:"format,omitempty"`
	Bitrate      int                    `json:"bitrate,omitempty"`
	Resolution   string                 `json:"resolution,omitempty"`
	ExternalIDs  map[string]string      `json:"external_ids"`
	Fingerprints map[string]string      `json:"fingerprints"`
	Similarity   float64                `json:"similarity"`
	MatchReasons []string               `json:"match_reasons"`
	Metadata     map[string]interface{} `json:"metadata"`
	AddedAt      time.Time              `json:"added_at"`
	LastSeen     time.Time              `json:"last_seen"`
}

// Similarity calculation components
type SimilarityAnalysis struct {
	OverallScore          float64            `json:"overall_score"`
	TitleSimilarity       float64            `json:"title_similarity"`
	MetadataSimilarity    float64            `json:"metadata_similarity"`
	FingerprintSimilarity float64            `json:"fingerprint_similarity"`
	FileSimilarity        float64            `json:"file_similarity"`
	ExternalIDMatch       bool               `json:"external_id_match"`
	HashMatch             bool               `json:"hash_match"`
	Components            map[string]float64 `json:"components"`
	MatchingFields        []string           `json:"matching_fields"`
	DifferencesFound      []string           `json:"differences_found"`
}

// Text similarity algorithms
type TextSimilarityMetrics struct {
	LevenshteinDistance int     `json:"levenshtein_distance"`
	JaroWinklerScore    float64 `json:"jaro_winkler_score"`
	CosineSimilarity    float64 `json:"cosine_similarity"`
	JaccardIndex        float64 `json:"jaccard_index"`
	LCSRatio            float64 `json:"lcs_ratio"`
	SoundexMatch        bool    `json:"soundex_match"`
	MetaphoneMatch      bool    `json:"metaphone_match"`
}

// Deduplication action
type DeduplicationAction struct {
	GroupID       string                 `json:"group_id"`
	Action        string                 `json:"action"` // keep_primary, keep_best_quality, merge, custom
	PrimaryItemID string                 `json:"primary_item_id"`
	ItemsToRemove []string               `json:"items_to_remove"`
	ItemsToKeep   []string               `json:"items_to_keep"`
	MergeStrategy string                 `json:"merge_strategy,omitempty"`
	UserID        int64                  `json:"user_id"`
	Reason        string                 `json:"reason"`
	ExecutedAt    time.Time              `json:"executed_at"`
	RollbackData  map[string]interface{} `json:"rollback_data,omitempty"`
}

// Duplicate detection request
type DuplicateDetectionRequest struct {
	MediaTypes       []MediaType `json:"media_types,omitempty"`
	IncludePaths     []string    `json:"include_paths,omitempty"`
	ExcludePaths     []string    `json:"exclude_paths,omitempty"`
	MinSimilarity    float64     `json:"min_similarity"`
	DetectionMethods []string    `json:"detection_methods"`
	IncludeExisting  bool        `json:"include_existing"`
	BatchSize        int         `json:"batch_size"`
	UserID           int64       `json:"user_id"`
}

func NewDuplicateDetectionService(
	db *database.DB,
	logger *zap.Logger,
	cacheService *CacheService,
) *DuplicateDetectionService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DuplicateDetectionService{
		db:           db,
		logger:       logger,
		cacheService: cacheService,
	}
}

// Main duplicate detection method
func (s *DuplicateDetectionService) DetectDuplicates(ctx context.Context, req *DuplicateDetectionRequest) ([]DuplicateGroup, error) {
	s.logger.Info("Starting duplicate detection",
		zap.Int("media_types", len(req.MediaTypes)),
		zap.Float64("min_similarity", req.MinSimilarity),
		zap.Int64("user_id", req.UserID))

	duplicateGroups := []DuplicateGroup{}

	// Process each media type
	mediaTypes := req.MediaTypes
	if len(mediaTypes) == 0 {
		// Default to all types
		mediaTypes = []MediaType{
			MediaTypeMovie, MediaTypeTVSeries, MediaTypeTVEpisode,
			MediaTypeMusic, MediaTypeAlbum, MediaTypeAudiobook,
			MediaTypeBook, MediaTypeComicBook, MediaTypeMagazine,
			MediaTypeGame, MediaTypeSoftware,
		}
	}

	for _, mediaType := range mediaTypes {
		groups, err := s.detectDuplicatesForMediaType(ctx, mediaType, req)
		if err != nil {
			s.logger.Error("Failed to detect duplicates for media type",
				zap.String("media_type", string(mediaType)),
				zap.Error(err))
			continue
		}
		duplicateGroups = append(duplicateGroups, groups...)
	}

	// Store detected duplicates
	for _, group := range duplicateGroups {
		if err := s.storeDuplicateGroup(ctx, &group); err != nil {
			s.logger.Error("Failed to store duplicate group",
				zap.String("group_id", group.ID),
				zap.Error(err))
		}
	}

	s.logger.Info("Duplicate detection completed",
		zap.Int("groups_found", len(duplicateGroups)))

	return duplicateGroups, nil
}

// Detect duplicates for a specific media type
func (s *DuplicateDetectionService) detectDuplicatesForMediaType(ctx context.Context, mediaType MediaType, req *DuplicateDetectionRequest) ([]DuplicateGroup, error) {
	// Get all items of this media type
	items, err := s.getMediaItems(ctx, mediaType, req)
	if err != nil {
		return nil, err
	}

	if len(items) < 2 {
		return []DuplicateGroup{}, nil
	}

	s.logger.Debug("Processing media items for duplicates",
		zap.String("media_type", string(mediaType)),
		zap.Int("item_count", len(items)))

	var duplicateGroups []DuplicateGroup

	// Compare each item with every other item
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			similarity := s.calculateSimilarity(ctx, &items[i], &items[j], mediaType)

			if similarity.OverallScore >= req.MinSimilarity {
				// Create or add to duplicate group
				group := s.createOrAddToDuplicateGroup(duplicateGroups, items[i], items[j], similarity, mediaType)
				if group != nil {
					duplicateGroups = s.mergeDuplicateGroup(duplicateGroups, *group)
				}
			}
		}
	}

	// Post-process groups to determine primary items
	for i := range duplicateGroups {
		s.determinePrimaryItem(&duplicateGroups[i])
	}

	return duplicateGroups, nil
}

// Calculate similarity between two media items
func (s *DuplicateDetectionService) calculateSimilarity(ctx context.Context, item1, item2 *DuplicateItem, mediaType MediaType) *SimilarityAnalysis {
	analysis := &SimilarityAnalysis{
		Components:       make(map[string]float64),
		MatchingFields:   []string{},
		DifferencesFound: []string{},
	}

	// 1. Exact hash match (highest priority)
	if item1.FileHash != "" && item2.FileHash != "" && item1.FileHash == item2.FileHash {
		analysis.HashMatch = true
		analysis.OverallScore = 1.0
		analysis.MatchingFields = append(analysis.MatchingFields, "file_hash")
		return analysis
	}

	// 2. External ID match
	for key, id1 := range item1.ExternalIDs {
		if id2, exists := item2.ExternalIDs[key]; exists && id1 == id2 && id1 != "" {
			analysis.ExternalIDMatch = true
			analysis.OverallScore = 0.95
			analysis.MatchingFields = append(analysis.MatchingFields, "external_id_"+key)
			return analysis
		}
	}

	// 3. Comprehensive similarity calculation
	weights := s.getSimilarityWeights(mediaType)

	// Title similarity
	analysis.TitleSimilarity = s.calculateTextSimilarity(item1.Title, item2.Title)
	analysis.Components["title"] = analysis.TitleSimilarity * weights["title"]

	// Metadata similarity based on media type
	switch mediaType {
	case MediaTypeMovie, MediaTypeTVSeries, MediaTypeTVEpisode:
		analysis.MetadataSimilarity = s.calculateVideoMetadataSimilarity(item1, item2)
	case MediaTypeMusic, MediaTypeAlbum, MediaTypeAudiobook:
		analysis.MetadataSimilarity = s.calculateAudioMetadataSimilarity(item1, item2)
	case MediaTypeBook, MediaTypeComicBook, MediaTypeMagazine:
		analysis.MetadataSimilarity = s.calculateBookMetadataSimilarity(item1, item2)
	case MediaTypeGame, MediaTypeSoftware:
		analysis.MetadataSimilarity = s.calculateSoftwareMetadataSimilarity(item1, item2)
	default:
		analysis.MetadataSimilarity = s.calculateGenericMetadataSimilarity(item1, item2)
	}
	analysis.Components["metadata"] = analysis.MetadataSimilarity * weights["metadata"]

	// Fingerprint similarity
	if len(item1.Fingerprints) > 0 && len(item2.Fingerprints) > 0 {
		analysis.FingerprintSimilarity = s.calculateFingerprintSimilarity(item1.Fingerprints, item2.Fingerprints)
		analysis.Components["fingerprint"] = analysis.FingerprintSimilarity * weights["fingerprint"]
	}

	// File similarity (size, format, quality)
	analysis.FileSimilarity = s.calculateFileSimilarity(item1, item2)
	analysis.Components["file"] = analysis.FileSimilarity * weights["file"]

	// Calculate overall score
	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	analysis.OverallScore = 0.0
	for _, score := range analysis.Components {
		analysis.OverallScore += score
	}
	if totalWeight > 0 {
		analysis.OverallScore /= totalWeight
	}

	// Determine matching fields and differences
	s.analyzeFieldMatches(item1, item2, analysis)

	return analysis
}

// Calculate text similarity using multiple algorithms
func (s *DuplicateDetectionService) calculateTextSimilarity(text1, text2 string) float64 {
	if text1 == "" || text2 == "" {
		return 0.0
	}

	// Normalize texts
	norm1 := s.normalizeText(text1)
	norm2 := s.normalizeText(text2)

	if norm1 == norm2 {
		return 1.0
	}

	// Calculate multiple similarity metrics
	metrics := s.calculateTextMetrics(norm1, norm2)

	// Weighted combination of metrics
	score := 0.0
	score += metrics.JaroWinklerScore * 0.4
	score += metrics.CosineSimilarity * 0.3
	score += metrics.JaccardIndex * 0.2
	score += metrics.LCSRatio * 0.1

	// Bonus for exact phonetic matches
	if metrics.SoundexMatch {
		score += 0.1
	}
	if metrics.MetaphoneMatch {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// Calculate video metadata similarity
func (s *DuplicateDetectionService) calculateVideoMetadataSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// Director similarity
	if item1.Director != "" && item2.Director != "" {
		score += s.calculateTextSimilarity(item1.Director, item2.Director)
		components++
	}

	// Year similarity
	if item1.Year > 0 && item2.Year > 0 {
		yearDiff := math.Abs(float64(item1.Year - item2.Year))
		yearScore := math.Max(0, 1.0-yearDiff/5.0) // Penalty for year differences
		score += yearScore
		components++
	}

	// Duration similarity
	if item1.Duration > 0 && item2.Duration > 0 {
		durationDiff := math.Abs(float64(item1.Duration - item2.Duration))
		durationScore := math.Max(0, 1.0-durationDiff/600000) // 10 minute tolerance
		score += durationScore
		components++
	}

	// Quality/Resolution similarity
	if item1.Quality != "" && item2.Quality != "" {
		qualityScore := 0.0
		if item1.Quality == item2.Quality {
			qualityScore = 1.0
		} else if s.areQualitiesSimilar(item1.Quality, item2.Quality) {
			qualityScore = 0.7
		}
		score += qualityScore
		components++
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Calculate audio metadata similarity
func (s *DuplicateDetectionService) calculateAudioMetadataSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// Artist similarity
	if item1.Artist != "" && item2.Artist != "" {
		score += s.calculateTextSimilarity(item1.Artist, item2.Artist)
		components++
	}

	// Album similarity
	if item1.Album != "" && item2.Album != "" {
		score += s.calculateTextSimilarity(item1.Album, item2.Album)
		components++
	}

	// Year similarity
	if item1.Year > 0 && item2.Year > 0 {
		yearDiff := math.Abs(float64(item1.Year - item2.Year))
		yearScore := math.Max(0, 1.0-yearDiff/2.0) // Smaller tolerance for music
		score += yearScore
		components++
	}

	// Duration similarity
	if item1.Duration > 0 && item2.Duration > 0 {
		durationDiff := math.Abs(float64(item1.Duration - item2.Duration))
		durationScore := math.Max(0, 1.0-durationDiff/30000) // 30 second tolerance
		score += durationScore
		components++
	}

	// Bitrate similarity
	if item1.Bitrate > 0 && item2.Bitrate > 0 {
		bitrateDiff := math.Abs(float64(item1.Bitrate - item2.Bitrate))
		bitrateScore := math.Max(0, 1.0-bitrateDiff/320) // Normalize by 320kbps
		score += bitrateScore
		components++
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Calculate book metadata similarity
func (s *DuplicateDetectionService) calculateBookMetadataSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// Author similarity
	if item1.Author != "" && item2.Author != "" {
		score += s.calculateTextSimilarity(item1.Author, item2.Author)
		components++
	}

	// Year similarity
	if item1.Year > 0 && item2.Year > 0 {
		yearDiff := math.Abs(float64(item1.Year - item2.Year))
		yearScore := math.Max(0, 1.0-yearDiff/5.0)
		score += yearScore
		components++
	}

	// Check for ISBN matches in external IDs
	isbn1 := s.extractISBN(item1.ExternalIDs)
	isbn2 := s.extractISBN(item2.ExternalIDs)
	if isbn1 != "" && isbn2 != "" {
		if isbn1 == isbn2 {
			score += 1.0
		} else if s.areISBNsRelated(isbn1, isbn2) {
			score += 0.8
		}
		components++
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Calculate software metadata similarity
func (s *DuplicateDetectionService) calculateSoftwareMetadataSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// Version similarity
	version1 := s.extractVersion(item1.Metadata)
	version2 := s.extractVersion(item2.Metadata)
	if version1 != "" && version2 != "" {
		score += s.calculateVersionSimilarity(version1, version2)
		components++
	}

	// Platform similarity
	platform1 := s.extractPlatform(item1.Metadata)
	platform2 := s.extractPlatform(item2.Metadata)
	if platform1 != "" && platform2 != "" {
		if platform1 == platform2 {
			score += 1.0
		} else if s.arePlatformsSimilar(platform1, platform2) {
			score += 0.5
		}
		components++
	}

	// File size similarity (important for software)
	if item1.FileSize > 0 && item2.FileSize > 0 {
		sizeDiff := math.Abs(float64(item1.FileSize - item2.FileSize))
		sizeScore := math.Max(0, 1.0-sizeDiff/float64(math.Max(float64(item1.FileSize), float64(item2.FileSize))))
		score += sizeScore
		components++
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Calculate generic metadata similarity
func (s *DuplicateDetectionService) calculateGenericMetadataSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// File name similarity
	score += s.calculateTextSimilarity(item1.FileName, item2.FileName)
	components++

	// Format similarity
	if item1.Format != "" && item2.Format != "" {
		if item1.Format == item2.Format {
			score += 1.0
		} else if s.areFormatsSimilar(item1.Format, item2.Format) {
			score += 0.5
		}
		components++
	}

	// File size similarity
	if item1.FileSize > 0 && item2.FileSize > 0 {
		sizeDiff := math.Abs(float64(item1.FileSize - item2.FileSize))
		maxSize := math.Max(float64(item1.FileSize), float64(item2.FileSize))
		if maxSize > 0 {
			sizeScore := math.Max(0, 1.0-sizeDiff/maxSize)
			score += sizeScore
			components++
		}
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Calculate fingerprint similarity
func (s *DuplicateDetectionService) calculateFingerprintSimilarity(fp1, fp2 map[string]string) float64 {
	if len(fp1) == 0 || len(fp2) == 0 {
		return 0.0
	}

	maxScore := 0.0

	// Compare each fingerprint type
	for fpType, hash1 := range fp1 {
		if hash2, exists := fp2[fpType]; exists {
			similarity := s.calculateHashSimilarity(hash1, hash2, fpType)
			if similarity > maxScore {
				maxScore = similarity
			}
		}
	}

	return maxScore
}

// Calculate file similarity
func (s *DuplicateDetectionService) calculateFileSimilarity(item1, item2 *DuplicateItem) float64 {
	score := 0.0
	components := 0

	// File name similarity
	score += s.calculateTextSimilarity(item1.FileName, item2.FileName)
	components++

	// File size similarity
	if item1.FileSize > 0 && item2.FileSize > 0 {
		sizeDiff := math.Abs(float64(item1.FileSize - item2.FileSize))
		maxSize := math.Max(float64(item1.FileSize), float64(item2.FileSize))
		if maxSize > 0 {
			sizeScore := 1.0 - sizeDiff/maxSize
			if sizeScore < 0 {
				sizeScore = 0
			}
			score += sizeScore
			components++
		}
	}

	// Format similarity
	if item1.Format != "" && item2.Format != "" {
		if strings.EqualFold(item1.Format, item2.Format) {
			score += 1.0
		} else if s.areFormatsSimilar(item1.Format, item2.Format) {
			score += 0.7
		}
		components++
	}

	if components > 0 {
		return score / float64(components)
	}
	return 0.0
}

// Helper methods for text analysis
func (s *DuplicateDetectionService) normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Remove common articles and prepositions
	stopWords := []string{"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by"}
	words := strings.Fields(text)
	var filtered []string

	for _, word := range words {
		isStopWord := false
		for _, stopWord := range stopWords {
			if word == stopWord {
				isStopWord = true
				break
			}
		}
		if !isStopWord {
			filtered = append(filtered, word)
		}
	}

	// Remove special characters and normalize spaces
	normalized := strings.Join(filtered, " ")
	normalized = regexp.MustCompile(`[^\p{L}\p{N}\s]`).ReplaceAllString(normalized, "")
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	return normalized
}

func (s *DuplicateDetectionService) calculateTextMetrics(text1, text2 string) *TextSimilarityMetrics {
	metrics := &TextSimilarityMetrics{}

	// Levenshtein distance
	metrics.LevenshteinDistance = s.levenshteinDistance(text1, text2)

	// Jaro-Winkler similarity
	metrics.JaroWinklerScore = s.jaroWinklerSimilarity(text1, text2)

	// Cosine similarity
	metrics.CosineSimilarity = s.cosineSimilarity(text1, text2)

	// Jaccard index
	metrics.JaccardIndex = s.jaccardIndex(text1, text2)

	// LCS ratio
	metrics.LCSRatio = s.lcsRatio(text1, text2)

	// Soundex match
	metrics.SoundexMatch = s.soundexMatch(text1, text2)

	// Metaphone match
	metrics.MetaphoneMatch = s.metaphoneMatch(text1, text2)

	return metrics
}

// Levenshtein distance algorithm
func (s *DuplicateDetectionService) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}

	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// Jaro-Winkler similarity
func (s *DuplicateDetectionService) jaroWinklerSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	len1, len2 := len(s1), len(s2)
	if len1 == 0 || len2 == 0 {
		return 0.0
	}

	matchWindow := max(len1, len2)/2 - 1
	if matchWindow < 0 {
		matchWindow = 0
	}

	s1Matches := make([]bool, len1)
	s2Matches := make([]bool, len2)

	matches := 0
	transpositions := 0

	// Find matches
	for i := 0; i < len1; i++ {
		start := max(0, i-matchWindow)
		end := min(i+matchWindow+1, len2)

		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	// Find transpositions
	k := 0
	for i := 0; i < len1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	jaro := (float64(matches)/float64(len1) + float64(matches)/float64(len2) + float64(matches-transpositions/2)/float64(matches)) / 3.0

	// Jaro-Winkler adjustment
	prefix := 0
	for i := 0; i < min(len1, len2) && i < 4; i++ {
		if s1[i] == s2[i] {
			prefix++
		} else {
			break
		}
	}

	return jaro + 0.1*float64(prefix)*(1.0-jaro)
}

// Cosine similarity
func (s *DuplicateDetectionService) cosineSimilarity(text1, text2 string) float64 {
	words1 := strings.Fields(text1)
	words2 := strings.Fields(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create word frequency maps
	freq1 := make(map[string]int)
	freq2 := make(map[string]int)

	for _, word := range words1 {
		freq1[word]++
	}
	for _, word := range words2 {
		freq2[word]++
	}

	// Calculate dot product and magnitudes
	dotProduct := 0.0
	magnitude1 := 0.0
	magnitude2 := 0.0

	allWords := make(map[string]bool)
	for word := range freq1 {
		allWords[word] = true
	}
	for word := range freq2 {
		allWords[word] = true
	}

	for word := range allWords {
		f1 := float64(freq1[word])
		f2 := float64(freq2[word])

		dotProduct += f1 * f2
		magnitude1 += f1 * f1
		magnitude2 += f2 * f2
	}

	if magnitude1 == 0 || magnitude2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(magnitude1) * math.Sqrt(magnitude2))
}

// Jaccard index
func (s *DuplicateDetectionService) jaccardIndex(text1, text2 string) float64 {
	words1 := make(map[string]bool)
	words2 := make(map[string]bool)

	for _, word := range strings.Fields(text1) {
		words1[word] = true
	}
	for _, word := range strings.Fields(text2) {
		words2[word] = true
	}

	intersection := 0
	union := 0

	allWords := make(map[string]bool)
	for word := range words1 {
		allWords[word] = true
	}
	for word := range words2 {
		allWords[word] = true
	}

	for word := range allWords {
		inFirst := words1[word]
		inSecond := words2[word]

		if inFirst && inSecond {
			intersection++
		}
		union++
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// Longest Common Subsequence ratio
func (s *DuplicateDetectionService) lcsRatio(s1, s2 string) float64 {
	lcsLength := s.lcsLength(s1, s2)
	maxLength := max(len(s1), len(s2))

	if maxLength == 0 {
		return 1.0
	}

	return float64(lcsLength) / float64(maxLength)
}

func (s *DuplicateDetectionService) lcsLength(s1, s2 string) int {
	m, n := len(s1), len(s2)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	return dp[m][n]
}

// Soundex matching (simplified)
func (s *DuplicateDetectionService) soundexMatch(s1, s2 string) bool {
	return s.soundex(s1) == s.soundex(s2)
}

func (s *DuplicateDetectionService) soundex(str string) string {
	if len(str) == 0 {
		return "0000"
	}

	str = strings.ToUpper(str)
	result := string(str[0])

	// Mapping table
	mapping := map[rune]string{
		'B': "1", 'F': "1", 'P': "1", 'V': "1",
		'C': "2", 'G': "2", 'J': "2", 'K': "2", 'Q': "2", 'S': "2", 'X': "2", 'Z': "2",
		'D': "3", 'T': "3",
		'L': "4",
		'M': "5", 'N': "5",
		'R': "6",
	}

	prev := ""
	for _, char := range str[1:] {
		if code, exists := mapping[char]; exists {
			if code != prev {
				result += code
				prev = code
			}
		} else {
			prev = ""
		}

		if len(result) >= 4 {
			break
		}
	}

	// Pad with zeros
	for len(result) < 4 {
		result += "0"
	}

	return result[:4]
}

// Metaphone matching (simplified)
func (s *DuplicateDetectionService) metaphoneMatch(s1, s2 string) bool {
	return s.metaphone(s1) == s.metaphone(s2)
}

func (s *DuplicateDetectionService) metaphone(str string) string {
	if len(str) == 0 {
		return ""
	}

	str = strings.ToUpper(str)
	result := ""

	// Simplified metaphone algorithm
	for i, char := range str {
		switch char {
		case 'A', 'E', 'I', 'O', 'U':
			if i == 0 {
				result += string(char)
			}
		case 'B':
			if i == len(str)-1 && str[i-1] == 'M' {
				continue
			}
			result += "B"
		case 'C':
			if i < len(str)-1 && str[i+1] == 'H' {
				result += "X"
			} else {
				result += "K"
			}
		case 'D':
			result += "T"
		case 'F', 'J', 'L', 'M', 'N', 'R':
			result += string(char)
		case 'G':
			result += "K"
		case 'H':
			if i == 0 || isVowel(rune(str[i-1])) {
				result += "H"
			}
		case 'K':
			if i == 0 || str[i-1] != 'C' {
				result += "K"
			}
		case 'P':
			if i < len(str)-1 && str[i+1] == 'H' {
				result += "F"
			} else {
				result += "P"
			}
		case 'Q':
			result += "K"
		case 'S':
			result += "S"
		case 'T':
			if i < len(str)-1 && str[i+1] == 'H' {
				result += "0"
			} else {
				result += "T"
			}
		case 'V':
			result += "F"
		case 'W', 'Y':
			if i == 0 || isVowel(rune(str[i-1])) {
				result += string(char)
			}
		case 'X':
			result += "KS"
		case 'Z':
			result += "S"
		}
	}

	return result
}

func isVowel(r rune) bool {
	return r == 'A' || r == 'E' || r == 'I' || r == 'O' || r == 'U'
}

// Utility functions
func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Additional helper methods would continue here...
// (Implementation continues with remaining methods for duplicate group management,
// deduplication actions, storage methods, etc.)

// Placeholder implementations for some methods
func (s *DuplicateDetectionService) getSimilarityWeights(mediaType MediaType) map[string]float64 {
	switch mediaType {
	case MediaTypeMusic:
		return map[string]float64{
			"title":       0.4,
			"metadata":    0.4,
			"fingerprint": 0.15,
			"file":        0.05,
		}
	case MediaTypeMovie:
		return map[string]float64{
			"title":       0.5,
			"metadata":    0.3,
			"fingerprint": 0.1,
			"file":        0.1,
		}
	default:
		return map[string]float64{
			"title":       0.4,
			"metadata":    0.3,
			"fingerprint": 0.2,
			"file":        0.1,
		}
	}
}

func (s *DuplicateDetectionService) getMediaItems(ctx context.Context, mediaType MediaType, req *DuplicateDetectionRequest) ([]DuplicateItem, error) {
	// Implementation would query database for media items
	return []DuplicateItem{}, nil
}

func (s *DuplicateDetectionService) createOrAddToDuplicateGroup(groups []DuplicateGroup, item1, item2 DuplicateItem, similarity *SimilarityAnalysis, mediaType MediaType) *DuplicateGroup {
	// Implementation would create new group or add to existing
	return nil
}

func (s *DuplicateDetectionService) mergeDuplicateGroup(groups []DuplicateGroup, newGroup DuplicateGroup) []DuplicateGroup {
	// Implementation would merge groups if needed
	return groups
}

func (s *DuplicateDetectionService) determinePrimaryItem(group *DuplicateGroup) {
	// Implementation would determine the best quality/primary item
}

func (s *DuplicateDetectionService) storeDuplicateGroup(ctx context.Context, group *DuplicateGroup) error {
	// Implementation would store duplicate group in database
	return nil
}

func (s *DuplicateDetectionService) analyzeFieldMatches(item1, item2 *DuplicateItem, analysis *SimilarityAnalysis) {
	// Implementation would analyze which fields match and which differ
}

func (s *DuplicateDetectionService) calculateHashSimilarity(hash1, hash2, fpType string) float64 {
	// Implementation would calculate hash similarity based on type
	return 0.0
}

func (s *DuplicateDetectionService) areQualitiesSimilar(q1, q2 string) bool {
	// Implementation would check if qualities are similar
	return false
}

func (s *DuplicateDetectionService) extractISBN(externalIDs map[string]string) string {
	// Implementation would extract ISBN from external IDs
	return ""
}

func (s *DuplicateDetectionService) areISBNsRelated(isbn1, isbn2 string) bool {
	// Implementation would check if ISBNs are related (ISBN-10 vs ISBN-13)
	return false
}

func (s *DuplicateDetectionService) extractVersion(metadata map[string]interface{}) string {
	// Implementation would extract version from metadata
	return ""
}

func (s *DuplicateDetectionService) extractPlatform(metadata map[string]interface{}) string {
	// Implementation would extract platform from metadata
	return ""
}

func (s *DuplicateDetectionService) calculateVersionSimilarity(v1, v2 string) float64 {
	// Implementation would calculate version similarity
	return 0.0
}

func (s *DuplicateDetectionService) arePlatformsSimilar(p1, p2 string) bool {
	// Implementation would check if platforms are similar
	return false
}

func (s *DuplicateDetectionService) areFormatsSimilar(f1, f2 string) bool {
	// Implementation would check if formats are similar
	return false
}
