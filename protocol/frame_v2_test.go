package protocol

import (
	"bytes"
	"testing"
)

func TestFrameV2RoundTrip(t *testing.T) {
	original := Frame{
		Version:  Version2,
		Type:     TypeData,
		StreamID: 42,
		Payload:  []byte("mux"),
	}
	encoded, err := EncodeFrameV2(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeFrameV2(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.StreamID != original.StreamID || decoded.Type != original.Type ||
		!bytes.Equal(decoded.Payload, original.Payload) {
		t.Fatalf("frame v2 mismatch: %+v vs %+v", decoded, original)
	}
}

func TestTargetPayloadRoundTrip(t *testing.T) {
	encoded := EncodeTargetPayload("example.com", 443)
	host, port, err := DecodeTargetPayload(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if host != "example.com" || port != 443 {
		t.Fatalf("target mismatch: %s:%d", host, port)
	}
}

func TestSessionPayloadRoundTrip(t *testing.T) {
	encoded := EncodeSessionPayload("secret")
	pass, err := DecodeSessionPayload(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pass != "secret" {
		t.Fatalf("password mismatch")
	}
}
