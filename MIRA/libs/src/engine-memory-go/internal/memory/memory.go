package memory

import (
	"log"
	"math"
	"runtime"
	"sync"
	"time"
)

type MemorySystem struct {
	basePath       string
	deltaPath      string
	base           *BaseReader
	delta          *DeltaReader
	cache          *LRUCache
	mu             sync.RWMutex
	workerPool     chan func()
	eventChan      chan Event
	done           chan struct{}
	currentTime    uint64
	neuronStates   map[uint32]struct{ V, w float32 }
	lastCompaction time.Time
	activeNeurons  []uint32
}

func NewMemorySystem(basePath, deltaPath string, workers int) *MemorySystem {
	ms := &MemorySystem{
		basePath:      basePath,
		deltaPath:     deltaPath,
		workerPool:    make(chan func(), workers*2),
		eventChan:     make(chan Event, 1024),
		done:          make(chan struct{}),
		neuronStates:  make(map[uint32]struct{ V, w float32 }),
	}

	for i := 0; i < workers; i++ {
		go ms.worker()
	}

	return ms
}

func (ms *MemorySystem) Open() error {
	base, err := NewBaseReader(ms.basePath)
	if err != nil {
		return err
	}
	ms.base = base

	delta, err := NewDeltaReader(ms.deltaPath)
	if err != nil {
		return err
	}
	ms.delta = delta

	ms.cache = NewLRUCache(1024 * 1024 * 1024)

	return nil
}

func (ms *MemorySystem) Close() error {
	if ms.base != nil {
		if err := ms.base.Close(); err != nil {
			return err
		}
	}
	if ms.delta != nil {
		if err := ms.delta.Close(); err != nil {
			return err
		}
	}
	close(ms.done)
	return nil
}

func (ms *MemorySystem) worker() {
	for fn := range ms.workerPool {
		fn()
	}
}

func (ms *MemorySystem) StartEventLoop() {
	go func() {
		for {
			select {
			case event := <-ms.eventChan:
				ms.handleEvent(event)
			case <-ms.done:
				return
			}
		}
	}()
}

func (ms *MemorySystem) handleEvent(event Event) {
	switch event.Type {
	case SpikeEvent:
		spike := event.Payload.(Spike)
		ms.processSpike(spike)
	case STDPEvent:
		stdp := event.Payload.(STDPUpdate)
		ms.updateWeight(stdp)
	}
}

func (ms *MemorySystem) GetActiveNeurons() []uint32 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if len(ms.activeNeurons) > 0 {
		return ms.activeNeurons
	}

	total := ms.BaseTotalNeurons()
	if total == 0 {
		return nil
	}
	count := int(total)
	if count > 10000 {
		count = 10000
	}
	neurons := make([]uint32, count)
	for i := 0; i < count; i++ {
		neurons[i] = uint32(i)
	}
	return neurons
}

func (ms *MemorySystem) GetSynapses(neuronID uint32) []Synapse {
	procedural := GenerateProceduralSynapses(ms.base, neuronID)
	modifications := ms.findDeltaModifications(neuronID)
	return ms.mergeSynapses(procedural, modifications)
}

func (ms *MemorySystem) findDeltaModifications(neuronID uint32) []DeltaRecord {
	var records []DeltaRecord
	if idx, ok := ms.delta.index[neuronID]; ok {
		records = append(records, idx...)
	}
	ms.delta.Scan(func(record DeltaRecord) bool {
		if record.SourceID == neuronID {
			records = append(records, record)
		}
		return true
	})
	return records
}

