package memory

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"

	"github.com/pierrec/lz4/v4"
)

func NewDeltaReader(path string) (*DeltaReader, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &DeltaReader{
		file:       file,
		index:      make(map[uint32][]DeltaRecord),
		recordCount: 0,
		fileSize:   stat.Size(),
	}, nil
}

func (d *DeltaReader) Close() error {
	return d.file.Close()
}

func (d *DeltaReader) Scan(fn func(DeltaRecord) bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.scan(fn)
}

func (d *DeltaReader) scan(fn func(DeltaRecord) bool) {
	if d.index != nil {
		for _, records := range d.index {
			for _, rec := range records {
				if !fn(rec) {
					return
				}
			}
		}
		return
	}

	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return
	}

	for {
		var sizeBuf [4]byte
		_, err := io.ReadFull(d.file, sizeBuf[:])
		if err != nil {
			return
		}
		size := binary.LittleEndian.Uint32(sizeBuf[:])
		if size == 0 || size > 100*1024*1024 {
			return
		}

		compressed := make([]byte, size)
		_, err = io.ReadFull(d.file, compressed)
		if err != nil {
			return
		}

		reader := lz4.NewReader(bytes.NewReader(compressed))
		var buf bytes.Buffer
		_, err = reader.WriteTo(&buf)
		if err != nil {
			continue
		}
		decompressed := buf.Bytes()
		if len(decompressed) == 0 {
			continue
		}

		offset := 0
		for offset+8 <= len(decompressed) {
			recordType := decompressed[offset]
			timestamp := binary.LittleEndian.Uint32(decompressed[offset+1 : offset+5])
			payloadLen := int(getUint24(decompressed[offset+5 : offset+8]))
			offset += 8

			if offset+payloadLen > len(decompressed) {
				break
			}
			payload := make([]byte, payloadLen)
			copy(payload, decompressed[offset:offset+payloadLen])
			offset += payloadLen

			sourceID := extractSourceID(recordType, payload)

			record := DeltaRecord{
				Type:      recordType,
				Timestamp: timestamp,
				SourceID:  sourceID,
				Payload:   payload,
			}

			d.recordCount++
			if !fn(record) {
				return
			}
		}
	}
}

func (d *DeltaReader) RecordCount() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.recordCount
}

func (d *DeltaReader) FileSize() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fileSize
}

func (d *DeltaReader) BuildIndex() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.index = make(map[uint32][]DeltaRecord)
	d.scan(func(record DeltaRecord) bool {
		d.index[record.SourceID] = append(d.index[record.SourceID], record)
		return true
	})
}

func extractSourceID(recordType uint8, payload []byte) uint32 {
	if len(payload) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(payload[0:4])
}

func DecodeRecordType(data []byte) uint8 {
	return data[0]
}

func DecodeTimestamp(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data[1:5])
}

func DecodePayloadLen(data []byte) int {
	return int(getUint24(data[5:8]))
}
