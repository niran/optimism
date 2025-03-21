package derive

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

// Batch format
// first byte is type followed by bytestring.
//
// An empty input is not a valid batch.
//
// Note: the type system is based on L1 typed transactions.
//
// encodeBufferPool holds temporary encoder buffers for batch encoding
var encodeBufferPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

const (
	// SingularBatchType is the first version of Batch format, representing a single L2 block.
	SingularBatchType = 0
	// SpanBatchType is the Batch version used after Delta hard fork, representing a span of L2 blocks.
	SpanBatchType = 1
)

// Batch contains information to build one or multiple L2 blocks.
// Batcher converts L2 blocks into Batch and writes encoded bytes to Channel.
// Derivation pipeline decodes Batch from Channel, and converts to one or multiple payload attributes.
type Batch interface {
	GetBatchType() int
	GetTimestamp() uint64
	LogContext(log.Logger) log.Logger
	AsSingularBatch() (*SingularBatch, bool)
	AsSpanBatch() (*SpanBatch, bool)
}

type batchWithMetadata struct {
	Batch
	comprAlgo CompressionAlgo
}

func (b batchWithMetadata) LogContext(l log.Logger) log.Logger {
	lgr := b.Batch.LogContext(l)
	if b.comprAlgo == "" {
		return lgr
	}
	return lgr.With("compression_algo", b.comprAlgo)
}

type BatchDataOption func(*BatchData) *BatchData

func WithBatchTypeOverride(batchTypeOverride func(*RawSpanBatch) InnerBatchData) BatchDataOption {
	return func(b *BatchData) *BatchData {
		b.makeBatch = batchTypeOverride
		return b
	}
}

// BatchData is used to represent the typed encoding & decoding.
// and wraps around a single interface InnerBatchData.
// Further fields such as cache can be added in the future, without embedding each type of InnerBatchData.
// Similar design with op-geth's types.Transaction struct.
type BatchData struct {
	makeBatch func(*RawSpanBatch) InnerBatchData
	inner     InnerBatchData
	ComprAlgo CompressionAlgo
}

// InnerBatchData is the underlying data of a BatchData.
// This is implemented by SingularBatch and RawSpanBatch.
type InnerBatchData interface {
	GetBatchType() int
	Encode(w io.Writer) error
	Decode(r *bytes.Reader) error
}

// EncodeRLP implements rlp.Encoder
func (b *BatchData) EncodeRLP(w io.Writer) error {
	buf := encodeBufferPool.Get().(*bytes.Buffer)
	defer encodeBufferPool.Put(buf)
	buf.Reset()
	if err := b.encodeTyped(buf); err != nil {
		return err
	}
	return rlp.Encode(w, buf.Bytes())
}

func (bd *BatchData) GetBatchType() uint8 {
	return uint8(bd.inner.GetBatchType())
}

// MarshalBinary returns the canonical encoding of the batch.
func (b *BatchData) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	err := b.encodeTyped(&buf)
	return buf.Bytes(), err
}

// encodeTyped encodes batch type and payload for each batch type.
func (b *BatchData) encodeTyped(buf *bytes.Buffer) error {
	if err := buf.WriteByte(b.GetBatchType()); err != nil {
		return err
	}

	if b.makeBatch != nil {
		if sb, ok := b.inner.(*RawSpanBatch); ok {
			wrappedBatch := b.makeBatch(sb)
			return wrappedBatch.Encode(buf)
		}
	}

	return b.inner.Encode(buf)
}

// DecodeRLP implements rlp.Decoder
func (b *BatchData) DecodeRLP(s *rlp.Stream) error {
	if b == nil {
		return errors.New("cannot decode into nil BatchData")
	}
	v, err := s.Bytes()
	if err != nil {
		return err
	}
	return b.decodeTyped(v)
}

// UnmarshalBinary decodes the canonical encoding of batch.
func (b *BatchData) UnmarshalBinary(data []byte) error {
	if b == nil {
		return errors.New("cannot decode into nil BatchData")
	}
	return b.decodeTyped(data)
}

// decodeTyped decodes a typed batchData
func (b *BatchData) decodeTyped(data []byte) error {
	if len(data) == 0 {
		return errors.New("batch too short")
	}
	var inner InnerBatchData
	switch data[0] {
	case SingularBatchType:
		inner = new(SingularBatch)
	case SpanBatchType:
		inner = new(RawSpanBatch)
	default:
		return fmt.Errorf("unrecognized batch type: %d", data[0])
	}
	if err := inner.Decode(bytes.NewReader(data[1:])); err != nil {
		return err
	}
	b.inner = inner
	return nil
}

// NewBatchData creates a new BatchData
func NewBatchData(inner InnerBatchData, options ...BatchDataOption) *BatchData {
	d := &BatchData{inner: inner}

	for _, opt := range options {
		d = opt(d)
	}

	return d
}
