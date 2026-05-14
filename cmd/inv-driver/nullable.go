package main

import "database/sql"

// Thin wrappers around sql.Null* whose only job is to emit JSON null
// rather than the Go zero value when the column is SQL NULL.

type sqlNullFloat struct{ sql.NullFloat64 }

func (n sqlNullFloat) toAny() any {
	if !n.Valid {
		return nil
	}
	return n.Float64
}

type sqlNullInt struct{ sql.NullInt64 }

func (n sqlNullInt) toAny() any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

type sqlNullString struct{ sql.NullString }

func (n sqlNullString) toAny() any {
	if !n.Valid {
		return nil
	}
	return n.String
}
