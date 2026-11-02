package apiexternal_v2

import "sync"

// NzbSlice collects search results; Add/AddSlice/Len may be called from
// concurrent indexer searches.
type NzbSlice struct {
	Arr []Nzbwithprio
	// mu protects Arr during concurrent indexer searches. Per-instance so
	// concurrent searches don't serialize on a single global lock.
	mu sync.Mutex
}

// Add appends one result.
func (n *NzbSlice) Add(nzb *Nzbwithprio) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Arr = append(n.Arr, *nzb)
}

// AddSlice appends all entries under a single lock acquisition.
func (n *NzbSlice) AddSlice(arr []Nzbwithprio) {
	if len(arr) == 0 {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.Arr = append(n.Arr, arr...)
}

// Len returns the current number of entries, safe to call while other
// goroutines add.
func (n *NzbSlice) Len() int {
	n.mu.Lock()
	defer n.mu.Unlock()

	return len(n.Arr)
}
