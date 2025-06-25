package compressor

import (
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
)

const (
	// safeCompressionOverhead is the largest potential blow-up in bytes we expect to see when
	// compressing arbitrary (e.g. random) data.  Here we account for a 2 byte header, 4 byte
	// digest, 5 byte EOF indicator, and then 5 byte flate block header for each 16k of potential
	// data. Assuming frames are max 128k size (the current max blob size) this is 2+4+5+(5*8) = 51
	// bytes.  If we start using larger frames (e.g. should max blob size increase) a larger blowup
	// might be possible, but it would be highly unlikely, and the system still works if our
	// estimate is wrong -- we just end up writing one more tx for the overflow.
	safeCompressionOverhead = 51
)

type ShadowCompressor struct {
	config Config

	inputBuffer      []byte
	shadowCompressor derive.ChannelCompressor

	fullErr error

	bound uint64 // best known upperbound on the size of the compressed output
}

// NewShadowCompressor creates a new derive.Compressor implementation that
// uses an underlying compressor to perform size estimation and a cache of the input data.
// When the ShadowCompressor is flushed or closed, the underlying compressor is first reset and
// the input data written to it and then flushed.
// This means we get an optimal compression ratio while getting an accurate size estimation.
// The underlying compressor is flushed on every write, which means
// the final compressed data is always slightly smaller than the target. There is one
// exception to this rule: the first write to the buffer is not checked against the
// target, which allows individual blocks larger than the target to be included (and will
// be split across multiple channel frames).
func NewShadowCompressor(config Config) (derive.Compressor, error) {
	c := &ShadowCompressor{
		config: config,
	}

	var err error
	c.shadowCompressor, err = derive.NewChannelCompressor(config.CompressionAlgo)
	if err != nil {
		return nil, err
	}

	c.bound = safeCompressionOverhead
	return c, nil
}

func (t *ShadowCompressor) Write(p []byte) (int, error) {
	if t.fullErr != nil {
		return 0, t.fullErr
	}
	n, err := t.shadowCompressor.Write(p)
	if err != nil {
		return 0, err
	}
	newBound := t.bound + uint64(len(p))
	if newBound > t.config.TargetOutputSize {
		// Only flush the buffer if there's some chance we will be over the size limit.
		// This reduces CPU but more importantly it makes the shadow compression ratio more
		// closely reflect the ultimate compression ratio.
		if err = t.shadowCompressor.Flush(); err != nil {
			return 0, err
		}
		newBound = uint64(t.shadowCompressor.Len()) + CloseOverheadZlib
		if newBound > t.config.TargetOutputSize {
			t.fullErr = derive.ErrCompressorFull
			if len(t.inputBuffer) > 0 {
				// only return an error if we've already written data to this compressor before
				// (otherwise single blocks over the target would never be written)
				return 0, t.fullErr
			}
		}
	}
	t.bound = newBound
	t.inputBuffer = append(t.inputBuffer, p...)
	return n, nil
}

func (t *ShadowCompressor) Close() error {
	t.Flush()
	return t.shadowCompressor.Close()
}

func (t *ShadowCompressor) Read(p []byte) (int, error) {
	return t.shadowCompressor.Read(p)
}

func (t *ShadowCompressor) Reset() {
	t.inputBuffer = t.inputBuffer[:0]
	t.shadowCompressor.Reset()
	t.fullErr = nil
	t.bound = safeCompressionOverhead
}

func (t *ShadowCompressor) Len() int {
	return int(t.bound)
}

func (t *ShadowCompressor) Flush() error {
	t.shadowCompressor.Reset()
	t.shadowCompressor.Write(t.inputBuffer)
	t.inputBuffer = t.inputBuffer[:0]
	return t.shadowCompressor.Flush()
}

func (t *ShadowCompressor) FullErr() error {
	return t.fullErr
}
