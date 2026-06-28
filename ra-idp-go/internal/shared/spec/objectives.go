package spec

import (
	"math"
	"time"
)

func (s *SCL) ObjectiveBool(name, key string) (bool, bool) {
	objective, ok := s.Objectives[name]
	if !ok {
		return false, false
	}
	values, ok := objective.Value.(map[string]any)
	if !ok {
		return false, false
	}
	value, ok := values[key].(bool)
	return value, ok
}

func (s *SCL) ObjectiveInt(name, key string) (int, bool) {
	objective, ok := s.Objectives[name]
	if !ok {
		return 0, false
	}
	values, ok := objective.Value.(map[string]any)
	if !ok {
		return 0, false
	}
	switch value := values[key].(type) {
	case int:
		return value, true
	case uint64:
		if value > math.MaxInt {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func (s *SCL) ObjectiveNestedInt(name, group, key string) (int, bool) {
	objective, ok := s.Objectives[name]
	if !ok {
		return 0, false
	}
	values, ok := objective.Value.(map[string]any)
	if !ok {
		return 0, false
	}
	nested, ok := values[group].(map[string]any)
	if !ok {
		return 0, false
	}
	switch value := nested[key].(type) {
	case int:
		return value, true
	case uint64:
		if value > math.MaxInt {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func (s *SCL) ObjectiveLifetime(name string) (time.Duration, bool) {
	objective, ok := s.Objectives[name]
	if !ok || objective.Kind != "lifetime" {
		return 0, false
	}
	value, err := time.ParseDuration(objective.TTL)
	return value, err == nil
}