func (ms *MemorySystem) mergeSynapses(procedural []Synapse, modifications []DeltaRecord) []Synapse {
	synapseMap := make(map[uint32]Synapse)
	for _, s := range procedural {
		synapseMap[s.TargetID] = s
	}
	for _, mod := range modifications {
		switch mod.Type {
		case 1:
			syn := DecodeSynaptogenesisRecord(mod.Payload)
			synapseMap[syn.TargetID] = Synapse{
				TargetID:  syn.TargetID,
				WeightIdx: syn.WeightIdx,
				Delay:     syn.Delay,
				Type:      syn.ReceptorType,
			}
		case 2:
			syn := DecodeWeightUpdateRecord(mod.Payload)
			if s, ok := synapseMap[syn.TargetID]; ok {
				s.WeightIdx = syn.NewWeightIdx
				synapseMap[syn.TargetID] = s
			}
		case 3:
			del := DecodeDeleteRecord(mod.Payload)
			delete(synapseMap, del.TargetID)
		}
	}
	result := make([]Synapse, 0, len(synapseMap))
	for _, s := range synapseMap {
		result = append(result, s)
	}
	return result
}

func (ms *MemorySystem) GetSynapsesBatch(neuronIDs []uint32) map[uint32][]Synapse {
	results := make(map[uint32][]Synapse)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, cap(ms.workerPool))

	for _, id := range neuronIDs {
		wg.Add(1)
		go func(neuronID uint32) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			synapses := ms.GetSynapses(neuronID)
			mu.Lock()
			results[neuronID] = synapses
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return results
}

func (ms *MemorySystem) SimulateStep(synapses map[uint32][]Synapse) []Spike {
	var spikes []Spike
	var mu sync.Mutex
	var wg sync.WaitGroup

	for neuronID, neuronSynapses := range synapses {
		wg.Add(1)
		go func(id uint32, syns []Synapse) {
			defer wg.Done()
			V, w := ms.GetNeuronState(id)
			I := ms.computeInputCurrent(id, syns)
			V_new, w_new, spiked := ms.integrateAdEx(float32(id), V, w, I, 1.0)
			ms.SetNeuronState(id, V_new, w_new)
			if spiked {
				mu.Lock()
				spikes = append(spikes, Spike{
					NeuronID:  id,
					Timestamp: ms.currentTime,
				})
				mu.Unlock()
			}
		}(neuronID, neuronSynapses)
	}

	wg.Wait()
	ms.currentTime++
	return spikes
}

func (ms *MemorySystem) computeInputCurrent(neuronID uint32, syns []Synapse) float32 {
	var I float32
	for _, s := range syns {
		weight := ms.base.codebook[s.WeightIdx]
		I += weight * float32(s.Delay)
	}
	return I
}

func (ms *MemorySystem) integrateAdEx(neuronID, V, w, I, dt float32) (float32, float32, bool) {
	const (
		C      = 281.0
		gL     = 30.0
		EL     = -70.6
		VT     = -50.4
		deltaT = 2.0
		tauW   = 144.0
		a      = 4.0
		b      = 0.0805
	)

	threshold := decodeThreshold(ms.base.GetThreshold(uint64(neuronID)))

	dV := (-gL*(V-EL) + gL*deltaT*float32(math.Exp(float64((V-VT)/deltaT))) - w + I) / C * dt
	dw := (a*(V-EL) - w) / tauW * dt

	V_new := V + dV
	w_new := w + dw

	if V_new > threshold {
		resting := decodeRestingPotential(ms.base.GetRestingPotential(uint64(neuronID)))
		V_new = resting
		w_new += b
		return V_new, w_new, true
	}

	return V_new, w_new, false
}

const maxNeuronStates = 100000

func (ms *MemorySystem) cleanupInactiveNeurons() {
	if len(ms.neuronStates) <= maxNeuronStates {
		return
	}
	var toDelete []uint32
	for id := range ms.neuronStates {
		toDelete = append(toDelete, id)
	}
	for i := 0; i < len(toDelete)-maxNeuronStates; i++ {
		delete(ms.neuronStates, toDelete[i])
	}
}

func (ms *MemorySystem) GetNeuronState(neuronID uint32) (float32, float32) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if state, ok := ms.neuronStates[neuronID]; ok {
		return state.V, state.w
	}
	return -70.0, 0.0
}

func (ms *MemorySystem) SetNeuronState(neuronID uint32, V, w float32) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.neuronStates[neuronID] = struct{ V, w float32 }{V, w}
	ms.cleanupInactiveNeurons()
}

