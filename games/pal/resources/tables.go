package resources

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
)

type EventObject struct {
	VanishTime                                                                              int16
	X, Y                                                                                    uint16
	Layer                                                                                   int16
	TriggerScript, AutoScript                                                               uint16
	State                                                                                   int16
	TriggerMode, Sprite, Frames, Direction, Frame, Idle, SpriteOffset, AutoFrames, AutoIdle uint16
}
type Scene struct{ Map, OnEnter, OnTeleport, FirstEvent uint16 }
type Script struct {
	Opcode   uint16
	Operands [3]uint16
}
type Tables struct {
	Events          []EventObject
	Scenes          []Scene
	Objects         [][7]uint16
	Scripts         []Script
	PlayerRoles     [75][6]uint16
	Words, Messages [][]byte
	Data            *Archive
}

func loadArchive(files fs.FS, name string) (*Archive, error) {
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	a, err := OpenArchive(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return a, nil
}

// Load98Tables reads the Windows 7-word object format. Returned text remains
// original GBK bytes; rendering decodes it without altering script text/control
// codes. Graphics and sound archives are deliberately not embedded in rules.
func Load98Tables(files fs.FS) (*Tables, error) {
	sss, err := loadArchive(files, "sss.mkf")
	if err != nil {
		return nil, err
	}
	data, err := loadArchive(files, "data.mkf")
	if err != nil {
		return nil, err
	}
	if sss.Count() != 5 || data.Count() < 4 {
		return nil, errors.New("missing PAL rule tables")
	}
	chunks := make([][]byte, 5)
	for i, width := range []int{32, 8, 14, 4, 8} {
		chunks[i], err = sss.Chunk(i)
		if err != nil || len(chunks[i]) == 0 || len(chunks[i])%width != 0 {
			return nil, fmt.Errorf("invalid 98 SSS table %d", i)
		}
	}
	t := &Tables{Events: make([]EventObject, len(chunks[0])/32), Scenes: make([]Scene, len(chunks[1])/8), Objects: make([][7]uint16, len(chunks[2])/14), Scripts: make([]Script, len(chunks[4])/8), Data: data}
	for i, dst := range []any{&t.Events, &t.Scenes, &t.Objects, nil, &t.Scripts} {
		if dst != nil {
			if err = binary.Read(bytes.NewReader(chunks[i]), binary.LittleEndian, dst); err != nil {
				return nil, err
			}
		}
	}
	roles, err := data.Chunk(3)
	if err != nil || len(roles) != 900 {
		return nil, errors.New("invalid original PLAYERROLES table")
	}
	if err = binary.Read(bytes.NewReader(roles), binary.LittleEndian, &t.PlayerRoles); err != nil {
		return nil, err
	}
	words, err := fs.ReadFile(files, "word.dat")
	if err != nil {
		return nil, err
	}
	if len(words) == 0 || len(words)%10 != 0 {
		return nil, errors.New("invalid WORD.DAT width")
	}
	for i := 0; i < len(words); i += 10 {
		t.Words = append(t.Words, append([]byte{}, bytes.TrimRight(words[i:i+10], " \x00")...))
	}
	messageData, err := fs.ReadFile(files, "m.msg")
	if err != nil {
		return nil, err
	}
	if len(chunks[3]) < 8 {
		return nil, errors.New("missing message index")
	}
	offsets := make([]uint32, len(chunks[3])/4)
	if err = binary.Read(bytes.NewReader(chunks[3]), binary.LittleEndian, &offsets); err != nil {
		return nil, err
	}
	for i, offset := range offsets {
		if uint64(offset) > uint64(len(messageData)) || i > 0 && offset < offsets[i-1] {
			return nil, errors.New("invalid M.MSG offset")
		}
		if i > 0 {
			t.Messages = append(t.Messages, append([]byte{}, messageData[offsets[i-1]:offset]...))
		}
	}
	if offsets[0] != 0 || int(offsets[len(offsets)-1]) != len(messageData) {
		return nil, errors.New("incomplete M.MSG index")
	}
	return t, nil
}
