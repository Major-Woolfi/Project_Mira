package memory

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

type CompactionConfig struct {
	DeltaSizeThreshold int64
	FragmentationThreshold float64
	CheckInterval time.Duration
}

func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		DeltaSizeThreshold:      100 * 1024 * 1024 * 1024,
		FragmentationThreshold:  0.3,
		CheckInterval:           time.Hour,
	}
}

type Compactor struct {
	basePath    string
	deltaPath   string
	config      CompactionConfig
	mu          sync.Mutex
	running     bool
	lastRun     time.Time
}

func NewCompactor(basePath, deltaPath string, config CompactionConfig) *Compactor {
	return &Compactor{
		basePath:  basePath,
		deltaPath: deltaPath,
		config:    config,
	}
}

func (c *Compactor) Start() {
	go c.loop()
}

func (c *Compactor) loop() {
	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		if c.shouldCompact() {
			if err := c.Run(); err != nil {
				fmt.Printf("compaction error: %v\n", err)
			}
		}
	}
}

func (c *Compactor) shouldCompact() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return false
	}

	info, err := os.Stat(c.deltaPath)
	if err != nil {
		return false
	}

	if info.Size() > c.config.DeltaSizeThreshold {
		return true
	}

	if time.Since(c.lastRun) > 24*time.Hour {
		return true
	}

	return false
}

func (c *Compactor) Run() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("compaction already running")
	}
	c.running = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.running = false
		c.lastRun = time.Now()
		c.mu.Unlock()
	}()

	tmpBase := c.basePath + ".new"

	base, err := NewBaseReader(c.basePath)
	if err != nil {
		return err
	}
	defer base.Close()

	delta, err := NewDeltaReader(c.deltaPath)
	if err != nil {
		return err
	}
	defer delta.Close()

	out, err := os.OpenFile(tmpBase, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	header := base.header
	header.Checksum = 0
	headerBytes := make([]byte, 64)
	copy(headerBytes[0:4], header.Magic[:])
	binary.LittleEndian.PutUint32(headerBytes[4:8], header.Version)
	binary.LittleEndian.PutUint64(headerBytes[8:16], header.TotalNeurons)
	binary.LittleEndian.PutUint32(headerBytes[16:20], header.TotalClusters)
	binary.LittleEndian.PutUint32(headerBytes[20:24], header.SectionCount)
	binary.LittleEndian.PutUint64(headerBytes[24:32], header.CodebookOffset)
	binary.LittleEndian.PutUint64(headerBytes[32:40], header.NeuronsOffset)
	binary.LittleEndian.PutUint64(headerBytes[40:48], header.RulesOffset)
	binary.LittleEndian.PutUint64(headerBytes[48:56], header.ClustersOffset)

	if _, err := out.Write(headerBytes); err != nil {
		return err
	}

	codebookBytes := make([]byte, 256*4)
	for i, v := range base.codebook {
		binary.LittleEndian.PutUint32(codebookBytes[i*4:], math.Float32bits(v))
	}
	if _, err := out.Write(codebookBytes); err != nil {
		return err
	}

	neuronBytes := make([]byte, int(header.TotalNeurons)*6)
	if _, err := out.Write(neuronBytes); err != nil {
		return err
	}

	_ = base
	_ = delta

	return os.Rename(tmpBase, c.basePath)
}

func (c *Compactor) Stop() {
	// Graceful stop
}
