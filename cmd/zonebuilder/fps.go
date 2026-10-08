package main

import (
	"log"
	"runtime"
	"strings"
	"time"

	"zonebuilder/internal/render"
)

// frameLogEvery is how often the -fps flag logs the frame times.
const frameLogEvery = 2 * time.Second

// frameLog measures the frame times for the -fps flag: the interval
// between frames, which with continuous redraw and no vsync is what one
// frame costs, logged with what the frames drew.
type frameLog struct {
	start, last time.Time
	frames      int
	worst       time.Duration
	sum         render.DrawStats
}

// frame records a frame drawn at now with stats, the tiles shown.
func (f *frameLog) frame(now time.Time, stats render.DrawStats, tiles []string) {
	if f.last.IsZero() {
		f.start, f.last = now, now
		return
	}
	f.worst = max(f.worst, now.Sub(f.last))
	f.last = now
	f.frames++
	f.sum.Draws += stats.Draws
	f.sum.Triangles += stats.Triangles
	f.sum.Sectors += stats.Sectors
	f.sum.Culled += stats.Culled
	span := now.Sub(f.start)
	if span < frameLogEvery {
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	n := f.frames
	log.Printf("fps: %.1f quadros/s em %v: quadro médio %.2f ms, pior %.2f ms; por quadro %d draws, %d triângulos, %d de %d setores fora da vista; tiles [%s]; heap Go %d MB",
		float64(n)/span.Seconds(), span.Round(time.Millisecond),
		float64(span.Microseconds())/float64(n)/1000, float64(f.worst.Microseconds())/1000,
		f.sum.Draws/n, f.sum.Triangles/n, f.sum.Culled/n, f.sum.Sectors/n,
		strings.Join(tiles, " "), mem.HeapInuse>>20)
	*f = frameLog{start: now, last: now}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
