package noise

import (
	"fmt"
	"math/bits"
	"math/rand"
)

// Generator fills a buffer with audio samples in the range [-32768, 32767].
type Generator interface {
	Fill(buf []int16)
}

// NewGenerator returns a white or pink noise generator.
// Returns an error for unknown types.
func NewGenerator(noiseType string) (Generator, error) {
	switch noiseType {
	case "white":
		return &whiteGenerator{}, nil
	case "pink":
		return &pinkGenerator{}, nil
	case "brown":
		return &brownGenerator{}, nil
	default:
		return nil, fmt.Errorf("unknown noise type %q: want white, pink, or brown", noiseType)
	}
}

// whiteGenerator produces uniformly distributed random samples. Stateless.
type whiteGenerator struct{}

func (g *whiteGenerator) Fill(buf []int16) {
	for i := range buf {
		buf[i] = int16(rand.Int63n(65536) - 32768)
	}
}

// brownGenerator produces brown (red/Brownian) noise using a leaky integer integrator.
// Each sample accumulates a scaled white-noise increment; a per-sample leak term
// prevents unbounded drift while preserving the 1/f² spectral slope.
// Not safe for concurrent use; each goroutine must own its own instance.
type brownGenerator struct {
	running int32
}

func (g *brownGenerator) Fill(buf []int16) {
	for i := range buf {
		white := int32(rand.Int63n(65536)) - 32768
		g.running += white >> 4
		g.running -= g.running >> 7
		if g.running > 32767 {
			g.running = 32767
		} else if g.running < -32768 {
			g.running = -32768
		}
		buf[i] = int16(g.running)
	}
}

// pinkGenerator produces pink (1/f) noise using the Voss-McCartney algorithm.
// 16 rows of running sums; one row updated per sample based on the trailing
// zeros of an incrementing counter; sum divided by 17 to stay in int16 range.
// Not safe for concurrent use; each goroutine must own its own instance.
type pinkGenerator struct {
	rows    [16]int32
	running int32
	counter uint64
}

func (g *pinkGenerator) Fill(buf []int16) {
	for i := range buf {
		g.counter++
		idx := bits.TrailingZeros64(g.counter) % 16
		white := int32(rand.Int63n(65536)) - 32768
		g.running += white - g.rows[idx]
		g.rows[idx] = white
		extra := int32(rand.Int63n(65536)) - 32768
		buf[i] = int16((g.running + extra) / 17)
	}
}
