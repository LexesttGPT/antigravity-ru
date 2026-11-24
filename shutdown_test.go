package main

import (
	"errors"
	"testing"
	"time"
)

func TestCloseAndWait(t *testing.T) {
	for _, c := range []struct {
		name                string
		stoppedAfter        int
		scanErrAt           int
		requestErr          bool
		wantClose, wantFail bool
	}{
		{"already stopped", 1, 0, false, false, false},
		{"normal close", 3, 0, false, true, false},
		{"refuses close", 99, 0, false, true, true},
		{"initial scan error", 99, 1, false, false, true},
		{"later scan error", 99, 2, false, true, true},
		{"request error", 99, 0, true, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			n, requests := 0, 0
			err := closeAndWait(func() (bool, error) {
				n++
				if n == c.scanErrAt {
					return false, errors.New("scan")
				}
				return n >= c.stoppedAfter, nil
			}, func() error {
				requests++
				if c.requestErr {
					return errors.New("request")
				}
				return nil
			}, time.Second, func(time.Duration) {})
			if (err != nil) != c.wantFail || (requests == 1) != c.wantClose || requests > 1 {
				t.Fatalf("err=%v requests=%d", err, requests)
			}
		})
	}
}
