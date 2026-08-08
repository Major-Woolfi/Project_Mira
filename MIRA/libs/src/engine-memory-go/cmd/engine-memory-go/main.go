package main

import (
	"log"
	"runtime"

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/memory"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	ms := memory.NewMemorySystem(
		"data/memory/base.mcog",
		"data/memory/delta.wal",
		runtime.NumCPU(),
	)

	if err := ms.Open(); err != nil {
		log.Fatal(err)
	}
	defer ms.Close()

	ms.StartEventLoop()

	go ms.RunCompactionLoop()

	for {
		activeNeurons := ms.GetActiveNeurons()
		synapses := ms.GetSynapsesBatch(activeNeurons)

		spikes := ms.SimulateStep(synapses)

		ms.ApplySTDP(spikes)

		ms.LogStats()
	}
}
