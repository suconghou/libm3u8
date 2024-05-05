package lruset

type LRUSet struct {
	capacity int
	items    map[string]bool
	ring     []string
	head     int
}

func NewLRUSet(capacity int) *LRUSet {
	return &LRUSet{
		capacity: capacity,
		items:    make(map[string]bool),
		ring:     make([]string, capacity),
	}
}

func (s *LRUSet) Add(key string) {
	if s.Exists(key) {
		return
	}

	if len(s.items) == s.capacity {
		delete(s.items, s.ring[s.head])
	}

	s.items[key] = true
	s.ring[s.head] = key
	s.head = (s.head + 1) % s.capacity
}

func (s *LRUSet) Exists(key string) bool {
	_, exists := s.items[key]
	return exists
}
