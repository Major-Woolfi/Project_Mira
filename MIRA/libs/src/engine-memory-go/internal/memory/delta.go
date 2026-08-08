package memory

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"

	"github.com/pierrec/lz4/v4"
)

var _ = (*sync.Mutex)(nil)

func NewDeltaWriter(path string, bufferSize int) (*DeltaWriter, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &DeltaWriter{
		file:       file,
		bufferSize: bufferSize,
	}, nil
}

func (w *DeltaWriter) Write(record []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, record...)

	if len(w.buffer) >= w.bufferSize {
		return w.flush()
	}
	return nil
}

func (w *DeltaWriter) flush() error {
	compressed := CompressDeltaBlock(w.buffer)
	_, err := w.file.Write(compressed)
	w.buffer = w.buffer[:0]
	return err
}

func (w *DeltaWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.buffer) > 0 {
		if err := w.flush(); err != nil {
			return err
		}
	}
	return w.file.Close()
}

func CompressDeltaBlock(data []byte) []byte {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	writer.Write(data)
	writer.Close()
	return buf.Bytes()
}

func WriteRecordHeader(recordType uint8, timestamp uint32, payloadLen int) []byte {
	header := make([]byte, 8)
	header[0] = recordType
	binary.LittleEndian.PutUint32(header[1:5], timestamp)
	putUint24(header[5:8], uint32(payloadLen))
	return header
}

func DecodeRecordHeader(data []byte) (uint8, uint32, int) {
	recordType := data[0]
	timestamp := binary.LittleEndian.Uint32(data[1:5])
	payloadLen := int(getUint24(data[5:8]))
	return recordType, timestamp, payloadLen
}

func putUint24(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
}

func getUint24(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func DecodeNeurogenesisRecord(data []byte) NeurogenesisRecord {
	parentID := binary.LittleEndian.Uint32(data[0:4])
	clusterID := binary.LittleEndian.Uint16(data[4:6])
	seed := binary.LittleEndian.Uint16(data[6:8])
	neuronType := data[8]
	threshold := data[9]
	restingPot := data[10]
	return NeurogenesisRecord{
		ParentID:     parentID,
		ClusterID:    clusterID,
		Seed:         seed,
		Type:         neuronType,
		Threshold:    threshold,
		RestingPot:   restingPot,
	}
}

func DecodeSynaptogenesisRecord(data []byte) SynaptogenesisRecord {
	sourceID := binary.LittleEndian.Uint32(data[0:4])
	targetID := binary.LittleEndian.Uint32(data[4:8])
	weightIdx := data[8]
	delay := data[9]
	receptorType := data[10]
	return SynaptogenesisRecord{
		SourceID:     sourceID,
		TargetID:     targetID,
		WeightIdx:    weightIdx,
		Delay:        delay,
		ReceptorType: receptorType,
	}
}

func DecodeWeightUpdateRecord(data []byte) WeightUpdateRecord {
	sourceID := binary.LittleEndian.Uint32(data[0:4])
	targetID := binary.LittleEndian.Uint32(data[4:8])
	oldWeightIdx := data[8]
	newWeightIdx := data[9]
	delta := int8(data[10])
	return WeightUpdateRecord{
		SourceID:     sourceID,
		TargetID:     targetID,
		OldWeightIdx: oldWeightIdx,
		NewWeightIdx: newWeightIdx,
		Delta:        delta,
	}
}

func DecodeDeleteRecord(data []byte) DeleteRecord {
	sourceID := binary.LittleEndian.Uint32(data[0:4])
	targetID := binary.LittleEndian.Uint32(data[4:8])
	reason := data[8]
	return DeleteRecord{
		SourceID: sourceID,
		TargetID: targetID,
		Reason:   reason,
	}
}
