// Package protocol implements the GS Protocol (GSP1) frame encoding and decoding.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// Magic is the protocol magic number "GSP1" (0x47535031).
	Magic uint32 = 0x47535031
	// Version is the current protocol version.
	Version uint8 = 1

	// HeaderSize is the fixed size of a frame header in bytes.
	HeaderSize = 12
)

// Frame type constants.
const (
	TypeConnect uint8 = 1
	TypeData    uint8 = 2
	TypePing    uint8 = 3
	TypePong    uint8 = 4
	TypeClose   uint8 = 5
	TypeError   uint8 = 6
	TypeHTTPReq  uint8 = 9
	TypeHTTPResp uint8 = 10
)

// Error codes returned in ERROR frames.
const (
	ErrAuthFailed      uint16 = 1001
	ErrInvalidTarget   uint16 = 1002
	ErrConnectFailed   uint16 = 1003
	ErrRateLimited     uint16 = 1004
	ErrInvalidFrame    uint16 = 1005
	ErrInternal        uint16 = 1006
)

// Frame represents a GSP1 protocol frame.
type Frame struct {
	Version  uint8
	Type     uint8
	Flags    uint16
	StreamID uint32 // v2 only
	Payload  []byte
}

// ConnectPayload holds the CONNECT frame payload fields.
type ConnectPayload struct {
	Host     string
	Port     uint16
	Password string
}

// ErrorPayload holds the ERROR frame payload fields.
type ErrorPayload struct {
	Code    uint16
	Message string
}

var (
	// ErrInvalidMagic is returned when the magic number does not match.
	ErrInvalidMagic = errors.New("protocol: invalid magic")
	// ErrInvalidVersion is returned when the protocol version is unsupported.
	ErrInvalidVersion = errors.New("protocol: unsupported version")
	// ErrPayloadTooLarge is returned when payload exceeds the maximum size.
	ErrPayloadTooLarge = errors.New("protocol: payload too large")
)

// MaxPayloadSize is the maximum allowed payload size (16 MB).
const MaxPayloadSize = 16 * 1024 * 1024

// EncodeConnectPayload serializes a CONNECT payload.
func EncodeConnectPayload(p ConnectPayload) []byte {
	hostBytes := []byte(p.Host)
	passBytes := []byte(p.Password)
	buf := make([]byte, 2+len(hostBytes)+2+2+len(passBytes))
	off := 0
	off += putString(buf[off:], hostBytes)
	binary.BigEndian.PutUint16(buf[off:], p.Port)
	off += 2
	putString(buf[off:], passBytes)
	return buf
}

// DecodeConnectPayload deserializes a CONNECT payload.
func DecodeConnectPayload(data []byte) (ConnectPayload, error) {
	var p ConnectPayload
	off := 0
	host, n, err := getString(data, off)
	if err != nil {
		return p, fmt.Errorf("decode connect host: %w", err)
	}
	p.Host = host
	off += n
	if off+2 > len(data) {
		return p, errors.New("decode connect: missing port")
	}
	p.Port = binary.BigEndian.Uint16(data[off:])
	off += 2
	pass, _, err := getString(data, off)
	if err != nil {
		return p, fmt.Errorf("decode connect password: %w", err)
	}
	p.Password = pass
	return p, nil
}

// EncodeErrorPayload serializes an ERROR payload.
func EncodeErrorPayload(p ErrorPayload) []byte {
	msgBytes := []byte(p.Message)
	buf := make([]byte, 2+2+len(msgBytes))
	binary.BigEndian.PutUint16(buf[0:], p.Code)
	putString(buf[2:], msgBytes)
	return buf
}

// DecodeErrorPayload deserializes an ERROR payload.
func DecodeErrorPayload(data []byte) (ErrorPayload, error) {
	var p ErrorPayload
	if len(data) < 2 {
		return p, errors.New("decode error: payload too short")
	}
	p.Code = binary.BigEndian.Uint16(data[0:2])
	msg, _, err := getString(data, 2)
	if err != nil {
		return p, fmt.Errorf("decode error message: %w", err)
	}
	p.Message = msg
	return p, nil
}

// EncodeFrame serializes a frame into bytes.
func EncodeFrame(f Frame) ([]byte, error) {
	if len(f.Payload) > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	buf := make([]byte, HeaderSize+len(f.Payload))
	binary.BigEndian.PutUint32(buf[0:4], Magic)
	buf[4] = f.Version
	buf[5] = f.Type
	binary.BigEndian.PutUint16(buf[6:8], f.Flags)
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(f.Payload)))
	copy(buf[HeaderSize:], f.Payload)
	return buf, nil
}

// DecodeFrame reads and decodes a single frame from r.
func DecodeFrame(r io.Reader) (Frame, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Frame{}, err
	}
	magic := binary.BigEndian.Uint32(header[0:4])
	if magic != Magic {
		return Frame{}, ErrInvalidMagic
	}
	ver := header[4]
	if ver != Version {
		return Frame{}, ErrInvalidVersion
	}
	length := binary.BigEndian.Uint32(header[8:12])
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
		Version: ver,
		Type:    header[5],
		Flags:   binary.BigEndian.Uint16(header[6:8]),
		Payload: payload,
	}, nil
}

func putString(buf []byte, s []byte) int {
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(s)))
	copy(buf[2:], s)
	return 2 + len(s)
}

func getString(data []byte, off int) (string, int, error) {
	if off+2 > len(data) {
		return "", 0, errors.New("missing string length")
	}
	length := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	if off+length > len(data) {
		return "", 0, errors.New("string length exceeds payload")
	}
	return string(data[off : off+length]), 2 + length, nil
}
