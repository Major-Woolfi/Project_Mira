# distutils: language = c++
# distutils: include_dirs = ../build/engine-memory-go
# distutils: library_dirs = ../build/engine-memory-go
# distutils: libraries = mira_memory
import os

from libc.stdint cimport uint8_t, uint16_t, uint32_t, uint64_t, int64_t
from libc.stdlib cimport free
from cpython.version cimport PY_MAJOR_VERSION

cdef extern from "mira_memory.h":
    ctypedef struct CMiraNeuron:
        uint32_t id
        uint16_t seed
        uint16_t cluster_id
        uint8_t type
        float threshold
        float resting_potential

    int MiraInit(const char* base_path, const char* delta_path)
    CMiraNeuron* MiraGetNeuron(uint32_t id)
    void MiraFreeNeuron(CMiraNeuron* p)
    void MiraClose()
    uint64_t MiraTotalNeurons()
    uint32_t MiraTotalClusters()
    int64_t MiraDeltaRecords()
    double MiraCacheHitRatio()
    int64_t MiraResidentMemoryMB()


cdef class MiraMemory:
    cdef bint _initialized

    def __cinit__(self, str base_path, str delta_path):
        base_bytes = base_path.encode('utf-8')
        delta_bytes = delta_path.encode('utf-8')

        result = MiraInit(base_bytes, delta_bytes)
        if result != 0:
            raise RuntimeError(
                f"Failed to initialize Mira Memory. "
                f"Check paths: {base_path}, {delta_path}"
            )
        self._initialized = True

    def get_neuron(self, uint32_t neuron_id):
        cdef CMiraNeuron* c_neuron = MiraGetNeuron(neuron_id)
        if c_neuron == NULL:
            raise ValueError(f"Neuron {neuron_id} not found")

        try:
            result = {
                'id': c_neuron.id,
                'seed': c_neuron.seed,
                'cluster_id': c_neuron.cluster_id,
                'type': c_neuron.type,
                'threshold': c_neuron.threshold,
                'resting_potential': c_neuron.resting_potential,
            }
            return result
        finally:
            MiraFreeNeuron(c_neuron)

    def get_stats(self):
        return {
            'total_neurons': MiraTotalNeurons(),
            'total_clusters': MiraTotalClusters(),
            'delta_records': MiraDeltaRecords(),
            'cache_hit_ratio': MiraCacheHitRatio(),
            'resident_memory_mb': MiraResidentMemoryMB(),
        }

    def close(self):
        if self._initialized:
            MiraClose()
            self._initialized = False

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
        return False

    def __dealloc__(self):
        self.close()
