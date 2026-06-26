package protocol

import (
	"testing"
)

func TestHTTPPayloadRoundTrip(t *testing.T) {
	req := HTTPReqPayload{
		Password: "secret",
		Method:   "GET",
		URL:      "https://whoer.net/",
		Headers:  [][2]string{{"User-Agent", "test"}, {"Accept", "*/*"}},
		Body:     []byte("body"),
	}
	data := EncodeHTTPReqPayload(req)
	got, err := DecodeHTTPReqPayload(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != req.Password || got.Method != req.Method || got.URL != req.URL {
		t.Fatalf("req mismatch: %+v", got)
	}
	if len(got.Headers) != 2 || string(got.Body) != "body" {
		t.Fatalf("headers/body mismatch: %+v", got)
	}

	resp := HTTPRespPayload{
		Status:  200,
		Headers: [][2]string{{"Content-Type", "text/html"}},
		Body:    []byte("<html></html>"),
	}
	rdata := EncodeHTTPRespPayload(resp)
	rgot, err := DecodeHTTPRespPayload(rdata)
	if err != nil {
		t.Fatal(err)
	}
	if rgot.Status != 200 || string(rgot.Body) != "<html></html>" {
		t.Fatalf("resp mismatch: %+v", rgot)
	}
}
