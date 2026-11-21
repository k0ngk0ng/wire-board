package cs16

import "encoding/json"

type inputSpans struct {
	root      map[string]jsonSpan
	world     map[string]jsonSpan
	players   []jsonSpan
	duplicate bool
}

type inputScanner struct {
	raw      []byte
	position int
	spans    inputSpans
}

func scanInputArchive(raw []byte) (inputSpans, bool) {
	scanner := inputScanner{raw: raw}
	scanner.position = jsonSpace(raw, 0)
	valid := scanner.value(1, 0)
	scanner.position = jsonSpace(raw, scanner.position)
	return scanner.spans, valid && scanner.position == len(raw)
}

func (scanner *inputScanner) value(mode, depth int) bool {
	if scanner.position >= len(scanner.raw) {
		return false
	}
	switch scanner.raw[scanner.position] {
	case '{':
		return depth < 10000 && scanner.object(mode, depth+1)
	case '[':
		return depth < 10000 && scanner.array(mode, depth+1)
	case '"':
		return scanner.string()
	case 't':
		return scanner.literal("true")
	case 'f':
		return scanner.literal("false")
	case 'n':
		return scanner.literal("null")
	default:
		return scanner.number()
	}
}

func (scanner *inputScanner) string() bool {
	scanner.position++
	for scanner.position < len(scanner.raw) {
		value := scanner.raw[scanner.position]
		scanner.position++
		if value == '"' {
			return true
		}
		if value < 32 {
			return false
		}
		if value != '\\' {
			continue
		}
		if scanner.position >= len(scanner.raw) {
			return false
		}
		escape := scanner.raw[scanner.position]
		scanner.position++
		switch escape {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		case 'u':
			for range 4 {
				if scanner.position >= len(scanner.raw) {
					return false
				}
				digit := scanner.raw[scanner.position]
				if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F') {
					return false
				}
				scanner.position++
			}
		default:
			return false
		}
	}
	return false
}

func (scanner *inputScanner) literal(value string) bool {
	if len(scanner.raw)-scanner.position < len(value) || string(scanner.raw[scanner.position:scanner.position+len(value)]) != value {
		return false
	}
	scanner.position += len(value)
	return true
}

func (scanner *inputScanner) digits() bool {
	begin := scanner.position
	for scanner.position < len(scanner.raw) && scanner.raw[scanner.position] >= '0' && scanner.raw[scanner.position] <= '9' {
		scanner.position++
	}
	return scanner.position > begin
}

func (scanner *inputScanner) number() bool {
	if scanner.raw[scanner.position] == '-' {
		scanner.position++
	}
	if scanner.position >= len(scanner.raw) {
		return false
	}
	if scanner.raw[scanner.position] == '0' {
		scanner.position++
	} else if scanner.raw[scanner.position] < '1' || scanner.raw[scanner.position] > '9' || !scanner.digits() {
		return false
	}
	if scanner.position < len(scanner.raw) && scanner.raw[scanner.position] == '.' {
		scanner.position++
		if !scanner.digits() {
			return false
		}
	}
	if scanner.position < len(scanner.raw) && (scanner.raw[scanner.position] == 'e' || scanner.raw[scanner.position] == 'E') {
		scanner.position++
		if scanner.position < len(scanner.raw) && (scanner.raw[scanner.position] == '+' || scanner.raw[scanner.position] == '-') {
			scanner.position++
		}
		return scanner.digits()
	}
	return true
}

func (scanner *inputScanner) key(begin int) string {
	raw := scanner.raw[begin:scanner.position]
	for _, value := range raw[1 : len(raw)-1] {
		if value == '\\' || value >= 128 {
			var key string
			json.Unmarshal(raw, &key)
			return key
		}
	}
	return string(raw[1 : len(raw)-1])
}

func (scanner *inputScanner) object(mode, depth int) bool {
	var fields map[string]jsonSpan
	if mode == 1 || mode == 2 {
		fields = map[string]jsonSpan{}
		if mode == 1 {
			scanner.spans.root = fields
		} else {
			scanner.spans.world = fields
		}
	}
	scanner.position = jsonSpace(scanner.raw, scanner.position+1)
	if scanner.position < len(scanner.raw) && scanner.raw[scanner.position] == '}' {
		scanner.position++
		return true
	}
	for scanner.position < len(scanner.raw) {
		begin := scanner.position
		if scanner.raw[begin] != '"' || !scanner.string() {
			return false
		}
		key := ""
		if fields != nil {
			key = scanner.key(begin)
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position)
		if scanner.position >= len(scanner.raw) || scanner.raw[scanner.position] != ':' {
			return false
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position+1)
		begin = scanner.position
		childMode := 0
		if mode == 1 && key == "world" {
			childMode = 2
		} else if mode == 2 && key == "players" {
			childMode = 3
		}
		if !scanner.value(childMode, depth) {
			return false
		}
		if fields != nil {
			if _, exists := fields[key]; exists {
				scanner.spans.duplicate = true
			}
			fields[key] = jsonSpan{begin: begin, end: scanner.position}
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position)
		if scanner.position >= len(scanner.raw) {
			return false
		}
		separator := scanner.raw[scanner.position]
		scanner.position++
		if separator == '}' {
			return true
		}
		if separator != ',' {
			return false
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position)
	}
	return false
}

func (scanner *inputScanner) array(mode, depth int) bool {
	scanner.position = jsonSpace(scanner.raw, scanner.position+1)
	if scanner.position < len(scanner.raw) && scanner.raw[scanner.position] == ']' {
		scanner.position++
		return true
	}
	for scanner.position < len(scanner.raw) {
		begin := scanner.position
		if !scanner.value(0, depth) {
			return false
		}
		if mode == 3 && len(scanner.spans.players) < 11 {
			scanner.spans.players = append(scanner.spans.players, jsonSpan{begin: begin, end: scanner.position})
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position)
		if scanner.position >= len(scanner.raw) {
			return false
		}
		separator := scanner.raw[scanner.position]
		scanner.position++
		if separator == ']' {
			return true
		}
		if separator != ',' {
			return false
		}
		scanner.position = jsonSpace(scanner.raw, scanner.position)
	}
	return false
}
