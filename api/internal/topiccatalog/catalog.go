package topiccatalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type catalog struct {
	Version       int               `json:"version"`
	MaxTopics     int               `json:"max_topics_per_article"`
	Topics        []string          `json:"topics"`
	Aliases       map[string]string `json:"aliases"`
	GenreFallback map[string]string `json:"genre_fallback"`
}

var (
	loadOnce     sync.Once
	topicMapJSON string
	genreMapJSON string
	topicMap     map[string]string
	genreMap     map[string]string
	canonicalSet map[string]struct{}
	filterMap    map[string][]string
	maxTopics    int
	loadErr      error
)

// MappingsJSON returns lower-cased legacy-topic and genre mappings for SQL
// aggregation. Both the worker and API read the same catalog artifact.
func MappingsJSON() (string, string, error) {
	loadOnce.Do(func() {
		paths := []string{
			os.Getenv("TOPIC_CATALOG_PATH"),
			"/app/shared/topic_catalog.json",
			"/shared/topic_catalog.json",
			"shared/topic_catalog.json",
			"../shared/topic_catalog.json",
			"../../shared/topic_catalog.json",
			"../../../shared/topic_catalog.json",
		}
		var data []byte
		for _, path := range paths {
			if path == "" {
				continue
			}
			data, loadErr = os.ReadFile(filepath.Clean(path))
			if loadErr == nil {
				break
			}
		}
		if loadErr != nil {
			loadErr = fmt.Errorf("read topic catalog: %w", loadErr)
			return
		}
		var c catalog
		if loadErr = json.Unmarshal(data, &c); loadErr != nil {
			return
		}
		if c.Version != 1 {
			loadErr = fmt.Errorf("unsupported topic catalog version: %d", c.Version)
			return
		}
		if c.MaxTopics < 1 || c.MaxTopics > 5 {
			loadErr = fmt.Errorf("invalid maximum topics per article: %d", c.MaxTopics)
			return
		}
		maxTopics = c.MaxTopics
		allowed := make(map[string]struct{}, len(c.Topics))
		mapping := make(map[string]string, len(c.Topics)+len(c.Aliases))
		filterValues := make(map[string][]string, len(c.Topics))
		for _, topic := range c.Topics {
			key := strings.ToLower(strings.TrimSpace(topic))
			if key == "" {
				loadErr = fmt.Errorf("empty topic in catalog")
				return
			}
			if _, duplicate := allowed[key]; duplicate {
				loadErr = fmt.Errorf("duplicate topic in catalog: %s", topic)
				return
			}
			allowed[key] = struct{}{}
			mapping[key] = topic
			filterValues[topic] = []string{topic}
		}
		for alias, topic := range c.Aliases {
			if _, ok := allowed[strings.ToLower(topic)]; !ok {
				loadErr = fmt.Errorf("unknown canonical topic: %s", topic)
				return
			}
			mapping[strings.ToLower(strings.TrimSpace(alias))] = topic
			filterValues[topic] = append(filterValues[topic], alias)
		}
		for _, topic := range c.GenreFallback {
			if _, ok := allowed[strings.ToLower(topic)]; !ok {
				loadErr = fmt.Errorf("unknown fallback topic: %s", topic)
				return
			}
		}
		var raw []byte
		raw, loadErr = json.Marshal(mapping)
		if loadErr != nil {
			return
		}
		topicMapJSON = string(raw)
		topicMap = mapping
		canonicalSet = allowed
		for topic := range filterValues {
			seen := make(map[string]struct{}, len(filterValues[topic]))
			for _, value := range filterValues[topic] {
				seen[value] = struct{}{}
				seen[strings.ToLower(value)] = struct{}{}
			}
			filterValues[topic] = filterValues[topic][:0]
			for value := range seen {
				filterValues[topic] = append(filterValues[topic], value)
			}
			sort.Strings(filterValues[topic])
		}
		filterMap = filterValues
		raw, loadErr = json.Marshal(c.GenreFallback)
		genreMapJSON = string(raw)
		genreMap = c.GenreFallback
	})
	return topicMapJSON, genreMapJSON, loadErr
}

// FilterValues expands a canonical suggestion into legacy stored labels and
// genre fallbacks. Non-catalog legacy labels retain their exact-match behavior.
func FilterValues(label string) ([]string, []string, error) {
	if _, _, err := MappingsJSON(); err != nil {
		return nil, nil, err
	}
	label = strings.TrimSpace(label)
	if _, ok := canonicalSet[strings.ToLower(label)]; !ok {
		return []string{label}, nil, nil
	}
	canonical := topicMap[strings.ToLower(label)]
	topics := append([]string(nil), filterMap[canonical]...)
	genres := make([]string, 0)
	for genre, mapped := range genreMap {
		if mapped == canonical {
			genres = append(genres, genre)
		}
	}
	sort.Strings(genres)
	return topics, genres, nil
}

// Normalize is the persistence boundary: unknown model output never becomes a
// new stored topic. A genre fallback keeps an article classifiable.
func Normalize(topics []string, genre string) ([]string, error) {
	if _, _, err := MappingsJSON(); err != nil {
		return nil, err
	}
	out := make([]string, 0, maxTopics)
	seen := make(map[string]struct{}, maxTopics)
	for _, raw := range topics {
		key := strings.ToLower(strings.Join(strings.Fields(raw), " "))
		canonical, ok := topicMap[key]
		if !ok {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
		if len(out) == maxTopics {
			break
		}
	}
	if len(out) == 0 {
		if fallback := genreMap[strings.ToLower(strings.TrimSpace(genre))]; fallback != "" {
			out = append(out, fallback)
		}
	}
	return out, nil
}
