# Brown Noise Generator Design

**Date:** 2026-05-17
**File:** `internal/domain/noise/generator.go`

## Summary

Add a brown noise generator to the existing noise package, following the same patterns as `whiteGenerator` and `pinkGenerator`.

## Algorithm

Brown noise (red/Brownian noise) is produced by integrating white noise — each sample is the running sum of the previous sample and a new white-noise increment. The spectrum falls at 6 dB/octave (1/f²).

A leaky integer integrator prevents unbounded drift while preserving the spectral slope:

```
running[n] = running[n-1] + (white[n] >> 4) - (running[n-1] >> 7)
```

- `white >> 4` scales the input amplitude to ±2048 (from the ±32768 raw range).
- `running >> 7` is the leak term: attenuates by ~1/128 per sample, yielding a corner frequency of ~54 Hz at 44100 Hz sample rate.
- Steady-state standard deviation ≈ ±9500, well within `int16` range.
- A hard clamp to `[-32768, 32767]` is applied as a safety net (not a regular code path).

## Implementation

### New type

```go
// brownGenerator produces brown (red/Brownian) noise using a leaky integer integrator.
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
```

### `NewGenerator` changes

- Add `case "brown": return &brownGenerator{}, nil`
- Update error string: `"unknown noise type %q: want white, pink, or brown"`

### HTTP exposure

No handler changes needed. The existing `GET /noise/:type` route already routes the `:type` param through `NewGenerator`, so `GET /noise/brown` works automatically.

## Testing

Add `TestBrownGenerator_fill` in `generator_test.go` matching the existing "not all zeros" pattern used for white and pink tests.

## Acceptance Criteria

- `NewGenerator("brown")` returns a non-nil generator and nil error.
- `Fill` on a 4096-sample buffer produces at least one non-zero sample.
- `NewGenerator("ogg")` (unknown type) still returns an error mentioning all three valid types.
- `go test ./internal/domain/noise/...` passes.
