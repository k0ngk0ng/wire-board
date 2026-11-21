package cs16

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const maxStreamCursorBytes = 64 * 1024

type streamCursor struct {
	Header  string
	Epoch   string
	Frame   int64
	EventID int64
	Round   int
	Object  map[string]json.RawMessage
}

func readStreamCursor(raw string, seat int) (streamCursor, bool) {
	value := streamCursor{}
	if raw == "" || len(raw) > maxStreamCursorBytes {
		return value, false
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 10 || len(parts[0]) != 24 || len(parts[4]) != 64 || (parts[5] != "0" && parts[5] != "1") || (parts[6] != "0" && parts[6] != "1") {
		return value, false
	}
	if _, err := hex.DecodeString(parts[4]); err != nil {
		return value, false
	}
	frame, frameErr := strconv.ParseInt(parts[1], 10, 64)
	eventID, eventErr := strconv.ParseInt(parts[2], 10, 64)
	round, roundErr := strconv.Atoi(parts[3])
	cursorSeat, seatErr := strconv.Atoi(parts[7])
	if frameErr != nil || eventErr != nil || roundErr != nil || seatErr != nil || cursorSeat != seat || frame < 0 || eventID < 0 || round < 1 {
		return value, false
	}
	body, err := base64.RawStdEncoding.DecodeString(parts[8])
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(body)) != parts[9] {
		return value, false
	}
	var identity struct {
		Schema  int    `json:"schema"`
		Epoch   string `json:"epoch"`
		Frame   int64  `json:"frame"`
		EventID int64  `json:"eventId"`
		Round   int    `json:"round"`
		You     int    `json:"you"`
	}
	if json.Unmarshal(body, &identity) != nil || identity.Schema != 1 || identity.Epoch != parts[0] || identity.Frame != frame || identity.EventID != eventID || identity.Round != round || identity.You != seat || json.Unmarshal(body, &value.Object) != nil {
		return streamCursor{}, false
	}
	value.Header, value.Epoch, value.Frame, value.EventID, value.Round = strings.Join(parts[:8], ":"), identity.Epoch, frame, eventID, round
	return value, true
}

func makeStreamCursor(view View, object map[string]json.RawMessage, brushDigest string) (string, error) {
	body, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	canAct, catchingUp := 0, 0
	if view.CanAct {
		canAct = 1
	}
	if view.CatchingUp {
		catchingUp = 1
	}
	cursor := fmt.Sprintf("%s:%d:%d:%d:%s:%d:%d:%d:%s:%x", view.Epoch, view.Frame, view.EventID, view.Round, brushDigest, canAct, catchingUp, view.You, base64.RawStdEncoding.EncodeToString(body), sha256.Sum256(body))
	if len(cursor) > maxStreamCursorBytes {
		return "recover", nil
	}
	return cursor, nil
}

func baselineObject(value View) (map[string]json.RawMessage, error) {
	value.Events, value.Recover = []EventView{}, false
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	return object, nil
}
