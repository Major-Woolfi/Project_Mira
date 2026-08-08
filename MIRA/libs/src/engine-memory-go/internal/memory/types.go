package memory

import (
	"os"
	"sync"
	"time"
)

type Neuron struct {
	ID               uint32
	Seed             uint16
	ClusterID        uint16
	Type             uint8
	Threshold        float32
	RestingPotential float32
}

type MCogHeader struct {
	Magic           [4]byte
	Version         uint32
	TotalNeurons    uint64
	TotalClusters   uint32
	SectionCount    uint32
	CodebookOffset  uint64
	NeuronsOffset   uint64
	RulesOffset     uint64
	ClustersOffset  uint64
	Checksum        uint32
	Padding         [8]byte
}

type Codebook [256]float32

type NeuronParams struct {
	Seeds       []uint16
	ClusterIDs  []uint16
	Types       []uint8
	Thresholds  []uint8
	RestingPots []uint8
}

type PackedParams struct {
	Data []uint8
}

type ConnectivityRule struct {
	SourceCluster uint16
	TargetCluster uint16
	Probability   float32
	DistanceFunc  uint8
}

type ClusterMeta struct {
	ID           uint16
	X, Y, Z      float32
	NeuronCount  uint32
	FirstNeuronID uint32
	LayerID      uint8
	Type         uint8
}

type DeltaRecord struct {
	Type      uint8
	Timestamp uint32
	SourceID  uint32
	Payload   []byte
}

type NeurogenesisRecord struct {
	ParentID     uint32
	ClusterID    uint16
	Seed         uint16
	Type         uint8
	Threshold    uint8
	RestingPot   uint8
}

type SynaptogenesisRecord struct {
	SourceID     uint32
	TargetID     uint32
	WeightIdx    uint8
	Delay        uint8
	ReceptorType uint8
}

type WeightUpdateRecord struct {
	SourceID     uint32
	TargetID     uint32
	OldWeightIdx uint8
	NewWeightIdx uint8
	Delta        int8
}

type DeleteRecord struct {
	SourceID uint32
	TargetID uint32
	Reason   uint8
}

type Synapse struct {
	TargetID  uint32
	WeightIdx uint8
	Delay     uint8
	Type      uint8
}

type Spike struct {
	NeuronID  uint32
	Timestamp uint64
}

type EventType uint8

const (
	SpikeEvent EventType = iota
	STDPEvent
)

type Event struct {
	Type    EventType
	Payload interface{}
}

type CacheItem struct {
	Key       uint32
	Synapses  []Synapse
	Size      int
	LastUsed  time.Time
	AccessCnt uint32
}

type DeltaReader struct {
	file       *os.File
	index      map[uint32][]DeltaRecord
	recordCount int64
	fileSize   int64
	mu         sync.Mutex
}

type DeltaWriter struct {
	file        *os.File
	buffer      []byte
	bufferSize  int
	mu          sync.Mutex
}

type STDPUpdate struct {
	SourceID uint32
	TargetID uint32
	Delta    float32
}

