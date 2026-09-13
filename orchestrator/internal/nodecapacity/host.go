package nodecapacity

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HostMetrics is a point-in-time view of resources consumed by the whole host.
type HostMetrics struct {
	CPUUsage        float64
	CPUValid        bool
	MemoryUsed      int64
	MemoryAvailable int64
	MemoryValid     bool
	CollectedAt     time.Time
}

type cpuSample struct {
	total uint64
	idle  uint64
}

var cpuState struct {
	sync.Mutex
	previous cpuSample
	have     bool
}

// SampleHostMetrics reads Linux host counters. If the counters are unavailable,
// ok is false and callers should omit the observation rather than report zero.
func SampleHostMetrics() (metrics HostMetrics, ok bool) {
	current, cpuOK := readCPU()
	memoryUsed, memoryAvailable, memoryOK := readMemory()
	if !cpuOK && !memoryOK {
		return HostMetrics{}, false
	}

	metrics.CollectedAt = time.Now().UTC()
	if memoryOK {
		metrics.MemoryUsed = memoryUsed
		metrics.MemoryAvailable = memoryAvailable
		metrics.MemoryValid = true
	}

	if cpuOK {
		cpuState.Lock()
		if cpuState.have && current.total > cpuState.previous.total {
			totalDelta := current.total - cpuState.previous.total
			idleDelta := current.idle - cpuState.previous.idle
			if idleDelta <= totalDelta {
				metrics.CPUUsage = float64(totalDelta-idleDelta) / float64(totalDelta)
				metrics.CPUValid = true
			}
		}
		cpuState.previous = current
		cpuState.have = true
		cpuState.Unlock()
	}
	return metrics, true
}

func readCPU() (cpuSample, bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}
	line := strings.SplitN(string(raw), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4] // iowait is idle time from the scheduler's perspective.
	}
	return cpuSample{total: total, idle: idle}, true
}

func readMemory() (used, available int64, ok bool) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer func() { _ = file.Close() }()

	var total int64
	available = -1
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = value * 1024
		case "MemAvailable":
			available = value * 1024
		}
	}
	if total <= 0 || available < 0 {
		return 0, 0, false
	}
	if available > total {
		available = total
	}
	return total - available, available, true
}
