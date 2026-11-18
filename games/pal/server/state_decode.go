package pal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
)

func decodeStateJSON(raw []byte) (state, error) {
	if len(raw) == 0 || len(raw) > 48<<20 {
		return state{}, errors.New("无效仙剑数据")
	}
	prefix := []byte(`{"native":"`)
	if bytes.HasPrefix(raw, prefix) {
		if offset := bytes.IndexByte(raw[len(prefix):], '"'); offset >= 0 {
			end := len(prefix) + offset
			encoded := raw[len(prefix):end]
			capacity := base64.StdEncoding.DecodedLen(len(encoded))
			if capacity <= 32<<20+2 && !bytes.ContainsAny(encoded, "\r\n") {
				native := make([]byte, capacity)
				length, failure := base64.StdEncoding.Decode(native, encoded)
				if failure == nil {
					suffix := raw[end+1:]
					var body []byte
					if len(suffix) == 1 && suffix[0] == '}' {
						body = []byte(`{}`)
					} else if len(suffix) > 0 && suffix[0] == ',' {
						tail := bytes.TrimLeft(suffix[1:], " \t\r\n")
						if len(tail) > 0 && tail[0] == '"' {
							body = append([]byte{'{'}, suffix[1:]...)
						}
					}
					if body != nil {
						var metadata state
						header := struct {
							*state
							Native json.RawMessage `json:"native"`
						}{state: &metadata}
						if strict(body, &header, 48<<20) == nil && header.Native == nil {
							if length > 32<<20 {
								return state{}, errors.New("仙剑原生存档超出大小限制")
							}
							metadata.Native = native[:length]
							return metadata, nil
						}
					}
				}
			}
		}
	}
	var metadata state
	err := strict(raw, &metadata, 48<<20)
	if err == nil && len(metadata.Native) > 32<<20 {
		return state{}, errors.New("仙剑原生存档超出大小限制")
	}
	return metadata, err
}
