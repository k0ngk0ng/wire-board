package cs16

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
)

type ObjectDelta struct {
	Values  map[string]json.RawMessage `json:"values,omitempty"`
	Remove  []string                   `json:"remove,omitempty"`
	Objects map[string]ObjectDelta     `json:"objects,omitempty"`
}

type EntityDelta struct {
	ID    int             `json:"id"`
	Full  json.RawMessage `json:"full,omitempty"`
	Patch *ObjectDelta    `json:"patch,omitempty"`
}

type CollectionDelta struct {
	Upsert []EntityDelta `json:"upsert,omitempty"`
	Remove []int         `json:"remove,omitempty"`
}

type DeltaView struct {
	Schema      int                        `json:"schema"`
	Epoch       string                     `json:"epoch"`
	BaseFrame   int64                      `json:"baseFrame"`
	BaseEventID int64                      `json:"baseEventId"`
	Frame       int64                      `json:"frame"`
	ServerAt    int64                      `json:"serverAt"`
	EventID     int64                      `json:"eventId"`
	Global      *ObjectDelta               `json:"global,omitempty"`
	Collections map[string]CollectionDelta `json:"collections,omitempty"`
	Events      []EventView                `json:"events"`
}

func objectDelta(previous, current map[string]json.RawMessage) ObjectDelta {
	value := ObjectDelta{}
	for key, raw := range current {
		before, exists := previous[key]
		if exists && bytes.Equal(before, raw) {
			continue
		}
		if exists && len(before) > 0 && len(raw) > 0 && before[0] == '{' && raw[0] == '{' {
			var oldObject, newObject map[string]json.RawMessage
			if json.Unmarshal(before, &oldObject) == nil && json.Unmarshal(raw, &newObject) == nil {
				nested := objectDelta(oldObject, newObject)
				if !emptyObjectDelta(nested) {
					if value.Objects == nil {
						value.Objects = map[string]ObjectDelta{}
					}
					value.Objects[key] = nested
				}
				continue
			}
		}
		if value.Values == nil {
			value.Values = map[string]json.RawMessage{}
		}
		value.Values[key] = raw
	}
	for key := range previous {
		if _, exists := current[key]; !exists {
			value.Remove = append(value.Remove, key)
		}
	}
	sort.Strings(value.Remove)
	return value
}

func emptyObjectDelta(value ObjectDelta) bool {
	return len(value.Values) == 0 && len(value.Remove) == 0 && len(value.Objects) == 0
}

func indexedCollection(raw json.RawMessage, idName string) (map[int]map[string]json.RawMessage, []int, error) {
	var entities []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entities); err != nil {
		return nil, nil, err
	}
	indexed := make(map[int]map[string]json.RawMessage, len(entities))
	ids := make([]int, 0, len(entities))
	for _, entity := range entities {
		var id int
		if err := json.Unmarshal(entity[idName], &id); err != nil {
			return nil, nil, err
		}
		if _, exists := indexed[id]; exists {
			return nil, nil, errors.New("实时投影实体编号重复")
		}
		indexed[id] = entity
		ids = append(ids, id)
	}
	return indexed, ids, nil
}

func collectionDelta(previous, current json.RawMessage, idName string, reset bool) (CollectionDelta, error) {
	value := CollectionDelta{}
	before, beforeIDs, err := indexedCollection(previous, idName)
	if err != nil {
		return value, err
	}
	after, afterIDs, err := indexedCollection(current, idName)
	if err != nil {
		return value, err
	}
	for _, id := range afterIDs {
		oldEntity, exists := before[id]
		entity := after[id]
		if reset || !exists || idName == "seat" && (!bytes.Equal(oldEntity["life"], entity["life"]) || !bytes.Equal(oldEntity["visible"], entity["visible"])) {
			raw, marshalErr := json.Marshal(entity)
			if marshalErr != nil {
				return value, marshalErr
			}
			value.Upsert = append(value.Upsert, EntityDelta{ID: id, Full: raw})
			continue
		}
		patch := objectDelta(oldEntity, entity)
		if !emptyObjectDelta(patch) {
			value.Upsert = append(value.Upsert, EntityDelta{ID: id, Patch: &patch})
		}
	}
	for _, id := range beforeIDs {
		if _, exists := after[id]; !exists {
			value.Remove = append(value.Remove, id)
		}
	}
	return value, nil
}

func fieldDelta(baseline streamCursor, current View, object map[string]json.RawMessage) (DeltaView, error) {
	value := DeltaView{Schema: 2, Epoch: current.Epoch, BaseFrame: baseline.Frame, BaseEventID: baseline.EventID, Frame: current.Frame, ServerAt: current.ServerAt, EventID: current.EventID, Events: current.Events}
	beforeGlobal := make(map[string]json.RawMessage, len(baseline.Object))
	afterGlobal := make(map[string]json.RawMessage, len(object))
	for key, raw := range baseline.Object {
		beforeGlobal[key] = raw
	}
	for key, raw := range object {
		afterGlobal[key] = raw
	}
	for _, name := range []string{"schema", "epoch", "frame", "serverAt", "eventId", "events", "recover"} {
		delete(beforeGlobal, name)
		delete(afterGlobal, name)
	}
	for _, name := range []string{"players", "hostages", "grenades", "drops", "brushes"} {
		idName := "id"
		if name == "players" {
			idName = "seat"
		}
		change, err := collectionDelta(beforeGlobal[name], afterGlobal[name], idName, name == "brushes" && baseline.Round != current.Round)
		if err != nil {
			return value, err
		}
		if len(change.Upsert) != 0 || len(change.Remove) != 0 {
			if value.Collections == nil {
				value.Collections = map[string]CollectionDelta{}
			}
			value.Collections[name] = change
		}
		delete(beforeGlobal, name)
		delete(afterGlobal, name)
	}
	global := objectDelta(beforeGlobal, afterGlobal)
	if !emptyObjectDelta(global) {
		value.Global = &global
	}
	return value, nil
}
