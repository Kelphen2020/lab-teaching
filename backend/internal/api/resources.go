package api

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SystemResources(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// disk
	var diskTotal, diskFree uint64
	var s syscall.Statfs_t
	if err := syscall.Statfs("/", &s); err == nil {
		diskTotal = s.Blocks * uint64(s.Bsize)
		diskFree = s.Bfree * uint64(s.Bsize)
	}

	// CPU load
	load1, load5, load15 := readLoadAvg()

	// DB pool
	dbStats := h.Repo.SQLDB().Stats()

	response.Success(c, gin.H{
		"memory": gin.H{
			"alloc_mb":       round1(float64(mem.Alloc) / 1024 / 1024),
			"total_alloc_mb": round1(float64(mem.TotalAlloc) / 1024 / 1024),
			"sys_mb":         round1(float64(mem.Sys) / 1024 / 1024),
			"gc_runs":        mem.NumGC,
		},
		"disk": gin.H{
			"total_gb": round1(float64(diskTotal) / 1024 / 1024 / 1024),
			"free_gb":  round1(float64(diskFree) / 1024 / 1024 / 1024),
			"used_pct": round1(diskUsedPct(diskTotal, diskFree)),
		},
		"cpu": gin.H{
			"load1":  load1,
			"load5":  load5,
			"load15": load15,
			"cores":  runtime.NumCPU(),
		},
		"db_pool": gin.H{
			"open_conns":      dbStats.OpenConnections,
			"in_use":          dbStats.InUse,
			"idle":            dbStats.Idle,
			"wait_count":      dbStats.WaitCount,
			"max_idle_closed": dbStats.MaxIdleClosed,
		},
	})
}

func readLoadAvg() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return 0, 0, 0
	}
	l1, _ := strconv.ParseFloat(parts[0], 64)
	l5, _ := strconv.ParseFloat(parts[1], 64)
	l15, _ := strconv.ParseFloat(parts[2], 64)
	return round1(l1), round1(l5), round1(l15)
}

func diskUsedPct(total, free uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(total-free) / float64(total) * 100
}

func round1(v float64) float64 {
	return float64(int(v*10)) / 10
}