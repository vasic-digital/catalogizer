package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
	"catalogizer/repository"

	"go.uber.org/zap"
)

// gamePlatformRe detects game platform hints in directory names.
// Kept here after title_parser.go was refactored to use digital.vasic.entities.
var gamePlatformRe = regexp.MustCompile(`(?i)\b(?:PC|Windows|Linux|Mac|macOS|PS[2-5]|PlayStation[\s._-]*[2-5]?|Xbox(?:[\s._-]*(?:One|360|Series[\s._-]*[XS]))?|Switch|Nintendo|GOG|Steam)\b`)

// AggregationService bridges the scan pipeline to structured media entities.
// After a scan completes, it analyzes scanned directories, detects media types,
// creates/updates MediaItem entities, and links files to them.
type AggregationService struct {
	db              *database.DB
	logger          *zap.Logger
	itemRepo        *repository.MediaItemRepository
	fileRepo        *repository.MediaFileRepository
	dirAnalysisRepo *repository.DirectoryAnalysisRepository
	extMetaRepo     *repository.ExternalMetadataRepository
}

// NewAggregationService creates a new aggregation service.
func NewAggregationService(
	db *database.DB,
	logger *zap.Logger,
	itemRepo *repository.MediaItemRepository,
	fileRepo *repository.MediaFileRepository,
	dirAnalysisRepo *repository.DirectoryAnalysisRepository,
	extMetaRepo *repository.ExternalMetadataRepository,
) *AggregationService {
	return &AggregationService{
		db:              db,
		logger:          logger,
		itemRepo:        itemRepo,
		fileRepo:        fileRepo,
		dirAnalysisRepo: dirAnalysisRepo,
		extMetaRepo:     extMetaRepo,
	}
}

// AggregateAfterScan runs after a scan completes to create entities from files.
//
// DEFECT-E fix (FINDING_aggregation_granularity.md): aggregation groups at TITLE
// granularity by walking the media-bearing leaf files, NOT at top-level-directory
// granularity. The old path (aggregateAfterScanLegacy) emitted one entity per
// top-level directory titled after the FOLDER name (so "movies"/"music"/"series"
// became entities while per-title files never did, and series/ — whose content
// lives under Season NN/ subdirs with zero direct leaf children — produced no
// entity at all). The walk below resolves each entity by its parsed title (the
// FILE name for movies/software, the show name for TV, the album folder for
// music), so category folders never become entities and shows whose folder has
// no direct leaf children still get a browsable tv_show + season/episode tree.
func (s *AggregationService) AggregateAfterScan(ctx context.Context, storageRootID int64) error {
	s.logger.Info("Starting post-scan aggregation", zap.Int64("storage_root_id", storageRootID))

	leaves, err := s.getMediaLeafFiles(ctx, storageRootID)
	if err != nil {
		return fmt.Errorf("get media leaf files: %w", err)
	}

	groups := s.groupLeafFilesByTitle(leaves)

	s.logger.Info("Grouped media files into title entities",
		zap.Int("leaf_files", len(leaves)),
		zap.Int("title_groups", len(groups)),
		zap.Int64("storage_root_id", storageRootID))

	created, updated := 0, 0
	for _, g := range groups {
		isNew, err := s.processTitleGroup(ctx, g, storageRootID)
		if err != nil {
			s.logger.Warn("Failed to process title group",
				zap.String("title", g.title),
				zap.String("media_type", g.mediaType),
				zap.Error(err))
			continue
		}
		if isNew {
			created++
		} else {
			updated++
		}
	}

	s.logger.Info("Post-scan aggregation completed",
		zap.Int64("storage_root_id", storageRootID),
		zap.Int("entities_created", created),
		zap.Int("entities_updated", updated))

	// Auto-enrich new entities with TMDB metadata (cover art, descriptions)
	if created > 0 {
		go s.enrichNewEntities(ctx, storageRootID, created)
	}

	return nil
}

// aggregateAfterScanLegacy is the PRE-DEFECT-E aggregation path: it keyed on
// top-level directories (parent_id IS NULL) and emitted ONE entity per top-level
// directory titled after the FOLDER name. It is retained ONLY as the RED leg of
// the §11.4.135 regression guard (TestAggregationGranularity_PerTitleEntities
// with RED_MODE=1) so the guard can reproduce the historical defect on demand.
// Production code MUST use AggregateAfterScan (the title-granular walk).
func (s *AggregationService) aggregateAfterScanLegacy(ctx context.Context, storageRootID int64) error {
	s.logger.Info("Starting post-scan aggregation (legacy dir-granular path)", zap.Int64("storage_root_id", storageRootID))

	// Get top-level directories from the scanned storage root
	dirs, err := s.getTopLevelDirectories(ctx, storageRootID)
	if err != nil {
		return fmt.Errorf("get top-level directories: %w", err)
	}

	s.logger.Info("Found top-level directories to analyze",
		zap.Int("count", len(dirs)),
		zap.Int64("storage_root_id", storageRootID))

	created, updated := 0, 0
	for _, dir := range dirs {
		isNew, err := s.processDirectory(ctx, dir, storageRootID)
		if err != nil {
			s.logger.Warn("Failed to process directory",
				zap.String("path", dir.path),
				zap.Error(err))
			continue
		}
		if isNew {
			created++
		} else {
			updated++
		}
	}

	s.logger.Info("Post-scan aggregation completed",
		zap.Int64("storage_root_id", storageRootID),
		zap.Int("entities_created", created),
		zap.Int("entities_updated", updated))

	// Auto-enrich new entities with TMDB metadata (cover art, descriptions)
	if created > 0 {
		go s.enrichNewEntities(ctx, storageRootID, created)
	}

	return nil
}

