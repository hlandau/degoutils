// Package bsda provides message framing on top of ordered reliable byte
// streams.
//
// The format is as follows:
//
//   Byte Stream Datagram Adaptation No. 1 (BSDA-1)
//   ==============================================
//
//   Stream:
//     Zero or more frames
//
//   Frame:
//     4  ui  Data Length
//   ...      Data
//
//   That is, each frame is a unsigned 32-bit integer indicating
//   the data length, followed by that many bytes of data.
//
//   All fields are little endian.
//
package bsda

import "io"
import "fmt"
import "sync"
import "sync/atomic"
import "encoding/binary"

type FrameReader interface {
	ReadFrame() ([]byte, error)
}

type FrameWriter interface {
	WriteFrame([]byte) error
}

type FrameReadWriter interface {
	FrameReader
	FrameWriter
}

// Bidirectional BSDA message stream.
type Stream struct {
	// The maximum frame size which may be received. An error is returned
	// if an oversize frame is received. May be changed at any time.
	// Defaults to 32ki.
	maxRxFrameSize uint32

	writeMutex  sync.Mutex
	reader      io.Reader
	writer      io.Writer
	oversizeLen uint32
}

var ErrOversizeFrame = fmt.Errorf("received frame in excess of permitted size")
var ErrUnidirectional = fmt.Errorf("unidirectional stream")

// Instantiates a new bidirectional BSDA message stream which provides framing
// on top of an underlying bytestream.
func New(stream io.ReadWriter) *Stream {
	return create(stream, stream)
}

// Instantiates a new unidirectional BSDA message stream which provides framing
// on top of an underlying bytestream.
func NewReader(reader io.Reader) *Stream {
	return create(reader, nil)
}

// Instantiates a new unidirectional BSDA message stream which provides framing
// on top of an underlying bytestream.
func NewWriter(writer io.Writer) *Stream {
	return create(nil, writer)
}

func create(reader io.Reader, writer io.Writer) *Stream {
	s := &Stream{
		reader:         reader,
		writer:         writer,
		maxRxFrameSize: 32 * 1024,
	}

	return s
}

var scratch [8192]byte

// Read a single frame. Underlying I/O errors are passed through.
//
// Returns ErrOversizeFrame if the received frame exceeded MaxRxFrameSize.
// Calling this method again will skip such a frame's body and receive the next
// frame.
//
// Do not call this method concurrently.
func (s *Stream) ReadFrame() ([]byte, error) {
	if s.reader == nil {
		return nil, ErrUnidirectional
	}

	// If this method has been called even after reading an
	// oversize frame, then the caller must want to ignore the oversize
	// frame and continue processing the stream.
	for s.oversizeLen > 0 {
		l := s.oversizeLen
		if l > uint32(len(scratch)) {
			l = uint32(len(scratch))
		}
		_, err := io.ReadFull(s.reader, scratch[0:l])
		if err != nil {
			return nil, err
		}
		s.oversizeLen -= l
	}

	// Read the frame header.
	var header [4]byte
	_, err := io.ReadFull(s.reader, header[:])
	if err != nil {
		return nil, err
	}

	L := binary.LittleEndian.Uint32(header[:])
	maxRx := atomic.LoadUint32(&s.maxRxFrameSize)
	if L > maxRx {
		s.oversizeLen = L
		return nil, ErrOversizeFrame
	}

	// Read the frame data.
	buf := make([]byte, L)
	_, err = io.ReadFull(s.reader, buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// Write a single frame. Underlying I/O errors are passed through.
//
// Unlike ReadFrame, this method may be called concurrently.
func (s *Stream) WriteFrame(buf []byte) error {
	if s.writer == nil {
		return ErrUnidirectional
	}

	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(buf)))

	_, err := s.writer.Write(header[:])
	if err != nil {
		return err
	}

	_, err = s.writer.Write(buf)
	return err
}

// Set the maximum frame receive size in bytes.
//
// Defaults to 32ki.
func (s *Stream) SetMaxReadSize(sz int) {
	atomic.StoreUint32(&s.maxRxFrameSize, uint32(sz))
}
