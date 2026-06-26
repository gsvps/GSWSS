package protocol

import (
	"bytes"
	"testing"
)

func TestConnectPayloadRoundTrip(t *testing.T) {
	original := ConnectPayload{
		Host:     "example.com",
		Port:     443,
		Password: "secret",
	}
	encoded := EncodeConnectPayload(original)
	decoded, err := DecodeConnectPayload(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Host != original.Host || decoded.Port != original.Port || decoded.Password != original.Password {
		t.Fatalf("round trip mismatch: %+v vs %+v", decoded, original)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	original := Frame{
		Version: Version,
		Type:    TypeData,
		Flags:   0,
		Payload: []byte("hello"),
	}
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeFrame(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Type != original.Type || !bytes.Equal(decoded.Payload, original.Payload) {
		t.Fatalf("frame mismatch: %+v vs %+v", decoded, original)
	}
}

func TestErrorPayloadRoundTrip(t *testing.T) {
	original := ErrorPayload{Code: ErrAuthFailed, Message: "bad password"}
	encoded := EncodeErrorPayload(original)
	decoded, err := DecodeErrorPayload(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Code != original.Code || decoded.Message != original.Message {
		t.Fatalf("error payload mismatch")
	}
}