// Extension classification sets shared by the title-granular walk. Keys are
// lower-cased and dot-prefixed.
var (
	videoExtSet = map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true, ".m4v": true, ".ts": true}
	audioExtSet = map[string]bool{".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true, ".m4a": true, ".wma": true, ".ape": true}
	isoExtSet   = map[string]bool{".iso": true, ".img": true, ".bin": true, ".nrg": true}
	ebookExtSet = map[string]bool{".epub": true, ".mobi": true, ".azw3": true, ".pdf": true, ".djvu": true}
	comicExtSet = map[string]bool{".cbr": true, ".cbz": true, ".cb7": true}
	exeExtSet   = map[string]bool{".exe": true, ".msi": true, ".dmg": true, ".deb": true, ".rpm": true, ".appimage": true}
)

// leafFile is a single media-bearing (or sidecar) file discovered during the walk.
type leafFile struct {
	id        int64
	name      string // file name including extension (e.g. "Inception.2010.720p.BluRay.mp4")
	path      string // full path relative to the storage root (e.g. "movies/Inception.2010.720p.BluRay.mp4")
	ext       string // lower-cased, dot-prefixed extension (e.g. ".mp4")
	size      int64
	mediaType string // detected primary media type, or "" for a sidecar / unknown
}

// titleGroup is one prospective media entity: a set of files that all belong to
// the same parsed title.
type titleGroup struct {
	mediaType  string // "movie", "tv_show", "music_album", "software", "game", "book", "comic"
	titleKey   string // de-duplication key (lower-cased title, with year for movies)
	title      string
	parsed     ParsedTitle
	files      []leafFile // every file that links to this entity (media + sidecars)
	tvShowPath string     // for tv_show: the show directory path used to walk descendants
}

