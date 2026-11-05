// Package data exposes only computational server resources. Client media,
// sprite pixels, audio and Wasm are never embedded by this package.
package data

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
)

//go:embed chu-world-rules.json.gz
var rulesGzip []byte

//go:embed renderstate-metadata.json.gz
var renderMetadataGzip []byte

//go:embed runtime-resource-descriptor.json
var resourceDescriptor []byte

// RuntimeResources is called once by the server's immutable library setup.
// Each returned byte slice is caller-owned; callbacks use already decoded data.
func RuntimeResources() (rules, metadata, descriptor []byte, err error) {
	unpack := func(raw []byte) ([]byte, error) {
		z, e := gzip.NewReader(bytes.NewReader(raw))
		if e != nil {
			return nil, e
		}
		defer z.Close()
		return io.ReadAll(io.LimitReader(z, 4<<20))
	}
	rules, err = unpack(rulesGzip)
	if err != nil {
		return nil, nil, nil, err
	}
	metadata, err = unpack(renderMetadataGzip)
	if err != nil {
		return nil, nil, nil, err
	}
	return rules, metadata, append([]byte(nil), resourceDescriptor...), nil
}
