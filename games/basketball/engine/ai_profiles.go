package engine

import "errors"

// Original pursuit choices by team and character slot. These are editable
// game parameters; no ROM or machine-code execution is needed at runtime.
var originalPursuitOptions = [8][4][4]AITacticalOption{
	{
		{{Destination: 0x20, Parameter: 4}, {Destination: 0x40, Parameter: 6}, {Destination: 0x48, Parameter: 10}, {Destination: 0x38, Parameter: 7}},
		{{Destination: 0x18, Parameter: 0}, {Destination: 0x38, Parameter: 1}, {Destination: 0x40, Parameter: 3}, {Destination: 0x50, Parameter: 6}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 3}, {Destination: 0x30, Parameter: 1}, {Destination: 0x30, Parameter: 2}},
		{{Destination: 0x20, Parameter: 6}, {Destination: 0x40, Parameter: 11}, {Destination: 0x30, Parameter: 3}, {Destination: 0x20, Parameter: 2}},
	},
	{
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 4}, {Destination: 0x18, Parameter: 7}, {Destination: 0x40, Parameter: 10}},
		{{Destination: 0x30, Parameter: 0}, {Destination: 0x40, Parameter: 3}, {Destination: 0x60, Parameter: 5}, {Destination: 0x50, Parameter: 6}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x30, Parameter: 6}, {Destination: 0x30, Parameter: 2}, {Destination: 0x30, Parameter: 7}},
		{{Destination: 0x30, Parameter: 6}, {Destination: 0x30, Parameter: 2}, {Destination: 0x18, Parameter: 3}, {Destination: 0x30, Parameter: 5}},
	},
	{
		{{Destination: 0x10, Parameter: 4}, {Destination: 0x20, Parameter: 0}, {Destination: 0x30, Parameter: 11}, {Destination: 0x10, Parameter: 3}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x30, Parameter: 2}, {Destination: 0x40, Parameter: 10}, {Destination: 0x48, Parameter: 7}},
		{{Destination: 0x30, Parameter: 11}, {Destination: 0x30, Parameter: 6}, {Destination: 0x30, Parameter: 7}, {Destination: 0x20, Parameter: 5}},
		{{Destination: 0x18, Parameter: 3}, {Destination: 0x18, Parameter: 0}, {Destination: 0x20, Parameter: 10}, {Destination: 0x18, Parameter: 4}},
	},
	{
		{{Destination: 0x10, Parameter: 0}, {Destination: 0x30, Parameter: 1}, {Destination: 0x20, Parameter: 2}, {Destination: 0x50, Parameter: 5}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x18, Parameter: 4}, {Destination: 0x20, Parameter: 10}, {Destination: 0x30, Parameter: 7}},
		{{Destination: 0x20, Parameter: 4}, {Destination: 0x20, Parameter: 7}, {Destination: 0x30, Parameter: 5}, {Destination: 0x30, Parameter: 1}},
		{{Destination: 0x30, Parameter: 3}, {Destination: 0x18, Parameter: 1}, {Destination: 0x18, Parameter: 4}, {Destination: 0x28, Parameter: 2}},
	},
	{
		{{Destination: 0x30, Parameter: 1}, {Destination: 0x38, Parameter: 7}, {Destination: 0x40, Parameter: 3}, {Destination: 0x30, Parameter: 5}},
		{{Destination: 0x20, Parameter: 3}, {Destination: 0x30, Parameter: 7}, {Destination: 0x20, Parameter: 1}, {Destination: 0x18, Parameter: 0}},
		{{Destination: 0x40, Parameter: 3}, {Destination: 0x20, Parameter: 4}, {Destination: 0x20, Parameter: 2}, {Destination: 0x20, Parameter: 6}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 1}, {Destination: 0x20, Parameter: 2}, {Destination: 0x20, Parameter: 7}},
	},
	{
		{{Destination: 0x20, Parameter: 4}, {Destination: 0x30, Parameter: 0}, {Destination: 0x40, Parameter: 5}, {Destination: 0x38, Parameter: 2}},
		{{Destination: 0x28, Parameter: 2}, {Destination: 0x30, Parameter: 6}, {Destination: 0x28, Parameter: 7}, {Destination: 0x40, Parameter: 11}},
		{{Destination: 0x20, Parameter: 4}, {Destination: 0x30, Parameter: 0}, {Destination: 0x40, Parameter: 5}, {Destination: 0x38, Parameter: 2}},
		{{Destination: 0x28, Parameter: 2}, {Destination: 0x30, Parameter: 6}, {Destination: 0x28, Parameter: 7}, {Destination: 0x40, Parameter: 11}},
	},
	{
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 4}, {Destination: 0x40, Parameter: 1}, {Destination: 0x18, Parameter: 3}},
		{{Destination: 0x28, Parameter: 0}, {Destination: 0x30, Parameter: 4}, {Destination: 0x40, Parameter: 3}, {Destination: 0x50, Parameter: 1}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 7}, {Destination: 0x20, Parameter: 4}, {Destination: 0x30, Parameter: 5}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 4}, {Destination: 0x30, Parameter: 5}, {Destination: 0x30, Parameter: 7}},
	},
	{
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x40, Parameter: 2}, {Destination: 0x38, Parameter: 1}, {Destination: 0x50, Parameter: 6}},
		{{Destination: 0x30, Parameter: 4}, {Destination: 0x30, Parameter: 5}, {Destination: 0x38, Parameter: 1}, {Destination: 0xf0, Parameter: 7}},
		{{Destination: 0x20, Parameter: 0}, {Destination: 0x20, Parameter: 1}, {Destination: 0x30, Parameter: 2}, {Destination: 0x30, Parameter: 6}},
		{{Destination: 0x50, Parameter: 7}, {Destination: 0x20, Parameter: 0}, {Destination: 0x30, Parameter: 10}, {Destination: 0x40, Parameter: 11}},
	},
}

func OriginalPursuitOptions(team, character uint8) ([4]AITacticalOption, error) {
	if team >= 8 || character >= 4 {
		return [4]AITacticalOption{}, errors.New("unknown original pursuit profile")
	}
	return originalPursuitOptions[team][character], nil
}
