package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// QualityBucket maps the stored quality_info JSON to a resolution bucket.
// Anything without a parsable resolution is "Other": the bucket is derived
// from data, never assumed. withDVD adds a "DVD" bucket (height 480..719);
// without it those files count as "Other".
func QualityBucket(qualityInfoJSON string, withDVD bool) string {
	var q QualityInfo
	if qualityInfoJSON == "" || json.Unmarshal([]byte(qualityInfoJSON), &q) != nil || q.Resolution == nil {
		return "Other"
	}
	w, h := q.Resolution.Width, q.Resolution.Height
	switch {
	case h >= 2160 || w >= 3840:
		return "4K/UHD"
	case h >= 1080 || w >= 1920:
		return "1080p"
	case h >= 720 || w >= 1280:
		return "720p"
	case withDVD && h >= 480:
		return "DVD"
	}
	return "Other"
}

// QualityDistribution counts media_files rows per resolution bucket by
// reading the real quality_info column. All buckets are always present.
func QualityDistribution(db *sql.DB, withDVD bool) (map[string]int, error) {
	dist := map[string]int{"4K/UHD": 0, "1080p": 0, "720p": 0, "Other": 0}
	if withDVD {
		dist["DVD"] = 0
	}
	rows, err := db.Query("SELECT COALESCE(quality_info, '') FROM media_files")
	if err != nil {
		return nil, fmt.Errorf("query media_files quality_info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var qi string
		if err := rows.Scan(&qi); err != nil {
			return nil, fmt.Errorf("scan quality_info: %w", err)
		}
		dist[QualityBucket(qi, withDVD)]++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quality_info: %w", err)
	}
	return dist, nil
}
