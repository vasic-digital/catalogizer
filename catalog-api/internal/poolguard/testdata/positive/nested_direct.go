package fixture

import "database/sql"

// Direct member of the class: a pooled query inside the `for rows.Next()` body.
func Direct(db *sql.DB) error {
	rows, err := db.Query("SELECT id FROM a")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		var n int
		_ = db.QueryRow("SELECT COUNT(*) FROM b WHERE a_id = ?", id).Scan(&n)
	}
	return nil
}