// getMediaLeafFiles returns every non-directory, non-deleted file under the
// storage root with the metadata the title walk needs. Unlike the legacy path,
// it does NOT depend on parent_id linkage being correct (§11.4.111
// resolve-by-content, not by a possibly-broken ordinal).
func (s *AggregationService) getMediaLeafFiles(ctx context.Context, storageRootID int64) ([]leafFile, error) {
	query := `SELECT id, name, path, extension, size FROM files
		WHERE storage_root_id = ? AND is_directory = 0 AND deleted = 0
		ORDER BY path`

	rows, err := s.db.QueryContext(ctx, query, storageRootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaves []leafFile
	for rows.Next() {
		var lf leafFile
		var ext *string
		if err := rows.Scan(&lf.id, &lf.name, &lf.path, &ext, &lf.size); err != nil {
			return nil, err
		}
		lf.ext = normalizeExt(ext, lf.name)
		lf.mediaType = classifyLeafMediaType(lf.ext)
		leaves = append(leaves, lf)
	}
	return leaves, rows.Err()
}

// normalizeExt returns a lower-cased, dot-prefixed extension, falling back to the
// file name's suffix when the stored extension column is empty/NULL.
func normalizeExt(ext *string, name string) string {
	e := ""
	if ext != nil {
		e = *ext
	}
	if e == "" {
		e = filepath.Ext(name)
	}
	e = strings.ToLower(e)
	if e != "" && !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	return e
}

// classifyLeafMediaType maps a file extension to a primary media type, or ""
// for sidecar / non-media files (e.g. ".srt", ".nfo", ".jpg").
func classifyLeafMediaType(ext string) string {
	switch {
	case videoExtSet[ext]:
		return "video"
	case audioExtSet[ext]:
		return "audio"
	case exeExtSet[ext] || isoExtSet[ext]:
		return "software"
	case comicExtSet[ext]:
		return "comic"
	case ebookExtSet[ext]:
		return "book"
	default:
		return ""
	}
}

// stripExt removes a trailing file extension from a file name so the title
// parsers operate on the bare title (e.g. "Artist - Song Title").
func stripExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// pathParts splits a file path into its components, dropping empties.
func pathParts(p string) []string {
	raw := strings.Split(p, "/")
	parts := raw[:0]
	for _, c := range raw {
		if c != "" {
			parts = append(parts, c)
		}
	}
	return parts
}

// groupLeafFilesByTitle is the heart of the DEFECT-E fix: it walks the leaf files
// and groups them into per-title entities by parsed title, never by the top-level
// directory name. Sidecar files (subtitles, art) link to the media entity sharing
// their directory.
func (s *AggregationService) groupLeafFilesByTitle(leaves []leafFile) []titleGroup {
	groups := make(map[string]*titleGroup)
	// dirEntityKey maps a directory path → the entity key of a media file living
	// in it, so sidecars in the same directory link to that entity.
	dirEntityKey := make(map[string]string)
	var sidecars []leafFile

	upsert := func(key string, base titleGroup, lf leafFile) {
		g, ok := groups[key]
		if !ok {
			base.titleKey = key
			groups[key] = &base
			g = groups[key]
		}
		g.files = append(g.files, lf)
		if dir := dirOf(lf.path); dir != "" {
			dirEntityKey[dir] = key
		}
	}

	for _, lf := range leaves {
		switch lf.mediaType {
		case "video":
			// Resolve the title from the BEST of {file name, dedicated per-title
			// folder name}. A messy file name ("episode.mkv", "Inception.2010")
			// inside a per-title folder ("Breaking Bad S01E01 720p",
			// "Inception (2010)") yields the canonical title/year/season from the
			// folder; a per-title file directly under a category folder
			// ("movies/Inception.2010.720p.mp4") yields it from the file name —
			// the category folder is never used.
			mediaType, parsed := s.resolveVideoTitle(lf)
			if parsed.Title == "" {
				continue
			}
			if mediaType == "tv_show" {
				key := "tv_show|" + strings.ToLower(parsed.Title)
				upsert(key, titleGroup{
					mediaType:  "tv_show",
					title:      parsed.Title,
					parsed:     parsed,
					tvShowPath: tvShowDirForFile(lf.path),
				}, lf)
				if g := groups[key]; g.tvShowPath == "" {
					g.tvShowPath = tvShowDirForFile(lf.path)
				}
				continue
			}
			upsert(movieKey(parsed), titleGroup{mediaType: "movie", title: parsed.Title, parsed: parsed}, lf)

		case "audio":
			parts := pathParts(lf.path)
			// An audio file nested in a dedicated album folder
			// (category/.../<album>/<file>, depth >= 3) keys on the album folder
			// name; a loose file directly under a category dir (category/<file>,
			// depth 2) keys per single from the parsed file name so the category
			// folder itself never becomes an album entity.
			if len(parts) >= 3 {
				albumFolder := parts[len(parts)-2]
				al := ParseMusicAlbum(albumFolder)
				if al.Title != "" {
					key := "music_album|" + strings.ToLower(al.Title)
					upsert(key, titleGroup{mediaType: "music_album", title: al.Title, parsed: al}, lf)
					break
				}
			}
			al := ParseMusicAlbum(stripExt(lf.name))
			if al.Title == "" {
				continue
			}
			key := "music_album|" + strings.ToLower(al.Title)
			upsert(key, titleGroup{mediaType: "music_album", title: al.Title, parsed: al}, lf)

		case "software":
			sw := ParseSoftwareTitle(stripExt(lf.name))
			if sw.Title == "" {
				continue
			}
			key := "software|" + strings.ToLower(sw.Title)
			upsert(key, titleGroup{mediaType: "software", title: sw.Title, parsed: sw}, lf)

		case "comic":
			c := ParsedTitle{Title: CleanTitle(stripExt(lf.name))}
			if c.Title == "" {
				continue
			}
			key := "comic|" + strings.ToLower(c.Title)
			upsert(key, titleGroup{mediaType: "comic", title: c.Title, parsed: c}, lf)

		case "book":
			b := ParsedTitle{Title: CleanTitle(stripExt(lf.name))}
			if b.Title == "" {
				continue
			}
			key := "book|" + strings.ToLower(b.Title)
			upsert(key, titleGroup{mediaType: "book", title: b.Title, parsed: b}, lf)

		default:
			// Sidecar / unknown — resolved after media files are grouped.
			sidecars = append(sidecars, lf)
		}
	}

	// Attach sidecars to the media entity sharing their directory, if any.
	for _, sc := range sidecars {
		if key, ok := dirEntityKey[dirOf(sc.path)]; ok {
			groups[key].files = append(groups[key].files, sc)
		}
	}

	result := make([]titleGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	return result
}

// categoryFolderNames are generic container directory names that MUST NOT be
// treated as a per-title folder (so "movies"/"series"/"music" never become
// entity titles). Matched case-insensitively against the whole folder name.
var categoryFolderNames = map[string]bool{
	"movies": true, "movie": true, "films": true, "film": true,
	"tv": true, "tv shows": true, "tvshows": true, "shows": true,
	"series": true, "tv series": true,
	"music": true, "audio": true, "songs": true, "albums": true,
	"software": true, "apps": true, "applications": true, "programs": true,
	"games": true, "game": true, "books": true, "book": true,
	"ebooks": true, "comics": true, "comic": true, "media": true,
	"video": true, "videos": true, "downloads": true,
}

// seasonContainerRe matches a "Season N" / "Disc N" style container folder that
// holds episode files but is NOT the per-title (show) folder.
var seasonContainerRe = regexp.MustCompile(`(?i)^(?:season|series|disc|disk|s)[\s._-]*\d{1,3}$`)

// isContainerFolder reports whether a folder name is a generic category or a
// season/disc container rather than a per-title folder.
func isContainerFolder(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if categoryFolderNames[n] {
		return true
	}
	return seasonContainerRe.MatchString(name)
}

// episodeSxxExxRe matches an "SxxExx" episode token anywhere in the name, with
// optional separators (S01E09, s1e9, S1.E09, "S01 E09"). The leading 's' must
// sit at a start/separator boundary AND be immediately followed by a digit, so
// movie titles like "Se7en" (no digit after the 's') never match.
var episodeSxxExxRe = regexp.MustCompile(`(?i)(?:^|[\s._-])s\d{1,2}[\s._-]*e\d{1,2}(?:\b|[\s._-]|$)`)

// episodeLeadingRe matches a LEADING "Episode N" token ("Episode 5 - ...").
// Anchored at the start so a real movie like "Star Wars Episode 1" (the token
// appears mid-title) is NOT reclassified.
var episodeLeadingRe = regexp.MustCompile(`(?i)^[\s._-]*episode[\s._-]*\d{1,3}\b`)

// episodeNxNNRe matches the "NxNN" season-x-episode notation (1x09, 2x05). The
// 2-3 digit episode part keeps it from matching resolutions like "1920x1080".
var episodeNxNNRe = regexp.MustCompile(`(?i)(?:^|[\s._-])\d{1,2}x\d{2,3}(?:\b|[\s._-]|$)`)

// looksLikeEpisodeName reports whether a directory/file name carries an
// unambiguous single-episode token (SxxExx / leading "Episode N" / NxNN).
//
// Conservative by design: it is consulted only AFTER ParseTVShow fails to find
// a show-name-prefixed season/episode, to rescue episodes whose name LEADS with
// the token (e.g. "S01E09 The Perfect Storm") and would otherwise leak onto the
// movie shelf. Real movies bearing plain numbers ("Ocean's Eleven", "2012",
// "Se7en", "Apollo 13") carry none of these tokens and stay classified as
// movies.
func looksLikeEpisodeName(name string) bool {
	return episodeSxxExxRe.MatchString(name) ||
		episodeLeadingRe.MatchString(name) ||
		episodeNxNNRe.MatchString(name)
}

// parseRichness ranks a parsed video title by how much structure it carries, so
// the richer of {file-name parse, folder-name parse} wins. TV (season+episode) >
// year-bearing movie > plain title.
func parseRichness(isTV bool, p ParsedTitle) int {
	switch {
	case isTV && p.Season != nil && p.Episode != nil:
		return 4
	case isTV && p.Season != nil:
		return 3
	case !isTV && p.Year != nil:
		return 2
	default:
		return 1
	}
}

// resolveVideoTitle picks the best title source for a video file: the file name,
// or — when the file lives in a dedicated (non-container) per-title folder — the
// folder name, choosing whichever yields the richer parse. Returns the media type
// ("movie" or "tv_show") and the parsed title.
func (s *AggregationService) resolveVideoTitle(lf leafFile) (string, ParsedTitle) {
	type candidate struct {
		mediaType string
		parsed    ParsedTitle
		rank      int
	}
	var best *candidate

	// winsOnTie lets the per-title FOLDER name displace an equal-rank FILE-name
	// parse, because a dedicated per-title folder ("Inception (2010)") is a
	// cleaner title source than a dotted file name ("Inception.2010" →
	// "Inception 2010"). A strictly richer parse always wins regardless.
	consider := func(raw string, winsOnTie bool) {
		if raw == "" {
			return
		}
		if tv := ParseTVShow(raw); (tv.Season != nil || tv.Episode != nil) && tv.Title != "" {
			c := candidate{mediaType: "tv_show", parsed: tv, rank: parseRichness(true, tv)}
			if best == nil || c.rank > best.rank || (winsOnTie && c.rank == best.rank) {
				best = &c
			}
			return
		}
		if mv := ParseMovieTitle(raw); mv.Title != "" {
			c := candidate{mediaType: "movie", parsed: mv, rank: parseRichness(false, mv)}
			if best == nil || c.rank > best.rank || (winsOnTie && c.rank == best.rank) {
				best = &c
			}
		}
	}

	consider(stripExt(lf.name), false)
	// The immediate parent folder is a candidate only when it is a real
	// per-title folder (not "movies"/"series"/"Season 01"/etc.).
	if parent := immediateParent(lf.path); parent != "" && !isContainerFolder(parent) {
		consider(parent, true)
	}

	if best == nil {
		return "", ParsedTitle{}
	}
	return best.mediaType, best.parsed
}

// immediateParent returns the name (not full path) of a file's immediate parent
// directory, or "" if it has none.
func immediateParent(p string) string {
	parts := pathParts(p)
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
}

// movieKey builds a de-dup key for a movie, including the year when known so two
// remakes with the same title do not collapse into one entity.
func movieKey(p ParsedTitle) string {
	key := "movie|" + strings.ToLower(p.Title)
	if p.Year != nil {
		key += fmt.Sprintf("|%d", *p.Year)
	}
	return key
}

// dirOf returns the directory portion of a path (everything before the last "/"),
// or "" if the path has no separator.
func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// tvShowDirForFile derives the show directory path for a TV episode file so
// buildTVHierarchyFromFiles can walk every descendant. For
// "series/Breaking Bad/Season 01/Breaking.Bad.S01E01.mkv" it returns
// "series/Breaking Bad" (the parent of a "Season"/"Series" container, when
// present), else the file's immediate directory.
func tvShowDirForFile(p string) string {
	parts := pathParts(p)
	if len(parts) == 0 {
		return ""
	}
	// Drop the file name.
	dirs := parts[:len(parts)-1]
	if len(dirs) == 0 {
		return ""
	}
	// If the immediate directory looks like a season/series container, climb one.
	last := strings.ToLower(dirs[len(dirs)-1])
	if len(dirs) >= 2 && (strings.HasPrefix(last, "season") || strings.HasPrefix(last, "series") || strings.HasPrefix(last, "s0") || strings.HasPrefix(last, "disc")) {
		return strings.Join(dirs[:len(dirs)-1], "/")
	}
	return strings.Join(dirs, "/")
}

// processTitleGroup creates/updates a single media entity from a parsed title
// group and links every file in the group to it. Returns true if a new entity
// was created.
func (s *AggregationService) processTitleGroup(ctx context.Context, g titleGroup, storageRootID int64) (bool, error) {
	_, typeID, err := s.itemRepo.GetMediaTypeByName(ctx, g.mediaType)
	if err != nil {
		return false, fmt.Errorf("get media type %q: %w", g.mediaType, err)
	}

	existing, err := s.itemRepo.GetByTitle(ctx, g.title, typeID)
	if err != nil {
		return false, err
	}

	var itemID int64
	isNew := false
	if existing != nil {
		itemID = existing.ID
		if existing.Year == nil && g.parsed.Year != nil {
			existing.Year = g.parsed.Year
			if err := s.itemRepo.Update(ctx, existing); err != nil {
				s.logger.Warn("Failed to update entity year",
					zap.Int64("media_item_id", itemID), zap.Error(err))
			}
		}
	} else {
		item := &models.MediaItem{
			MediaTypeID: typeID,
			Title:       g.title,
			Year:        g.parsed.Year,
			Status:      "detected",
		}
		itemID, err = s.itemRepo.Create(ctx, item)
		if err != nil {
			return false, fmt.Errorf("create media item: %w", err)
		}
		isNew = true
	}

	// Link every file in the group to the entity.
	for i, lf := range g.files {
		isPrimary := i == 0
		if _, err := s.fileRepo.LinkFileToItem(ctx, itemID, lf.id, nil, nil, isPrimary); err != nil {
			s.logger.Warn("Failed to link file to entity",
				zap.Int64("file_id", lf.id),
				zap.Int64("media_item_id", itemID),
				zap.Error(err))
		}
	}

	// Record directory analysis against the entity's representative directory.
	if dir := s.groupRepresentativeDir(g); dir != "" {
		confidence := 0.5
		if g.parsed.Year != nil {
			confidence = 0.8
		}
		if len(g.parsed.QualityHints) > 0 {
			confidence = 0.9
		}
		var totalSize int64
		for _, lf := range g.files {
			totalSize += lf.size
		}
		da := &models.DirectoryAnalysis{
			DirectoryPath:   dir,
			MediaItemID:     &itemID,
			ConfidenceScore: confidence,
			DetectionMethod: "title_parser",
			FilesCount:      len(g.files),
			TotalSize:       totalSize,
		}
		if existingDA, _ := s.dirAnalysisRepo.GetByPath(ctx, dir); existingDA != nil {
			da.ID = existingDA.ID
			_ = s.dirAnalysisRepo.Update(ctx, da)
		} else {
			_, _ = s.dirAnalysisRepo.Create(ctx, da)
		}
	}

	// Build the season/episode hierarchy for TV shows from every descendant file.
	if g.mediaType == "tv_show" {
		if g.parsed.Season != nil {
			s.buildTVHierarchy(ctx, itemID, typeID, g.parsed)
		}
		if g.tvShowPath != "" {
			if err := s.buildTVHierarchyFromFiles(ctx, itemID, typeID, storageRootID, g.tvShowPath); err != nil {
				s.logger.Warn("Failed to build tv hierarchy from files",
					zap.Int64("show_id", itemID),
					zap.String("dir", g.tvShowPath),
					zap.Error(err))
			}
		}
	}

	return isNew, nil
}

// groupRepresentativeDir returns a representative directory path for a group for
// directory-analysis bookkeeping: the tv show dir for TV, else the directory of
// the group's first file.
func (s *AggregationService) groupRepresentativeDir(g titleGroup) string {
	if g.mediaType == "tv_show" && g.tvShowPath != "" {
		return g.tvShowPath
	}
	if len(g.files) > 0 {
		return dirOf(g.files[0].path)
	}
	return ""
}

// enrichNewEntities fetches TMDB metadata for entities that lack external metadata.
// Runs asynchronously after aggregation to avoid blocking the scan completion.
func (s *AggregationService) enrichNewEntities(ctx context.Context, storageRootID int64, count int) {
	s.logger.Info("Starting automatic metadata enrichment",
		zap.Int64("storage_root_id", storageRootID),
		zap.Int("new_entities", count))

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		s.logger.Warn("TMDB_API_KEY not set, skipping metadata enrichment")
		return
	}

	// Find entities without external metadata
	rows, err := s.db.QueryContext(ctx,
		`SELECT mi.id, mi.title, COALESCE(mi.year, 0), mt.name
		 FROM media_items mi
		 JOIN media_types mt ON mt.id = mi.media_type_id
		 LEFT JOIN external_metadata em ON em.media_item_id = mi.id
		 WHERE em.id IS NULL
		 LIMIT 200`)
	if err != nil {
		s.logger.Error("Failed to query unenriched entities", zap.Error(err))
		return
	}
	defer rows.Close()

	type entity struct {
		id        int64
		title     string
		year      int
		mediaType string
	}
	var entities []entity
	for rows.Next() {
		var e entity
		if err := rows.Scan(&e.id, &e.title, &e.year, &e.mediaType); err != nil {
			continue
		}
		entities = append(entities, e)
	}

	if len(entities) == 0 {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	enriched := 0

	for _, e := range entities {
		searchType := "movie"
		if e.mediaType == "tv_show" || e.mediaType == "tv_season" || e.mediaType == "tv_episode" {
			searchType = "tv"
		}
		if e.mediaType == "music_artist" || e.mediaType == "music_album" || e.mediaType == "song" {
			continue // Skip music — MusicBrainz handles this
		}

		// Build TMDB search URL. Skip obviously wrong years (> current year
		// or < 1888, the year of the first film) to avoid empty results.
		yearParam := e.year
		currentYear := time.Now().Year()
		if yearParam > currentYear || yearParam < 1888 {
			yearParam = 0
		}

		searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
			searchType, apiKey, url.QueryEscape(e.title))
		if yearParam > 0 {
			searchURL += fmt.Sprintf("&year=%d", yearParam)
		}

		var searchResp struct {
			Results []struct {
				ID          int     `json:"id"`
				PosterPath  string  `json:"poster_path"`
				Overview    string  `json:"overview"`
				VoteAverage float64 `json:"vote_average"`
			} `json:"results"`
		}

		// Try search with year first, retry without year if no results
		for attempt := 0; attempt < 2; attempt++ {
			reqURL := searchURL
			if attempt == 1 {
				// Retry without year
				reqURL = fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
					searchType, apiKey, url.QueryEscape(e.title))
			}

			resp, err := client.Get(reqURL) // #nosec G704 — hardcoded TMDB API URL, not user-controlled
			if err != nil || resp.StatusCode != 200 {
				if resp != nil {
					resp.Body.Close()
				}
				break
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				break
			}

			if err := json.Unmarshal(body, &searchResp); err != nil {
				break
			}
			if len(searchResp.Results) > 0 {
				break // Found results
			}
			// No results with year — retry without
			time.Sleep(300 * time.Millisecond)
		}
		if len(searchResp.Results) == 0 {
			continue
		}

		r := searchResp.Results[0]
		if r.PosterPath == "" {
			continue
		}

		coverURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", r.PosterPath)
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO external_metadata (media_item_id, provider, external_id, data, rating, cover_url, last_fetched)
			 VALUES (?, 'tmdb', ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			e.id, fmt.Sprintf("tmdb:%d", r.ID), r.Overview, r.VoteAverage, coverURL)
		if err != nil {
			s.logger.Warn("Failed to insert TMDB metadata", zap.Int64("entity", e.id), zap.Error(err))
			continue
		}

		// Update entity description and rating
		if r.Overview != "" {
			s.db.ExecContext(ctx,
				`UPDATE media_items SET description = ? WHERE id = ? AND (description IS NULL OR description = '')`,
				r.Overview, e.id)
		}
		if r.VoteAverage > 0 {
			s.db.ExecContext(ctx,
				`UPDATE media_items SET rating = ? WHERE id = ? AND (rating IS NULL OR rating = 0)`,
				r.VoteAverage, e.id)
		}

		enriched++
		// TMDB rate limit: 40 requests per 10 seconds
		time.Sleep(300 * time.Millisecond)
	}

	s.logger.Info("Automatic metadata enrichment completed",
		zap.Int("enriched", enriched),
		zap.Int("total", len(entities)))
}

