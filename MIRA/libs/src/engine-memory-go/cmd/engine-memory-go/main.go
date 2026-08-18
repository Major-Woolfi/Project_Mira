package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/memory"
)

func main() {
	basePath := flag.String("base", "DATA/MIRA/base.mcog", "Path to base.mcog")
	deltaPath := flag.String("delta", "DATA/MIRA/delta.wal", "Path to delta.wal")
	fps := flag.Int("fps", 60, "Simulation FPS")
	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	ms := memory.NewMemorySystem(*basePath, *deltaPath, runtime.NumCPU())

	if err := ms.Open(); err != nil {
		log.Fatal(err)
	}
	defer ms.Close()

	ms.StartEventLoop()

	go ms.RunCompactionLoop()

	ticker := time.NewTicker(time.Duration(1000/int(*fps)) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		activeNeurons := ms.GetActiveNeurons()
		synapses := ms.GetSynapsesBatch(activeNeurons)

		spikes := ms.SimulateStep(synapses)

		ms.ApplySTDP(spikes)

		ms.LogStats()
	}
}
