package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// Version2 enables stream multiplexing over a single WebSocket.
	Version2 uint8 = 2

	// HeaderSizeV2 is the v2 frame header size (includes StreamID).
	HeaderSizeV2 = 16

	TypeSession   uint8 = 7
	TypeSessionOK uint8 = 8
)

// EncodeTargetPayload serializes host+port for v2 CONNECT (password is session-scoped).
func EncodeTargetPayload(host string, port uint16) []byte {
	hostBytes := []byte(host)
	buf := make([]byte, 2+len(hostBytes)+2)
	off := putString(buf, hostBytes)
	binary.BigEndian.PutUint16(buf[off:], port)
	return buf
}

// DecodeTargetPayload deserializes v2 CONNECT payload.
func DecodeTargetPayload(data []byte) (host string, port uint16, err error) {
	off := 0
	host, n, err := getString(data, off)
	if err != nil {
		return "", 0, fmt.Errorf("decode target host: %w", err)
	}
	off += n
	if off+2 > len(data) {
		return "", 0, errors.New("decode target: missing port")
	}
	port = binary.BigEndian.Uint16(data[off:])
	return host, port, nil
}

// EncodeSessionPayload serializes a SESSION frame payload (password only).
func EncodeSessionPayload(password string) []byte {
	return putStringOnly([]byte(password))
}

// DecodeSessionPayload deserializes a SESSION frame payload.
func DecodeSessionPayload(data []byte) (string, error) {
	pass, _, err := getString(data, 0)
	return pass, err
}

func putStringOnly(s []byte) []byte {
	buf := make([]byte, 2+len(s))
	putString(buf, s)
	return buf
}

// EncodeFrameV2 serializes a v2 frame.
func EncodeFrameV2(f Frame) ([]byte, error) {
	if f.Version != Version2 {
		f.Version = Version2
	}
	if len(f.Payload) > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	buf := make([]byte, HeaderSizeV2+len(f.Payload))
	binary.BigEndian.PutUint32(buf[0:4], Magic)
	buf[4] = Version2
	buf[5] = f.Type
	binary.BigEndian.PutUint32(buf[6:10], f.StreamID)
	binary.BigEndian.PutUint32(buf[10:14], uint32(len(f.Payload)))
	copy(buf[HeaderSizeV2:], f.Payload)
	return buf, nil
}

// DecodeFrameV2 reads a v2 frame from r.
func DecodeFrameV2(r io.Reader) (Frame, error) {
	header := make([]byte, HeaderSizeV2)
	if _, err := io.ReadFull(r, header); err != nil {
		return Frame{}, err
	}
	magic := binary.BigEndian.Uint32(header[0:4])
	if magic != Magic {
		return Frame{}, ErrInvalidMagic
	}
	if header[4] != Version2 {
		return Frame{}, ErrInvalidVersion
	}
	length := binary.BigEndian.Uint32(header[10:14])
	if length > MaxPayloadSize {
		return Frame{}, ErrPayloadTooLarge
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return Frame{}, err
		}
	}
	return Frame{
		Version:  Version2,
		Type:     header[5],
		StreamID: binary.BigEndian.Uint32(header[6:10]),
		Payload:  payload,
	}, nil
}

// DecodeFrameAny reads v1 or v2 based on the version byte after magic.
func DecodeFrameAny(data []byte) (Frame, error) {
	if len(data) < 5 {
		return Frame{}, errors.New("frame too short")
	}
	if binary.BigEndian.Uint32(data[0:4]) != Magic {
		return Frame{}, ErrInvalidMagic
	}
	switch data[4] {
	case Version:
		return DecodeFrame(newBytesReaderFrom(data))
	case Version2:
		return DecodeFrameV2(newBytesReaderFrom(data))
	default:
		return Frame{}, ErrInvalidVersion
	}
}

func newBytesReaderFrom(data []byte) *sliceReader {
	return &sliceReader{data: data}
}

type sliceReader struct {
	data []byte
	off  int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}
