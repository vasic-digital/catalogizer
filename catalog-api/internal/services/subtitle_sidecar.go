package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"catalogizer/database"

	"go.uber.org/zap"
)

// D11b: a scan used to ignore sidecar subtitle files (Movie.2020.en.srt next to Movie.2020.mp4).
// After a successful scan of a LOCAL storage root, ImportSidecarSubtitles turns every sidecar
// that sits in the same directory as a video and whose base name starts with the video's base
// name into a subtitle_tracks row (source "sidecar", path = the catalog path of the sidecar).
//
// Rules:
//   - local roots only (a remote root is never read from the host filesystem), and every file is
//     read only after it resolved, with symlinks followed, to a regular file inside the resolved
//     storage root (the confinement of resolveMediaFile);
//   - the bytes must parse as a subtitle of the format named by the extension (the same
//     validation as an upload), otherwise the file is skipped;
//   - idempotent: a row is keyed by (media item, sidecar path); a rescan inserts nothing new and
//     only refreshes the stored content when the file changed;
//   - the language comes from a dot-separated tag between the video base name and the
//     extension (Movie.en.srt -> en); no recognisable tag means "und" (undetermined);
//   - a failure on one file never fails the scan; the caller only logs the summary.
//
// Honest limits: only UTF-8 (BOM allowed) sidecars are imported (other encodings are skipped and
// counted); a sidecar is matched to the video with the LONGEST base name that prefixes it; sidecars
// whose video is not in the same directory are not imported; removed sidecars are not deleted from
// subtitle_tracks.
const (
	sidecarSource   = "sidecar"
	sidecarMaxBytes = 8 << 20
)

// SidecarImportResult summarises one import run.
type SidecarImportResult struct {
	Imported, Refreshed, Unchanged, Skipped int
}

var sidecarFormats = map[string]string{"srt": "srt", "vtt": "vtt", "ass": "ass", "ssa": "ssa"}

// sidecarLanguages maps ISO 639-1 codes (and common 639-2 spellings) to a display name.
var sidecarLanguages = map[string][2]string{
	"en": {"en", "English"}, "eng": {"en", "English"},
	"es": {"es", "Spanish"}, "spa": {"es", "Spanish"},
	"fr": {"fr", "French"}, "fre": {"fr", "French"}, "fra": {"fr", "French"},
	"de": {"de", "German"}, "ger": {"de", "German"}, "deu": {"de", "German"},
	"it": {"it", "Italian"}, "ita": {"it", "Italian"},
	"pt": {"pt", "Portuguese"}, "por": {"pt", "Portuguese"},
	"ru": {"ru", "Russian"}, "rus": {"ru", "Russian"},
	"ja": {"ja", "Japanese"}, "jpn": {"ja", "Japanese"},
	"ko": {"ko", "Korean"}, "kor": {"ko", "Korean"},
	"zh": {"zh", "Chinese"}, "chi": {"zh", "Chinese"}, "zho": {"zh", "Chinese"},
	"ar": {"ar", "Arabic"}, "ara": {"ar", "Arabic"},
	"hi": {"hi", "Hindi"}, "hin": {"hi", "Hindi"},
	"nl": {"nl", "Dutch"}, "dut": {"nl", "Dutch"}, "nld": {"nl", "Dutch"},
	"sv": {"sv", "Swedish"}, "swe": {"sv", "Swedish"},
	"no": {"no", "Norwegian"}, "nor": {"no", "Norwegian"},
	"da": {"da", "Danish"}, "dan": {"da", "Danish"},
	"fi": {"fi", "Finnish"}, "fin": {"fi", "Finnish"},
	"pl": {"pl", "Polish"}, "pol": {"pl", "Polish"},
	"tr": {"tr", "Turkish"}, "tur": {"tr", "Turkish"},
}

// sidecarTags splits the part of a sidecar base name that follows the video base name into the
// language (code, name) and the forced flag.
func sidecarTags(tag string) (code, name string, forced bool) {
	code, name = "und", "Unknown"
	found := false
	for _, p := range strings.Split(strings.ToLower(tag), ".") {
		if p == "forced" {
			forced = true
			continue
		}
		if l, ok := sidecarLanguages[p]; ok && !found {
			code, name, found = l[0], l[1], true
		}
	}
	return
}

type sidecarMedia struct {
	id   int64
	stem string
}

// confinedRegularFile resolves a catalog path under the storage root; ok only for a regular file
// whose fully resolved path lies inside the fully resolved root.
func confinedRegularFile(rootPath, catalogPath string) (string, bool) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", false
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", false
	}
	real, err := filepath.EvalSymlinks(filepath.Join(absRoot, filepath.FromSlash(catalogPath)))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	if st, err := os.Stat(real); err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	return real, true
}

