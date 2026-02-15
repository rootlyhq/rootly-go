package rootly

import "github.com/google/uuid"

// ID is an ID type for the Rootly APIs, which is either a UUID or a slug.
type ID string

func (id ID) String() string {
	return string(id)
}

// IsUUID reports whether the ID is a valid UUID.
func (id ID) IsUUID() bool {
	_, ok := id.UUID()
	return ok
}

// UUID returns the resource UUID, or `false` if it is a slug-based resource ID
func (id ID) UUID() (uuid.UUID, bool) {
	parsed, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.UUID{}, false
	}
	// Reject null/zero UUIDs as invalid
	if parsed == (uuid.UUID{}) {
		return uuid.UUID{}, false
	}
	return parsed, true
}
