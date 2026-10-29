package types

import "strconv"

// ID identifies a persisted SQLite record. Platform identifiers remain strings.
type ID int64

func (id ID) Int64() int64 {
	return int64(id)
}

func (id ID) String() string {
	return strconv.FormatInt(int64(id), 10)
}