// ImportSidecarSubtitles imports the sidecar subtitles of one local storage root.
func ImportSidecarSubtitles(ctx context.Context, db *database.DB, logger *zap.Logger, storageRootID int64, rootPath string) (SidecarImportResult, error) {
	var res SidecarImportResult
	if db == nil || rootPath == "" {
		return res, nil
	}
	svc := &SubtitleService{db: db, logger: logger}

	videos := map[string][]sidecarMedia{} // directory -> videos
	rows, err := db.QueryContext(ctx,
		`SELECT id, path, name FROM files WHERE storage_root_id = ? AND is_directory = ? AND (deleted IS NULL OR deleted = ?) AND file_type = 'video'`,
		storageRootID, false, false)
	if err != nil {
		return res, fmt.Errorf("list videos: %w", err)
	}
	for rows.Next() {
		var id int64
		var p, name string
		if err := rows.Scan(&id, &p, &name); err != nil {
			rows.Close()
			return res, fmt.Errorf("scan video row: %w", err)
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		dir := pathpkg.Dir(p)
		videos[dir] = append(videos[dir], sidecarMedia{id: id, stem: stem})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, err
	}
	rows.Close()
	if len(videos) == 0 {
		return res, nil
	}

	srows, err := db.QueryContext(ctx,
		`SELECT path, name, extension FROM files WHERE storage_root_id = ? AND is_directory = ? AND (deleted IS NULL OR deleted = ?)
		 AND LOWER(extension) IN ('srt','vtt','ass','ssa')`, storageRootID, false, false)
	if err != nil {
		return res, fmt.Errorf("list sidecars: %w", err)
	}
	type sidecar struct{ path, name, ext string }
	var cands []sidecar
	for srows.Next() {
		var s sidecar
		var ext sql.NullString
		if err := srows.Scan(&s.path, &s.name, &ext); err != nil {
			srows.Close()
			return res, fmt.Errorf("scan sidecar row: %w", err)
		}
		s.ext = strings.ToLower(ext.String)
		cands = append(cands, s)
	}
	if err := srows.Err(); err != nil {
		srows.Close()
		return res, err
	}
	srows.Close()

	for _, sc := range cands {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		format, ok := sidecarFormats[sc.ext]
		if !ok {
			continue
		}
		base := strings.TrimSuffix(sc.name, filepath.Ext(sc.name))
		var best *sidecarMedia
		for i, m := range videos[pathpkg.Dir(sc.path)] {
			if (base == m.stem || strings.HasPrefix(base, m.stem+".")) && (best == nil || len(m.stem) > len(best.stem)) {
				best = &videos[pathpkg.Dir(sc.path)][i]
			}
		}
		if best == nil {
			continue // not a sidecar of any video here: not counted as skipped
		}
		real, ok := confinedRegularFile(rootPath, sc.path)
		if !ok {
			res.Skipped++
			continue
		}
		content, ok := readSidecar(real)
		if !ok {
			res.Skipped++
			continue
		}
		if _, perr := svc.parseCues(content, format); perr != nil {
			res.Skipped++
			continue
		}
		state, err := svc.upsertSidecar(ctx, best.id, sc.path, format, base, best.stem, content)
		if err != nil {
			if logger != nil {
				logger.Warn("sidecar subtitle import failed", zap.Int64("media_id", best.id), zap.Error(err))
			}
			res.Skipped++
			continue
		}
		switch state {
		case "imported":
			res.Imported++
		case "refreshed":
			res.Refreshed++
		default:
			res.Unchanged++
		}
	}
	return res, nil
}

// readSidecar reads a bounded UTF-8 text file; ok=false for oversize or non-UTF-8 content.
func readSidecar(real string) (string, bool) {
	f, err := os.Open(real)
	if err != nil {
		return "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, sidecarMaxBytes+1))
	if err != nil || len(b) > sidecarMaxBytes {
		return "", false
	}
	s := strings.TrimPrefix(string(b), "\xef\xbb\xbf")
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return "", false
	}
	return s, true
}

// upsertSidecar inserts the track, or refreshes its content when the file changed.
func (s *SubtitleService) upsertSidecar(ctx context.Context, mediaID int64, catalogPath, format, base, stem, content string) (string, error) {
	var id int64
	var old sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, content FROM subtitle_tracks WHERE media_item_id = ? AND source = ? AND path = ?`,
		mediaID, sidecarSource, catalogPath).Scan(&id, &old)
	switch {
	case err == nil:
		if old.Valid && old.String == content {
			return "unchanged", nil
		}
		if _, uerr := s.db.ExecContext(ctx, `UPDATE subtitle_tracks SET content = ?, updated_at = ? WHERE id = ?`, content, time.Now(), id); uerr != nil {
			return "", uerr
		}
		return "refreshed", nil
	case err != sql.ErrNoRows:
		return "", err
	}
	code, name, forced := sidecarTags(strings.TrimPrefix(strings.TrimPrefix(base, stem), "."))
	cp := catalogPath
	c := content
	track := &SubtitleTrack{
		Language: name, LanguageCode: code, Source: sidecarSource, Format: format,
		Content: &c, Path: &cp, IsForced: forced, Encoding: "utf-8", CreatedAt: time.Now(),
	}
	if _, err := s.saveSubtitleTrackOwned(ctx, mediaID, track, nil); err != nil {
		return "", err
	}
	return "imported", nil
}
