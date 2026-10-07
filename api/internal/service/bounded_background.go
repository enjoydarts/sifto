package service

import "sync"

// BoundedBackground coalesces pending values by key; at most eight keys run.
type BoundedBackground struct {
	mu      sync.Mutex
	pending map[string]map[string]struct{}
}

func (b *BoundedBackground) Submit(key, value string, work func([]string)) bool {
	b.mu.Lock()
	if b.pending == nil {
		b.pending = make(map[string]map[string]struct{})
	}
	if batch, busy := b.pending[key]; busy {
		if len(batch) < 100 {
			batch[value] = struct{}{}
		}
		b.mu.Unlock()
		return true
	}
	if len(b.pending) >= 8 {
		b.mu.Unlock()
		return false
	}
	b.pending[key] = map[string]struct{}{value: {}}
	b.mu.Unlock()
	go func() {
		defer func() {
			if recover() != nil {
				b.mu.Lock()
				delete(b.pending, key)
				b.mu.Unlock()
			}
		}()
		for {
			b.mu.Lock()
			batch := b.pending[key]
			if len(batch) == 0 {
				delete(b.pending, key)
				b.mu.Unlock()
				return
			}
			values := make([]string, 0, len(batch))
			for value := range batch {
				values = append(values, value)
			}
			b.pending[key] = make(map[string]struct{})
			b.mu.Unlock()
			work(values)
		}
	}()
	return true
}
