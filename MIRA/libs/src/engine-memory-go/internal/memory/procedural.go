package memory

import (

	"github.com/Major-Woolfi/Project_Mira/libs/src/engine-memory-go/internal/prng"
)

func hashCombine(seed uint16, targetID uint32) uint32 {
	return uint32(seed)*uint32(0x9E3779B9) + targetID
}

func GenerateProceduralSynapses(base *BaseReader, neuronID uint32) []Synapse {
	seed := base.GetSeed(uint64(neuronID))
	clusterID := base.GetClusterID(uint64(neuronID))

	rng := prng.NewPCG32(uint64(seed))

	var synapses []Synapse
	connectedClusters := getConnectedClusters(base, uint16(clusterID))

	for _, targetCluster := range connectedClusters {
		probability := base.GetProbability(uint16(clusterID), targetCluster)
		if probability == 0.0 {
			continue
		}

		clusterMeta := base.GetClusterMeta(targetCluster)
		if clusterMeta == nil {
			continue
		}

		targetNeurons := clusterMeta.NeuronCount
		for i := uint32(0); i < targetNeurons; i++ {
			if rng.NextFloat32() < probability {
				targetID := clusterMeta.FirstNeuronID + i

				weightIdx := uint8(hashCombine(seed, targetID) % 256)

				synapses = append(synapses, Synapse{
					TargetID:  targetID,
					WeightIdx: weightIdx,
					Delay:     uint8(rng.Next() % 16),
					Type:      uint8(rng.Next() % 4),
				})
			}
		}
	}

	return synapses
}

func getConnectedClusters(base *BaseReader, clusterID uint16) []uint16 {
	var connected []uint16
	for _, rule := range base.rules {
		if rule.SourceCluster == clusterID {
			connected = append(connected, rule.TargetCluster)
		}
	}
	return connected
}
