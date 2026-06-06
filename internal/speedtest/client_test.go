package speedtest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseColo(t *testing.T) {
	body := "fl=123\ncolo=SIN\nloc=SG\n"
	if got := parseColo(body); got != "SIN" {
		t.Fatalf("parseColo = %q, want SIN", got)
	}
}

func TestParseColoMissing(t *testing.T) {
	if got := parseColo("fl=123\n"); got != "" {
		t.Fatalf("parseColo = %q, want empty", got)
	}
}

func TestFetchColo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("colo=LHR\n"))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), TraceURL: srv.URL}
	got, err := c.fetchColo()
	if err != nil {
		t.Fatalf("fetchColo error: %v", err)
	}
	if got != "LHR" {
		t.Fatalf("fetchColo = %q, want LHR", got)
	}
}
