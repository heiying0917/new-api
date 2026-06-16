package official_pricing

import "sync"

// In-memory cache of the most recently built book. The controller is
// responsible for populating it (after a refresh, or lazily from persistence).

var (
	cacheMu    sync.RWMutex
	cachedBook *Book
)

// GetCachedBook returns the in-memory book, or nil if none is loaded yet.
func GetCachedBook() *Book {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return cachedBook
}

// SetCachedBook stores the book in memory, rebuilding its lookup index.
func SetCachedBook(b *Book) {
	if b != nil {
		b.EnsureIndex()
	}
	cacheMu.Lock()
	cachedBook = b
	cacheMu.Unlock()
}
