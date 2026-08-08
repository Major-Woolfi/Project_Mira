package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/memory"
	"github.com/prometheus/client_golang/prometheus"
)

type Server struct {
	mem *memory.MemorySystem
}

func NewServer(mem *memory.MemorySystem) *Server {
	return &Server{mem: mem}
}

func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"neurons_total":      s.mem.BaseTotalNeurons(),
		"clusters_total":     s.mem.BaseTotalClusters(),
		"delta_records":      s.mem.DeltaRecordCount(),
		"delta_size_mb":      s.mem.DeltaFileSize() / (1024 * 1024),
		"cache_size_mb":      s.mem.CacheMemoryUsage() / (1024 * 1024),
		"cache_hit_ratio":    s.mem.CacheHitRatio(),
		"last_compaction":    s.mem.LastCompactionTime(),
		"mmap_resident_mb":   s.mem.GetResidentMemoryMB(),
		"active_neurons_pct": s.mem.GetActiveNeuronsPercent(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) NeuronHandler(w http.ResponseWriter, r *http.Request) {
	id := parseIDFromURL(r)
	neuron := s.mem.GetNeuron(id)
	synapses := s.mem.GetSynapses(id)

	response := map[string]interface{}{
		"neuron":         neuron,
		"synapses_count": len(synapses),
		"synapses":       synapses[:min(100, len(synapses))],
		"procedural":     s.mem.CountProceduralSynapses(id),
		"delta_modified": s.mem.CountDeltaModifications(id),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) CompactHandler(w http.ResponseWriter, r *http.Request) {
	go s.mem.RunCompaction()
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Compaction started"))
}

func parseIDFromURL(r *http.Request) uint32 {
	path := r.URL.Path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			id, err := strconv.ParseUint(path[i+1:], 10, 32)
			if err != nil {
				return 0
			}
			return uint32(id)
		}
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Metrics struct {
	NeuronsTotal        prometheus.Gauge
	SynapsesGenerated   prometheus.Counter
	DeltaRecordsWritten prometheus.Counter
	CacheHitRatio       prometheus.Gauge
	MmapPageFaults      prometheus.Counter
	CompactionDuration  prometheus.Histogram
}

func NewMetrics() *Metrics {
	return &Metrics{
		NeuronsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mira_neurons_total",
			Help: "Общее количество нейронов в системе",
		}),
		SynapsesGenerated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mira_synapses_generated_total",
			Help: "Количество процедурно сгенерированных синапсов",
		}),
		DeltaRecordsWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mira_delta_records_written_total",
			Help: "Количество записей в Delta-лог",
		}),
		CacheHitRatio: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mira_cache_hit_ratio",
			Help: "Процент попаданий в procedural cache",
		}),
		MmapPageFaults: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mira_mmap_page_faults_total",
			Help: "Количество page faults при mmap-доступе",
		}),
		CompactionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "mira_compaction_duration_seconds",
			Help:    "Время выполнения compaction",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		}),
	}
}

func (m *Metrics) Register() {
	prometheus.MustRegister(m.NeuronsTotal)
	prometheus.MustRegister(m.SynapsesGenerated)
	prometheus.MustRegister(m.DeltaRecordsWritten)
	prometheus.MustRegister(m.CacheHitRatio)
	prometheus.MustRegister(m.MmapPageFaults)
	prometheus.MustRegister(m.CompactionDuration)
}