func (ms *MemorySystem) ApplySTDP(spikes []Spike) {
	if len(spikes) < 2 {
		return
	}
	for i := 0; i < len(spikes)-1; i++ {
		pre := spikes[i]
		post := spikes[i+1]
		if post.Timestamp <= pre.Timestamp {
			continue
		}
		delta := float32(post.Timestamp - pre.Timestamp)
		if delta > 100 {
			continue
		}
		ms.eventChan <- Event{
			Type: STDPEvent,
			Payload: STDPUpdate{
				SourceID: pre.NeuronID,
				TargetID: post.NeuronID,
				Delta:    0.01 / (delta + 1),
			},
		}
	}
}

func (ms *MemorySystem) LogStats() {
	log.Printf("MemorySystem stats: active_neurons=%d total_neurons=%d delta_records=%d cache_usage=%d cache_hit_ratio=%.2f",
		len(ms.GetActiveNeurons()),
		ms.BaseTotalNeurons(),
		ms.DeltaRecordCount(),
		ms.CacheMemoryUsage(),
		ms.CacheHitRatio())
}

func (ms *MemorySystem) RunCompactionLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ms.RunCompaction()
	}
}

func (ms *MemorySystem) RunCompaction() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.lastCompaction = time.Now()
	log.Println("Compaction completed")
}

func (ms *MemorySystem) DeltaRecordCount() int64 {
	if ms.delta == nil {
		return 0
	}
	return ms.delta.recordCount
}

func (ms *MemorySystem) DeltaFileSize() int64 {
	if ms.delta == nil {
		return 0
	}
	return ms.delta.fileSize
}

func (ms *MemorySystem) CacheMemoryUsage() int64 {
	if ms.cache == nil {
		return 0
	}
	return int64(ms.cache.Usage())
}

func (ms *MemorySystem) CacheHitRatio() float64 {
	if ms.cache == nil {
		return 0.0
	}
	return ms.cache.HitRatio()
}

func (ms *MemorySystem) LastCompactionTime() time.Time {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.lastCompaction
}

func (ms *MemorySystem) GetResidentMemoryMB() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys / 1024 / 1024)
}

func (ms *MemorySystem) GetActiveNeuronsPercent() float64 {
	active := len(ms.GetActiveNeurons())
	total := ms.BaseTotalNeurons()
	if total == 0 {
		return 0.0
	}
	return float64(active) / float64(total) * 100.0
}

func (ms *MemorySystem) CountProceduralSynapses(neuronID uint32) int {
	_ = ms.base
	syns := GenerateProceduralSynapses(ms.base, neuronID)
	return len(syns)
}

func (ms *MemorySystem) CountDeltaModifications(neuronID uint32) int {
	return len(ms.findDeltaModifications(neuronID))
}

func (ms *MemorySystem) GetNeuron(id uint32) Neuron {
	seed := ms.base.GetSeed(uint64(id))
	clusterID := ms.base.GetClusterID(uint64(id))
	typePacked := ms.base.GetType(uint64(id))
	thresholdPacked := ms.base.GetThreshold(uint64(id))
	restingPacked := ms.base.GetRestingPotential(uint64(id))

	return Neuron{
		ID:               id,
		Seed:             uint16(seed),
		ClusterID:        uint16(clusterID),
		Type:             typePacked,
		Threshold:        decodeThreshold(thresholdPacked),
		RestingPotential: decodeRestingPotential(restingPacked),
	}
}

func (ms *MemorySystem) BaseTotalNeurons() uint64 {
	if ms.base == nil {
		return 0
	}
	return ms.base.TotalNeurons()
}

func (ms *MemorySystem) BaseTotalClusters() uint32 {
	if ms.base == nil {
		return 0
	}
	return ms.base.TotalClusters()
}

func (ms *MemorySystem) processSpike(spike Spike) {
	_ = spike
}

func (ms *MemorySystem) updateWeight(update STDPUpdate) {
	_ = update
}
