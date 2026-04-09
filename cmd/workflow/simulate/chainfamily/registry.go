package chainfamily

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	adapters = map[string]Adapter{}
)

// Register adds an adapter for a chain family. Typically called from init().
// Panics if the family is already registered.
func Register(a Adapter) {
	mu.Lock()
	defer mu.Unlock()

	family := a.Family()
	if _, exists := adapters[family]; exists {
		panic(fmt.Sprintf("chainfamily: adapter already registered for %q", family))
	}
	adapters[family] = a
}

// Get returns the adapter for the given family, or nil if not registered.
func Get(family string) Adapter {
	mu.RLock()
	defer mu.RUnlock()
	return adapters[family]
}

// All returns all registered adapters in no particular order.
func All() []Adapter {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]Adapter, 0, len(adapters))
	for _, a := range adapters {
		out = append(out, a)
	}
	return out
}
