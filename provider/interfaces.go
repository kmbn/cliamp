package provider

// FavoriteToggler is implemented by providers that support marking items
// as favorites (e.g. radio station favorites).
type FavoriteToggler interface {
	ToggleFavorite(id string) (added bool, name string, err error)
}

// CatalogLoader is implemented by providers that support lazy-loading
// catalog pages from an external source (e.g. Radio Browser API).
type CatalogLoader interface {
	// LoadCatalogPage fetches the next page of catalog entries starting at
	// offset. Returns the number of items added and any error.
	LoadCatalogPage(offset, limit int) (added int, err error)
}

// CatalogSearcher is implemented by providers that support server-side
// catalog search (e.g. radio station search via an API).
type CatalogSearcher interface {
	// SearchCatalog performs a server-side search. Results are reflected
	// in the next Playlists() call.
	SearchCatalog(query string) (int, error)
	ClearSearch()
	IsSearching() bool
}

// SectionedList is implemented by providers whose playlist list has
// logical sections (e.g. local stations, favorites, catalog).
type SectionedList interface {
	// IDPrefix returns the section prefix for a playlist ID (e.g. "f", "c", "s").
	IDPrefix(id string) string
	// IsFavoritableID reports whether the given ID can be favorited.
	IsFavoritableID(id string) bool
}

// Closer is implemented by providers that hold resources (sessions,
// connections) that should be released on shutdown.
type Closer interface {
	Close()
}
