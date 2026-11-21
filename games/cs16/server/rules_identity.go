package cs16

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/cs16/data"
)

var rulesIdentities = loadRulesIdentities()

func loadRulesIdentities() map[string]string {
	var rows []json.RawMessage
	if err := json.Unmarshal([]byte(data.MapsJSON), &rows); err != nil {
		panic(err)
	}
	identities := map[string]string{}
	for _, row := range rows {
		var header struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(row, &header); err != nil {
			panic(err)
		}
		if header.ID == "" || identities[header.ID] != "" {
			panic("地图规则身份无效")
		}
		identities[header.ID] = fmt.Sprintf("%x", sha256.Sum256(row))
	}
	return identities
}
