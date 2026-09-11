package diagnostics

import "sync"

type seen struct {
	mu   sync.Mutex
	keys map[string]map[string]struct{}
}

var prior = &seen{keys: map[string]map[string]struct{}{}}

func (s *seen) sift(path string, findings []finding) []finding {
	s.mu.Lock()
	defer s.mu.Unlock()
	remembered, tracked := s.keys[path]
	var fresh []finding
	for _, f := range findings {
		if tracked {
			if _, shown := remembered[f.key()]; shown {
				continue
			}
		}
		fresh = append(fresh, f)
	}
	s.remember(path, findings)
	return fresh
}

func (s *seen) learn(path string, findings []finding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remember(path, findings)
}

func (s *seen) remember(path string, findings []finding) {
	current := make(map[string]struct{}, len(findings))
	for _, f := range findings {
		current[f.key()] = struct{}{}
	}
	s.keys[path] = current
}
