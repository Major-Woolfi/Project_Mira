package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
    uint32_t id;
    uint16_t seed;
    uint16_t cluster_id;
    uint8_t  type;
    float    threshold;
    float    resting_potential;
} CMiraNeuron;
*/
import "C"
import (
	"unsafe"

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/memory"
)

var globalMS *memory.MemorySystem

//export MiraInit
func MiraInit(basePath *C.char, deltaPath *C.char) C.int {
	base := C.GoString(basePath)
	delta := C.GoString(deltaPath)
	globalMS = memory.NewMemorySystem(base, delta, 4)
	if err := globalMS.Open(); err != nil {
		return -1
	}
	return 0
}

//export MiraGetNeuron
func MiraGetNeuron(id C.uint32_t) *C.CMiraNeuron {
	if globalMS == nil {
		return nil
	}
	n := globalMS.GetNeuron(uint32(id))
	cNeuron := (*C.CMiraNeuron)(C.malloc(C.sizeof_CMiraNeuron))
	cNeuron.id = C.uint32_t(n.ID)
	cNeuron.seed = C.uint16(n.Seed)
	cNeuron.cluster_id = C.uint16(n.ClusterID)
	cNeuron.type = C.uint8(n.Type)
	cNeuron.threshold = C.float(n.Threshold)
	cNeuron.resting_potential = C.float(n.RestingPotential)
	return cNeuron
}

//export MiraFreeNeuron
func MiraFreeNeuron(p *C.CMiraNeuron) {
	C.free(unsafe.Pointer(p))
}

//export MiraClose
func MiraClose() {
	if globalMS != nil {
		globalMS.Close()
		globalMS = nil
	}
}

//export MiraTotalNeurons
func MiraTotalNeurons() C.uint64_t {
	if globalMS == nil {
		return 0
	}
	return C.uint64_t(globalMS.BaseTotalNeurons())
}

//export MiraTotalClusters
func MiraTotalClusters() C.uint32_t {
	if globalMS == nil {
		return 0
	}
	return C.uint32_t(globalMS.BaseTotalClusters())
}

//export MiraDeltaRecords
func MiraDeltaRecords() C.int64_t {
	if globalMS == nil {
		return 0
	}
	return C.int64_t(globalMS.DeltaRecordCount())
}

//export MiraCacheHitRatio
func MiraCacheHitRatio() C.double {
	if globalMS == nil {
		return 0.0
	}
	return C.double(globalMS.CacheHitRatio())
}

//export MiraResidentMemoryMB
func MiraResidentMemoryMB() C.int64_t {
	if globalMS == nil {
		return 0
	}
	return C.int64_t(globalMS.GetResidentMemoryMB())
}

func main() {}
