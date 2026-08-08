package memory

import (
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"os"
	"unsafe"

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/mmap"
)

type BaseReader struct {
	mmap       *mmap.MMap
	header     MCogHeader
	codebook   Codebook
	neurons    NeuronParams
	rules      []ConnectivityRule
	clusters   []ClusterMeta
	data       []byte
}

func NewBaseReader(path string) (*BaseReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	m, err := mmap.Open(path, stat.Size())
	if err != nil {
		return nil, err
	}

	data := m.Data()

	if len(data) < 64 {
		return nil, errors.New("invalid base.mcog: file too small")
	}

	header := MCogHeader{}
	copy(header.Magic[:], data[0:4])
	if string(header.Magic[:]) != "MCOG" {
		return nil, errors.New("invalid magic")
	}

	header.Version = binary.LittleEndian.Uint32(data[4:8])
	header.TotalNeurons = binary.LittleEndian.Uint64(data[8:16])
	header.TotalClusters = binary.LittleEndian.Uint32(data[16:20])
	header.SectionCount = binary.LittleEndian.Uint32(data[20:24])
	header.CodebookOffset = binary.LittleEndian.Uint64(data[24:32])
	header.NeuronsOffset = binary.LittleEndian.Uint64(data[32:40])
	header.RulesOffset = binary.LittleEndian.Uint64(data[40:48])
	header.ClustersOffset = binary.LittleEndian.Uint64(data[48:56])
	header.Checksum = binary.LittleEndian.Uint32(data[56:60])

	rand.Seed(42)

	codebook := Codebook{}
	for i := 0; i < 256; i++ {
		codebook[i] = float32(math.Exp(rand.NormFloat64()*0.8 - 1.0))
	}

	var rules []ConnectivityRule
	var clusters []ClusterMeta

	if header.RulesOffset > 0 && header.RulesOffset < uint64(len(data)) {
		offset := header.RulesOffset
		limit := uint64(len(data))
		if header.ClustersOffset > 0 && header.ClustersOffset < limit {
			limit = header.ClustersOffset
		}
		for offset+9 <= limit {
			rule := ConnectivityRule{
				SourceCluster: binary.LittleEndian.Uint16(data[offset : offset+2]),
				TargetCluster: binary.LittleEndian.Uint16(data[offset+2 : offset+4]),
				Probability:   math.Float32frombits(binary.LittleEndian.Uint32(data[offset+4 : offset+8])),
				DistanceFunc:  data[offset+8],
			}
			rules = append(rules, rule)
			offset += 9
		}
	}

	if header.ClustersOffset > 0 && header.ClustersOffset < uint64(len(data)) {
		offset := header.ClustersOffset
		for offset+24 <= uint64(len(data)) {
			cluster := ClusterMeta{
				ID:            binary.LittleEndian.Uint16(data[offset : offset+2]),
				X:             math.Float32frombits(binary.LittleEndian.Uint32(data[offset+2 : offset+6])),
				Y:             math.Float32frombits(binary.LittleEndian.Uint32(data[offset+6 : offset+10])),
				Z:             math.Float32frombits(binary.LittleEndian.Uint32(data[offset+10 : offset+14])),
				NeuronCount:   binary.LittleEndian.Uint32(data[offset+14 : offset+18]),
				FirstNeuronID: binary.LittleEndian.Uint32(data[offset+18 : offset+22]),
				LayerID:       data[offset+22],
				Type:          data[offset+23],
			}
			clusters = append(clusters, cluster)
			offset += 24
		}
	}

	return &BaseReader{
		mmap:     m,
		header:   header,
		codebook: codebook,
		rules:    rules,
		clusters: clusters,
		data:     data,
	}, nil
}

func (b *BaseReader) Close() error {
	return b.mmap.Close()
}

func (b *BaseReader) TotalNeurons() uint64 {
	return b.header.TotalNeurons
}

func (b *BaseReader) TotalClusters() uint32 {
	return b.header.TotalClusters
}

func (b *BaseReader) GetSeed(neuronID uint64) uint16 {
	byteOffset := neuronID * 6
	return *(*uint16)(unsafe.Pointer(&b.data[int(b.header.NeuronsOffset)+int(byteOffset)]))
}

func (b *BaseReader) GetClusterID(neuronID uint64) uint16 {
	byteOffset := neuronID*6 + 2
	return *(*uint16)(unsafe.Pointer(&b.data[int(b.header.NeuronsOffset)+int(byteOffset)]))
}

func (b *BaseReader) GetType(neuronID uint64) uint8 {
	byteOffset := neuronID*6 + 4
	packed := b.data[int(b.header.NeuronsOffset)+int(byteOffset)]
	return (packed >> 4) & 0x0F
}

func (b *BaseReader) GetThreshold(neuronID uint64) uint8 {
	byteOffset := neuronID*6 + 4
	packed := b.data[int(b.header.NeuronsOffset)+int(byteOffset)]
	return packed & 0x0F
}

func (b *BaseReader) GetRestingPotential(neuronID uint64) uint8 {
	byteOffset := neuronID*6 + 5
	return b.data[int(b.header.NeuronsOffset)+int(byteOffset)]
}

func (b *BaseReader) GetProbability(source, target uint16) float32 {
	for _, rule := range b.rules {
		if rule.SourceCluster == source && rule.TargetCluster == target {
			return rule.Probability
		}
	}
	return 0.0
}

func (b *BaseReader) GetClusterMeta(clusterID uint16) *ClusterMeta {
	for i := range b.clusters {
		if b.clusters[i].ID == clusterID {
			return &b.clusters[i]
		}
	}
	return nil
}

func GenerateCodebook() Codebook {
	var cb Codebook
	for i := 0; i < 256; i++ {
		cb[i] = float32(math.Exp(rand.NormFloat64()*0.8 - 1.0))
	}
	return cb
}

func decodeThreshold(v uint8) float32 {
	return -60.0 + float32(v)*(20.0/256.0)
}

func decodeRestingPotential(v uint8) float32 {
	return -75.0 + float32(v)*(20.0/256.0)
}
