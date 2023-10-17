package ecs

import (
	"fmt"

	"pkg.world.dev/world-engine/cardinal/ecs/archetype"
	"pkg.world.dev/world-engine/cardinal/ecs/entity"
	"pkg.world.dev/world-engine/cardinal/ecs/filter"
	"pkg.world.dev/world-engine/cardinal/ecs/storage"
	"pkg.world.dev/world-engine/cardinal/ecs/store"
)

type cache struct {
	archetypes []archetype.ID
	seen       int
}

// Search represents a search for entities.
// It is used to filter entities based on their components.
// It receives arbitrary filters that are used to filter entities.
// It contains a cache that is used to avoid re-evaluating the search.
// So it is not recommended to create a new search every time you want
// to filter entities with the same search.
type Search struct {
	archMatches map[Namespace]*cache
	filter      filter.ComponentFilter
}

// NewSearch creates a new search.
// It receives arbitrary filters that are used to filter entities.
func NewSearch(filter filter.ComponentFilter) *Search {
	return &Search{
		archMatches: make(map[Namespace]*cache),
		filter:      filter,
	}
}

type SearchCallBackFn func(entity.ID) bool

// Each iterates over all entities that match the search.
// If you would like to stop the iteration, return false to the callback. To continue iterating, return true.
func (q *Search) Each(w *World, callback SearchCallBackFn) error {
	result := q.evaluateSearch(w.namespace, w.StoreManager())
	iter := storage.NewEntityIterator(0, w.StoreManager(), result)
	for iter.HasNext() {
		entities, err := iter.Next()
		if err != nil {
			return err
		}
		for _, id := range entities {
			cont := callback(id)
			if !cont {
				return nil
			}
		}
	}
	return nil
}

// Count returns the number of entities that match the search.
func (q *Search) Count(w *World) (int, error) {
	result := q.evaluateSearch(w.namespace, w.StoreManager())
	iter := storage.NewEntityIterator(0, w.StoreManager(), result)
	ret := 0
	for iter.HasNext() {
		entities, err := iter.Next()
		if err != nil {
			return 0, err
		}
		ret += len(entities)
	}
	return ret, nil
}

// First returns the first entity that matches the search.
func (q *Search) First(w *World) (id entity.ID, err error) {
	result := q.evaluateSearch(w.namespace, w.StoreManager())
	iter := storage.NewEntityIterator(0, w.StoreManager(), result)
	if !iter.HasNext() {
		return storage.BadID, err
	}
	for iter.HasNext() {
		entities, err := iter.Next()
		if err != nil {
			return 0, err
		}
		if len(entities) > 0 {
			return entities[0], nil
		}
	}
	return storage.BadID, err
}

func (q *Search) MustFirst(w *World) entity.ID {
	id, err := q.First(w)
	if err != nil {
		panic(fmt.Sprintf("no entity matches the search."))
	}
	return id
}

func (q *Search) evaluateSearch(namespace Namespace, sm store.IManager) []archetype.ID {
	if _, ok := q.archMatches[namespace]; !ok {
		q.archMatches[namespace] = &cache{
			archetypes: make([]archetype.ID, 0),
			seen:       0,
		}
	}
	cache := q.archMatches[namespace]
	for it := sm.SearchFrom(q.filter, cache.seen); it.HasNext(); {
		cache.archetypes = append(cache.archetypes, it.Next())
	}
	cache.seen = sm.ArchetypeCount()
	return cache.archetypes
}