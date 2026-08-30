package service

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
)

func TestApplyResponseHeaderTimeout(t *testing.T) {
	orig := common.RelayResponseHeaderTimeout
	t.Cleanup(func() { common.RelayResponseHeaderTimeout = orig })

	common.RelayResponseHeaderTimeout = 0
	tr := &http.Transport{}
	applyResponseHeaderTimeout(tr)
	if tr.ResponseHeaderTimeout != 0 {
		t.Fatalf("0 must keep the unbounded default, got %v", tr.ResponseHeaderTimeout)
	}

	common.RelayResponseHeaderTimeout = 1800
	tr = &http.Transport{}
	applyResponseHeaderTimeout(tr)
	if tr.ResponseHeaderTimeout != 1800*time.Second {
		t.Fatalf("got %v, want 30m", tr.ResponseHeaderTimeout)
	}

	common.RelayResponseHeaderTimeout = math.MaxInt
	tr = &http.Transport{}
	applyResponseHeaderTimeout(tr)
	if tr.ResponseHeaderTimeout <= 0 {
		t.Fatalf("huge value must clamp to a positive duration, got %v", tr.ResponseHeaderTimeout)
	}
}
