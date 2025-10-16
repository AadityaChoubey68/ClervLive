package buffer

import (
	"runtime"
	"sync/atomic"
	"time"
)

const (
	MinBifferSize = 100
	MaxBufferSize = 1000
	ReCalcTime    = 5 * time.Second
	AvgMessSize   = 1024
)

type AddaptiveBufferManager struct {
	maxTotalMemory int64
	suncriberCount atomic.Int32
	bufferSize     atomic.Int32
	stopChan       chan struct{}
}

func NewAdaptiveBufferManager(maxMemort int64) *AddaptiveBufferManager {
	adm := &AddaptiveBufferManager{
		maxTotalMemory: maxMemort,
		stopChan:       make(chan struct{}),
	}

	adm.bufferSize.Store(MaxBufferSize)
	return adm
}

func (adm *AddaptiveBufferManager) Start() {
	go adm.monitorLoop()
}

func (adm *AddaptiveBufferManager) monitorLoop() {
	ticker := time.NewTicker(ReCalcTime)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			adm.recalculate()
		case <-adm.stopChan:
			return
		}
	}
}

func (adm *AddaptiveBufferManager) recalculate() {
	subCount := adm.suncriberCount.Load()

	if subCount == 0 {
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentMomory := int64(m.Alloc)

	availableMemory := adm.maxTotalMemory - currentMomory
	if availableMemory < 0 {
		availableMemory = 0
	}

	memoryPerSub := availableMemory / int64(subCount)
	bufferPerSub := int32(memoryPerSub / AvgMessSize)

	if bufferPerSub < MinBifferSize {
		bufferPerSub = MinBifferSize
	}
	if bufferPerSub > MaxBufferSize {
		bufferPerSub = MaxBufferSize
	}

	adm.bufferSize.Store(bufferPerSub)
}

func (adm *AddaptiveBufferManager) GetBufferSize() int {
	return int(adm.bufferSize.Load())
}

func (adm *AddaptiveBufferManager) AddNewSubscriber() {
	adm.suncriberCount.Add(1)
}

func (adm *AddaptiveBufferManager) OnSubscriberRemoval() {
	adm.suncriberCount.Add(-1)
}

func (adm *AddaptiveBufferManager) Stop() {
	close(adm.stopChan)
}
