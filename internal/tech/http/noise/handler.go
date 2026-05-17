package noise

import (
	"encoding/binary"
	nethttp "net/http"

	"github.com/gin-gonic/gin"

	domainnoise "home-go/internal/domain/noise"
)

const bufferSize = 4096

// wavHeader is the 44-byte WAV preamble for a 44100 Hz mono 16-bit stream.
// Both the RIFF chunk size and the data chunk size are set to 0x7FFFFFFF
// (~13.5 hours at 44100 Hz) — safely above the 11-hour night window.
var wavHeader = [44]byte{
	'R', 'I', 'F', 'F',
	0xFF, 0xFF, 0xFF, 0x7F, // RIFF chunk size = 0x7FFFFFFF (LE)
	'W', 'A', 'V', 'E',
	'f', 'm', 't', ' ',
	16, 0, 0, 0, // fmt chunk size = 16
	1, 0,             // audio format: PCM
	1, 0,             // channels: mono
	0x44, 0xAC, 0, 0, // sample rate: 44100 Hz (LE)
	0x88, 0x58, 1, 0, // byte rate: 88200 (LE)
	2, 0,  // block align: 2 bytes
	16, 0, // bits per sample: 16
	'd', 'a', 't', 'a',
	0xFF, 0xFF, 0xFF, 0x7F, // data chunk size = 0x7FFFFFFF (LE)
}

// Handler streams infinite WAV noise.
type Handler struct{}

// ServeNoise handles GET /noise/:type.
// Writes a WAV header then streams PCM samples until the client disconnects.
func (h *Handler) ServeNoise(c *gin.Context) {
	noiseType := c.Param("type")
	gen, err := domainnoise.NewGenerator(noiseType)
	if err != nil {
		c.AbortWithStatus(nethttp.StatusNotFound)
		return
	}

	c.Header("Content-Type", "audio/wav")
	c.Header("Cache-Control", "no-cache")
	c.Header("Transfer-Encoding", "chunked")

	if _, err := c.Writer.Write(wavHeader[:]); err != nil {
		return
	}
	c.Writer.Flush()

	buf := make([]int16, bufferSize)
	raw := make([]byte, bufferSize*2)
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		gen.Fill(buf)
		for i, s := range buf {
			binary.LittleEndian.PutUint16(raw[i*2:], uint16(s))
		}
		if _, err := c.Writer.Write(raw); err != nil {
			return
		}
		c.Writer.Flush()
	}
}
