package cs16

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sync"
)

const maxHistoryBytes = 2097152

var historyEncoders = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return writer
}}

func validStoredSchema(schema int) bool {
	return schema == 1 || schema == 2
}

func compressHistory(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw) > maxHistoryBytes {
		return nil, errors.New("历史检查点大小无效")
	}
	var buffer bytes.Buffer
	writer := historyEncoders.Get().(*gzip.Writer)
	writer.Reset(&buffer)
	defer func() {
		writer.Reset(io.Discard)
		historyEncoders.Put(writer)
	}()
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func expandHistory(packed []byte, size int) ([]byte, error) {
	if size <= 0 || size > maxHistoryBytes || len(packed) == 0 || len(packed) > maxHistoryBytes {
		return nil, errors.New("历史检查点大小无效")
	}
	source := bytes.NewReader(packed)
	reader, err := gzip.NewReader(source)
	if err != nil {
		return nil, errors.New("历史检查点压缩格式无效")
	}
	reader.Multistream(false)
	raw, readErr := io.ReadAll(io.LimitReader(reader, int64(size)+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || len(raw) != size || source.Len() != 0 {
		return nil, errors.New("历史检查点校验失败")
	}
	return raw, nil
}