type directoryInfo struct {
	path       string
	name       string
	fileCount  int
	totalSize  int64
	fileIDs    []int64
	fileTypes  map[string]int
	extensions []string
}

// getTopLevelDirectories returns top-level directories and their file stats.
func (s *AggregationService) getTopLevelDirectories(ctx context.Context, storageRootID int64) ([]directoryInfo, error) {
	// Get directories (parent_id IS NULL means top-level)
	query := `SELECT id, path, name FROM files
		WHERE storage_root_id = ? AND is_directory = 1 AND deleted = 0 AND parent_id IS NULL
		ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query, storageRootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Buffer the directory rows and release the cursor (and its pooled connection) BEFORE the
	// per-directory child query: a query issued while this cursor is open needs a second pooled
	// connection, so concurrent calls exceeding the pool would all hold one and wait for another.
	type topDir struct {
		id  int64
		dir directoryInfo
	}
	var top []topDir
	for rows.Next() {
		var td topDir
		if err := rows.Scan(&td.id, &td.dir.path, &td.dir.name); err != nil {
			return nil, err
		}
		top = append(top, td)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	var dirs []directoryInfo
	for _, td := range top {
		id, dir := td.id, td.dir

		// Get child files for this directory
		childQuery := `SELECT id, extension, size FROM files
			WHERE storage_root_id = ? AND parent_id = ? AND is_directory = 0 AND deleted = 0`

		childRows, err := s.db.QueryContext(ctx, childQuery, storageRootID, id)
		if err != nil {
			continue
		}

		dir.fileTypes = make(map[string]int)
		func() {
			defer childRows.Close()
			for childRows.Next() {
				var fileID, size int64
				var ext *string
				if err := childRows.Scan(&fileID, &ext, &size); err != nil {
					continue
				}
				dir.fileIDs = append(dir.fileIDs, fileID)
				dir.totalSize += size
				dir.fileCount++
				if ext != nil {
					dir.fileTypes[*ext]++
					dir.extensions = append(dir.extensions, *ext)
				}
			}
		}()

		if dir.fileCount > 0 {
			dirs = append(dirs, dir)
		}
	}

	return dirs, nil
}

// processDirectory analyzes a directory and creates/updates a media entity.
// Returns true if a new entity was created.
func (s *AggregationService) processDirectory(ctx context.Context, dir directoryInfo, storageRootID int64) (bool, error) {
	// Detect media type from directory name and file types
	mediaTypeName, parsed := s.detectMediaType(dir)
	if mediaTypeName == "" {
		return false, nil // Couldn't determine type
	}

	// Get media type ID
	_, typeID, err := s.itemRepo.GetMediaTypeByName(ctx, mediaTypeName)
	if err != nil {
		return false, fmt.Errorf("get media type %q: %w", mediaTypeName, err)
	}

	// Check if entity already exists
	existing, err := s.itemRepo.GetByTitle(ctx, parsed.Title, typeID)
	if err != nil {
		return false, err
	}

	var itemID int64
	isNew := false

	if existing != nil {
		itemID = existing.ID
		// Update if needed
		if existing.Year == nil && parsed.Year != nil {
			existing.Year = parsed.Year
			if err := s.itemRepo.Update(ctx, existing); err != nil {
				// Log but continue - not critical for aggregation
			}
		}
	} else {
		// Create new entity
		item := &models.MediaItem{
			MediaTypeID: typeID,
			Title:       parsed.Title,
			Year:        parsed.Year,
			Status:      "detected",
		}
		itemID, err = s.itemRepo.Create(ctx, item)
		if err != nil {
			return false, fmt.Errorf("create media item: %w", err)
		}
		isNew = true
	}

	// Link files to entity
	for i, fileID := range dir.fileIDs {
		isPrimary := i == 0 // First file is primary
		_, err := s.fileRepo.LinkFileToItem(ctx, itemID, fileID, nil, nil, isPrimary)
		if err != nil {
			s.logger.Warn("Failed to link file to entity",
				zap.Int64("file_id", fileID),
				zap.Int64("media_item_id", itemID),
				zap.Error(err))
		}
	}

	// Store directory analysis
	confidence := 0.5
	if parsed.Year != nil {
		confidence = 0.8
	}
	if len(parsed.QualityHints) > 0 {
		confidence = 0.9
	}

	da := &models.DirectoryAnalysis{
		DirectoryPath:   dir.path,
		MediaItemID:     &itemID,
		ConfidenceScore: confidence,
		DetectionMethod: "title_parser",
		FilesCount:      dir.fileCount,
		TotalSize:       dir.totalSize,
	}

	existingDA, _ := s.dirAnalysisRepo.GetByPath(ctx, dir.path)
	if existingDA != nil {
		da.ID = existingDA.ID
		if err := s.dirAnalysisRepo.Update(ctx, da); err != nil {
			// Log but continue - directory analysis is informational
		}
	} else {
		if _, err := s.dirAnalysisRepo.Create(ctx, da); err != nil {
			// Log but continue - directory analysis is informational
		}
	}

	// Build hierarchy for TV shows. Two paths:
	//
	//  (a) The top-level directory name already has season/
	//      episode info (e.g. "Asterix Season 1 S01E01"). Use
	//      the parser output from detectMediaType.
	//
	//  (b) More common: the dir name is just the show title
	//      (e.g. "Asterix") and the season/episode data lives
	//      in the file names underneath. Walk every
	//      descendant file once and call buildTVHierarchy for
	//      each (show, season, episode) triple the parser can
	//      extract. Without this path the entity system ends
	//      up with tv_show rows but zero tv_season / tv_episode
	//      rows — exactly what the 2026-04-11 audit surfaced.
	if mediaTypeName == "tv_show" {
		if parsed.Season != nil {
			s.buildTVHierarchy(ctx, itemID, typeID, parsed)
		}
		if err := s.buildTVHierarchyFromFiles(ctx, itemID, typeID, storageRootID, dir.path); err != nil {
			s.logger.Warn("Failed to build tv hierarchy from files",
				zap.Int64("show_id", itemID),
				zap.String("dir", dir.path),
				zap.Error(err))
		}
	}

	return isNew, nil
}

// buildTVHierarchyFromFiles walks every descendant file of a
// tv_show directory, parses each filename with ParseTVShow, and
// upserts a season + episode row for every distinct (season,
// episode) pair. Uses ParseTVShow's existing SxxEyy / NxNN /
// "Season N / Episode M" regexes so all three scan patterns we
// see on our Synology NAS are handled.
func (s *AggregationService) buildTVHierarchyFromFiles(
	ctx context.Context,
	showID, showTypeID, storageRootID int64,
	showDirPath string,
) error {
	query := `SELECT id, name FROM files
		WHERE storage_root_id = ?
		  AND is_directory = 0
		  AND deleted = 0
		  AND path LIKE ?`
	rows, err := s.db.QueryContext(ctx, query, storageRootID, showDirPath+"/%")
	if err != nil {
		return fmt.Errorf("list descendant files: %w", err)
	}
	defer rows.Close()

	type fileRow struct {
		id   int64
		name string
	}
	var files []fileRow
	for rows.Next() {
		var f fileRow
		if err := rows.Scan(&f.id, &f.name); err != nil {
			continue
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate descendant files: %w", err)
	}

	seenEpisode := make(map[[2]int]int64)
	for _, f := range files {
		p := ParseTVShow(f.name)
		if p.Season == nil || p.Episode == nil {
			continue
		}
		key := [2]int{*p.Season, *p.Episode}
		if _, already := seenEpisode[key]; already {
			continue
		}
		s.buildTVHierarchy(ctx, showID, showTypeID, p)
		seenEpisode[key] = f.id
	}
	return nil
}

// detectMediaType determines the media type from directory info and filename.
func (s *AggregationService) detectMediaType(dir directoryInfo) (string, ParsedTitle) {
	name := dir.name

	// Check file extensions to help classify
	hasVideo := false
	hasAudio := false
	hasISO := false
	hasEbook := false
	hasComic := false
	hasExecutable := false

	videoExts := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true, ".m4v": true, ".ts": true}
	audioExts := map[string]bool{".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true, ".m4a": true, ".wma": true, ".ape": true}
	isoExts := map[string]bool{".iso": true, ".img": true, ".bin": true, ".nrg": true}
	ebookExts := map[string]bool{".epub": true, ".mobi": true, ".azw3": true, ".pdf": true, ".djvu": true}
	comicExts := map[string]bool{".cbr": true, ".cbz": true, ".cb7": true}
	exeExts := map[string]bool{".exe": true, ".msi": true, ".dmg": true, ".deb": true, ".rpm": true, ".AppImage": true}

	for ext, count := range dir.fileTypes {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if videoExts[ext] && count > 0 {
			hasVideo = true
		}
		if audioExts[ext] && count > 0 {
			hasAudio = true
		}
		if isoExts[ext] && count > 0 {
			hasISO = true
		}
		if ebookExts[ext] && count > 0 {
			hasEbook = true
		}
		if comicExts[ext] && count > 0 {
			hasComic = true
		}
		if exeExts[ext] && count > 0 {
			hasExecutable = true
		}
	}

	// TV show detection first (specific pattern). ParseTVShow requires a
	// show-name prefix BEFORE the SxxExx token, so "Breaking Bad S01E01" parses
	// (Season/Episode set) and aggregates under the show entity.
	tvParsed := ParseTVShow(name)
	if tvParsed.Season != nil || tvParsed.Episode != nil {
		return "tv_show", tvParsed
	}

	// Un-prefixed / LEADING episode tokens that ParseTVShow does not catch —
	// e.g. "S01E09 The Perfect Storm", "S2E02 - The Awful Truth",
	// "Episode 5 - Kissed by Fire", "S03E12 The Lady Killer", "1x09 ...".
	// These are individual TV episodes that previously leaked onto the movie
	// shelf (and could never match TMDB as movies); classify them as
	// tv_episode. Checked before the coarse substring fallback below so an
	// episode is never mis-bucketed as a whole tv_show.
	if looksLikeEpisodeName(name) {
		epParsed := ParseTVShow(name)
		if epParsed.Title == "" {
			epParsed = ParsedTitle{Title: CleanTitle(name)}
		}
		return "tv_episode", epParsed
	}

	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "season") || strings.Contains(nameLower, "complete") ||
		strings.Contains(nameLower, "s01") || strings.Contains(nameLower, "s02") {
		return "tv_show", ParseTVShow(name)
	}

	// Comic detection
	if hasComic {
		return "comic", ParsedTitle{Title: CleanTitle(name)}
	}

	// Ebook detection
	if hasEbook && !hasVideo && !hasAudio {
		return "book", ParsedTitle{Title: CleanTitle(name)}
	}

	// Music detection
	if hasAudio && !hasVideo {
		return "music_album", ParseMusicAlbum(name)
	}

	// Software detection
	if hasExecutable || hasISO {
		return "software", ParseSoftwareTitle(name)
	}

	// Game detection (ISO with game-like names, or directory names with platform)
	if hasISO && gamePlatformRe.MatchString(name) {
		return "game", ParseGameTitle(name)
	}

	// Default: video content = movie
	if hasVideo {
		return "movie", ParseMovieTitle(name)
	}

	// Try to parse as movie from name alone
	parsed := ParseMovieTitle(name)
	if parsed.Year != nil {
		return "movie", parsed
	}

	return "", ParsedTitle{}
}

// buildTVHierarchy creates season and episode entities under a TV show.
func (s *AggregationService) buildTVHierarchy(ctx context.Context, showID, showTypeID int64, parsed ParsedTitle) {
	// Get season type
	_, seasonTypeID, err := s.itemRepo.GetMediaTypeByName(ctx, "tv_season")
	if err != nil {
		return
	}

	if parsed.Season != nil {
		seasonTitle := fmt.Sprintf("Season %d", *parsed.Season)
		existingSeason, _ := s.itemRepo.GetByTitle(ctx, seasonTitle, seasonTypeID)

		var seasonID int64
		if existingSeason != nil && existingSeason.ParentID != nil && *existingSeason.ParentID == showID {
			seasonID = existingSeason.ID
		} else {
			season := &models.MediaItem{
				MediaTypeID:  seasonTypeID,
				Title:        seasonTitle,
				Status:       "detected",
				ParentID:     &showID,
				SeasonNumber: parsed.Season,
			}
			seasonID, err = s.itemRepo.Create(ctx, season)
			if err != nil {
				return
			}
		}

		// Create episode if we have one.
		//
		// Idempotency (§11.4.115 DEFECT-H): the episode-build path is
		// invoked twice for the same (season, episode) — once via the
		// representative-file g.parsed path and once via
		// buildTVHierarchyFromFiles, whose internal seenEpisode map is
		// blind to the representative-file insert. Without a guard this
		// produced two tv_episode rows for one episode. We dedup on the
		// STABLE key (parent_id=seasonID, episode_number) — NOT title,
		// because episode titles are mutable (DEFECT-I rewrites them, so
		// title is not a stable key). If an episode already exists under
		// this season with the same episode number, reuse/update it
		// instead of inserting a duplicate.
		if parsed.Episode != nil {
			_, epTypeID, err := s.itemRepo.GetMediaTypeByName(ctx, "tv_episode")
			if err != nil {
				return
			}

			// DEFECT-I Layer B (§11.4.108 SOURCE->USER-VISIBLE):
			// use the real human episode title parsed by
			// digital.vasic.entities/pkg/parser into ParsedTitle.EpisodeTitle
			// (Layer A populates it, e.g. "Breaking Bad S01E01.Pilot.mkv" ->
			// EpisodeTitle="Pilot"). The hardcoded "Episode %d" discarded the
			// real title so "Pilot" displayed as "Episode 1". Fall back to the
			// "Episode %d" placeholder ONLY when the filename carries no human
			// title (EpisodeTitle empty). The dedup key remains the STABLE
			// (parent_id, episode_number) pair (DEFECT-H) — the title is
			// display-only and never participates in identity.
			epTitle := fmt.Sprintf("Episode %d", *parsed.Episode)
			if parsed.EpisodeTitle != "" {
				epTitle = parsed.EpisodeTitle
			}
			ep := &models.MediaItem{
				MediaTypeID:   epTypeID,
				Title:         epTitle,
				Status:        "detected",
				ParentID:      &seasonID,
				SeasonNumber:  parsed.Season,
				EpisodeNumber: parsed.Episode,
			}

			if existingEp := s.findExistingEpisode(ctx, seasonID, *parsed.Episode); existingEp != nil {
				ep.ID = existingEp.ID
				_ = s.itemRepo.Update(ctx, ep)
			} else {
				_, _ = s.itemRepo.Create(ctx, ep)
			}
		}
	}
}

// findExistingEpisode returns the existing tv_episode child of seasonID whose
// EpisodeNumber equals episodeNumber, or nil if none. The lookup is keyed on
// (parent_id, episode_number) — the stable identity of an episode — rather than
// on title, which is mutable. This is the single dedup guard both episode-build
// invocation paths (the representative-file g.parsed path and
// buildTVHierarchyFromFiles) converge on (§11.4.115 DEFECT-H).
func (s *AggregationService) findExistingEpisode(ctx context.Context, seasonID int64, episodeNumber int) *models.MediaItem {
	children, err := s.itemRepo.GetChildren(ctx, seasonID)
	if err != nil {
		return nil
	}
	for _, child := range children {
		if child.EpisodeNumber != nil && *child.EpisodeNumber == episodeNumber {
			return child
		}
	}
	return nil
}

// GetStorageRootName returns the storage root name for display.
func (s *AggregationService) getStorageRootName(ctx context.Context, storageRootID int64) string {
	var name string
	_ = s.db.QueryRowContext(ctx, "SELECT name FROM storage_roots WHERE id = ?", storageRootID).Scan(&name)
	return name
}

// DetectMediaTypeFromPath analyzes a file path and returns the likely media type.
func DetectMediaTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".m4v":
		return "video"
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".wma":
		return "audio"
	case ".iso", ".img", ".bin":
		return "disc_image"
	case ".epub", ".mobi", ".azw3":
		return "ebook"
	case ".cbr", ".cbz", ".cb7":
		return "comic"
	case ".exe", ".msi", ".dmg", ".deb", ".rpm":
		return "software"
	default:
		return "other"
	}
}
