package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// HTTPReqPayload is the HTTP_REQ frame body.
type HTTPReqPayload struct {
	Password string
	Method   string
	URL      string
	Headers  [][2]string
	Body     []byte
}

// HTTPRespPayload is the HTTP_RESP frame body.
type HTTPRespPayload struct {
	Status  uint16
	Headers [][2]string
	Body    []byte
}

// EncodeHTTPReqPayload serializes an HTTP request for worker fetch.
func EncodeHTTPReqPayload(p HTTPReqPayload) []byte {
	headerBytes := encodeHeaderPairs(p.Headers)
	bodyLen := len(p.Body)
	buf := make([]byte, 0, 256+bodyLen+len(headerBytes))
	buf = appendString(buf, p.Password)
	buf = appendString(buf, p.Method)
	buf = appendString(buf, p.URL)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(p.Headers)))
	buf = append(buf, headerBytes...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(bodyLen))
	if bodyLen > 0 {
		buf = append(buf, p.Body...)
	}
	return buf
}

// DecodeHTTPReqPayload deserializes an HTTP_REQ payload.
func DecodeHTTPReqPayload(data []byte) (HTTPReqPayload, error) {
	var p HTTPReqPayload
	off := 0
	var n int
	var err error

	if p.Password, n, err = readString(data, off); err != nil {
		return p, fmt.Errorf("decode http req password: %w", err)
	}
	off += n
	if p.Method, n, err = readString(data, off); err != nil {
		return p, fmt.Errorf("decode http req method: %w", err)
	}
	off += n
	if p.URL, n, err = readString(data, off); err != nil {
		return p, fmt.Errorf("decode http req url: %w", err)
	}
	off += n
	if off+2 > len(data) {
		return p, errors.New("decode http req: missing header count")
	}
	headerCount := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	if p.Headers, n, err = readHeaderPairs(data, off, headerCount); err != nil {
		return p, fmt.Errorf("decode http req headers: %w", err)
	}
	off += n
	if off+4 > len(data) {
		return p, errors.New("decode http req: missing body length")
	}
	bodyLen := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	if bodyLen < 0 || off+bodyLen > len(data) {
		return p, errors.New("decode http req: invalid body length")
	}
	if bodyLen > 0 {
		p.Body = make([]byte, bodyLen)
		copy(p.Body, data[off:off+bodyLen])
	}
	return p, nil
}

// EncodeHTTPRespPayload serializes an HTTP response from worker fetch.
func EncodeHTTPRespPayload(p HTTPRespPayload) []byte {
	headerBytes := encodeHeaderPairs(p.Headers)
	bodyLen := len(p.Body)
	buf := make([]byte, 2+2+len(headerBytes)+4+bodyLen)
	binary.BigEndian.PutUint16(buf[0:2], p.Status)
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(p.Headers)))
	copy(buf[4:], headerBytes)
	off := 4 + len(headerBytes)
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(bodyLen))
	off += 4
	if bodyLen > 0 {
		copy(buf[off:], p.Body)
	}
	return buf
}

// DecodeHTTPRespPayload deserializes an HTTP_RESP payload.
func DecodeHTTPRespPayload(data []byte) (HTTPRespPayload, error) {
	var p HTTPRespPayload
	if len(data) < 4 {
		return p, errors.New("decode http resp: payload too short")
	}
	p.Status = binary.BigEndian.Uint16(data[0:2])
	headerCount := int(binary.BigEndian.Uint16(data[2:4]))
	off := 4
	var n int
	var err error
	if p.Headers, n, err = readHeaderPairs(data, off, headerCount); err != nil {
		return p, fmt.Errorf("decode http resp headers: %w", err)
	}
	off += n
	if off+4 > len(data) {
		return p, errors.New("decode http resp: missing body length")
	}
	bodyLen := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	if bodyLen < 0 || off+bodyLen > len(data) {
		return p, errors.New("decode http resp: invalid body length")
	}
	if bodyLen > 0 {
		p.Body = make([]byte, bodyLen)
		copy(p.Body, data[off:off+bodyLen])
	}
	return p, nil
}

func appendString(buf []byte, s string) []byte {
	b := []byte(s)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(b)))
	return append(buf, b...)
}

func readString(data []byte, off int) (string, int, error) {
	if off+2 > len(data) {
		return "", 0, errors.New("missing string length")
	}
	length := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	if off+length > len(data) {
		return "", 0, errors.New("string exceeds payload")
	}
	return string(data[off : off+length]), 2 + length, nil
}

func encodeHeaderPairs(headers [][2]string) []byte {
	var buf []byte
	for _, h := range headers {
		buf = appendString(buf, h[0])
		buf = appendString(buf, h[1])
	}
	return buf
}

func readHeaderPairs(data []byte, off int, count int) ([][2]string, int, error) {
	start := off
	headers := make([][2]string, 0, count)
	for i := 0; i < count; i++ {
		key, n, err := readString(data, off)
		if err != nil {
			return nil, 0, err
		}
		off += n
		val, n, err := readString(data, off)
		if err != nil {
			return nil, 0, err
		}
		off += n
		headers = append(headers, [2]string{key, val})
	}
	return headers, off - start, nil
}
