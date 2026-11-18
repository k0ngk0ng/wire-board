package resources

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

type VisualFile struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	MIME   string `json:"mime"`
}
type VisualIndex struct {
	Version int                   `json:"version"`
	Files   map[string]VisualFile `json:"files"`
	Sources map[string]string     `json:"sources"`
}
type AtlasFrame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type Atlas struct {
	Width  int          `json:"width"`
	Height int          `json:"height"`
	Image  string       `json:"image"`
	Frames []AtlasFrame `json:"frames"`
}
type VisualMap struct {
	ID     int    `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Tiles  string `json:"tiles"`
	Atlas  string `json:"atlas"`
}

// ExportVisuals keeps original palette indices in the PNG red channel and
// original transparency in alpha. Clients apply the selected original palette
// once per cached atlas; this avoids duplicating every atlas for every palette.
func ExportVisuals(input fs.FS, output string) (VisualIndex, error) {
	index := VisualIndex{Version: 1, Files: map[string]VisualFile{}, Sources: map[string]string{}}
	write := func(name, mime string, data []byte) error {
		if !fs.ValidPath(name) {
			return fmt.Errorf("invalid visual output path %q", name)
		}
		target := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		index.Files[name] = VisualFile{int64(len(data)), hex.EncodeToString(sum[:]), mime}
		return nil
	}
	writeJSON := func(name string, value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return write(name, "application/json", data)
	}
	open := func(name string) (*Archive, error) {
		data, err := fs.ReadFile(input, name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		index.Sources[name] = hex.EncodeToString(sum[:])
		return OpenArchive(data)
	}
	makeAtlas := func(group string, id int, bitmaps []IndexedBitmap) error {
		width := 512
		for _, b := range bitmaps {
			if b.Width > width {
				width = b.Width
			}
		}
		frames := make([]AtlasFrame, len(bitmaps))
		x, y, row := 0, 0, 0
		for i, b := range bitmaps {
			w, h := max(b.Width, 1), max(b.Height, 1)
			if x+w > width {
				x = 0
				y += row
				row = 0
			}
			frames[i] = AtlasFrame{x, y, b.Width, b.Height}
			x += w + 1
			row = max(row, h+1)
		}
		height := max(y+row, 1)
		if height > 16384 {
			return fmt.Errorf("atlas %s/%d exceeds texture limit", group, id)
		}
		pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
		for i, b := range bitmaps {
			r := frames[i]
			for yy := 0; yy < b.Height; yy++ {
				for xx := 0; xx < b.Width; xx++ {
					source := yy*b.Width + xx
					target := (r.Y+yy)*pixels.Stride + (r.X+xx)*4
					pixels.Pix[target] = b.Pixels[source]
					pixels.Pix[target+3] = b.Alpha[source]
				}
			}
		}
		base := path.Join("atlases", group, fmt.Sprint(id))
		var encoded bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&encoded, pixels); err != nil {
			return err
		}
		if err := write(base+".png", "image/png", encoded.Bytes()); err != nil {
			return err
		}
		return writeJSON(base+".json", Atlas{width, height, base + ".png", frames})
	}
	for _, group := range []string{"gop", "mgo", "abc", "f", "fire", "ball", "rgm", "fbp", "data"} {
		a, err := open(group + ".mkf")
		if err != nil {
			return index, err
		}
		for id := 0; id < a.Count(); id++ {
			if group == "data" && id != 9 && id != 10 && id != 12 {
				continue
			}
			data, _ := a.Chunk(id)
			if len(data) == 0 {
				continue
			}
			if group == "mgo" || group == "abc" || group == "f" || group == "fire" || group == "fbp" {
				data, err = DecompressYJ2(data, 8*1024*1024)
				if err != nil {
					return index, fmt.Errorf("%s/%d: %w", group, id, err)
				}
			}
			var bitmaps []IndexedBitmap
			if group == "fbp" {
				if len(data) != 64000 {
					return index, fmt.Errorf("invalid original background %d", id)
				}
				alpha := bytes.Repeat([]byte{255}, 64000)
				bitmaps = []IndexedBitmap{{320, 200, data, alpha}}
			} else if group == "ball" || group == "rgm" {
				bitmap, err := DecodeRLE(data)
				if err != nil {
					return index, fmt.Errorf("%s/%d: %w", group, id, err)
				}
				bitmaps = []IndexedBitmap{bitmap}
			} else {
				frames, err := SpriteAddressableMetrics(data)
				if err != nil {
					return index, fmt.Errorf("%s/%d: %w", group, id, err)
				}
				for frame, metric := range frames {
					bitmap, err := DecodeRLE(data[metric.Offset:])
					if err != nil {
						return index, fmt.Errorf("%s/%d frame %d: %w", group, id, frame, err)
					}
					bitmaps = append(bitmaps, bitmap)
				}
			}
			if err := makeAtlas(group, id, bitmaps); err != nil {
				return index, err
			}
		}
	}
	movies, err := open("rng.mkf")
	if err != nil {
		return index, err
	}
	for clip := 0; clip < movies.Count(); clip++ {
		chunk, _ := movies.Chunk(clip)
		if len(chunk) == 0 {
			continue
		}
		frames, err := DecodeRNGClip(chunk)
		if err != nil {
			return index, fmt.Errorf("RNG %d: %w", clip, err)
		}
		commands, err := packArchive(frames)
		if err != nil {
			return index, err
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err = writer.Write(commands); err != nil {
			return index, err
		}
		if err = writer.Close(); err != nil {
			return index, err
		}
		name := path.Join("movies", fmt.Sprint(clip))
		if err = write(name+".bin.gz", "application/gzip", compressed.Bytes()); err != nil {
			return index, err
		}
		metadata := struct {
			Version int    `json:"version"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			Frames  int    `json:"frames"`
			Data    string `json:"data"`
		}{1, 320, 200, len(frames), name + ".bin.gz"}
		if err = writeJSON(name+".json", metadata); err != nil {
			return index, err
		}
	}
	maps, err := open("map.mkf")
	if err != nil {
		return index, err
	}
	for id := 1; id < maps.Count(); id++ {
		data, _ := maps.Chunk(id)
		if len(data) == 0 {
			continue
		}
		tiles, err := DecompressYJ2(data, 65536)
		if err != nil {
			return index, err
		}
		if len(tiles) != 65536 {
			return index, fmt.Errorf("invalid original map %d", id)
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err = writer.Write(tiles); err != nil {
			return index, err
		}
		if err = writer.Close(); err != nil {
			return index, err
		}
		name := path.Join("maps", fmt.Sprint(id)+".bin.gz")
		if err = write(name, "application/gzip", compressed.Bytes()); err != nil {
			return index, err
		}
		if err = writeJSON(path.Join("maps", fmt.Sprint(id)+".json"), VisualMap{id, 64, 128, name, path.Join("atlases", "gop", fmt.Sprint(id)+".json")}); err != nil {
			return index, err
		}
	}
	pat, err := open("pat.mkf")
	if err != nil {
		return index, err
	}
	palettes := make([][2][256][3]uint8, pat.Count())
	for id := range palettes {
		data, _ := pat.Chunk(id)
		if len(data) < 768 {
			return index, fmt.Errorf("invalid palette %d", id)
		}
		for night := 0; night < 2; night++ {
			offset := 0
			if len(data) > 768 {
				offset = night * 768
			}
			if offset+768 > len(data) {
				return index, fmt.Errorf("truncated night palette %d", id)
			}
			for color := 0; color < 256; color++ {
				for k := 0; k < 3; k++ {
					palettes[id][night][color][k] = data[offset+color*3+k] << 2
				}
			}
		}
	}
	if err = writeJSON("palettes.json", palettes); err != nil {
		return index, err
	}
	data, err := json.Marshal(index)
	if err != nil {
		return index, err
	}
	if err = os.WriteFile(filepath.Join(output, "index.json"), data, 0644); err != nil {
		return index, err
	}
	return index, nil
}
