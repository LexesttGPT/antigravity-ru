package main

import (
	"errors"
	"time"
)

// closeAndWait uses only a normal close request, never forced termination.
// stopped returns true only when the process scan succeeds and finds none.
func closeAndWait(stopped func() (bool, error), request func() error, timeout time.Duration, pause func(time.Duration)) error {
	done, err := stopped()
	if err != nil || done {
		return err
	}
	if err = request(); err != nil {
		return err
	}
	// Bounded retries, deterministic and testable. Recheck after every wait.
	const interval = 250 * time.Millisecond
	for left := timeout; left > 0; left -= interval {
		wait := interval
		if left < wait {
			wait = left
		}
		pause(wait)
		done, err = stopped()
		if err != nil || done {
			return err
		}
	}
	return errors.New("Antigravity продолжает работать. Изменения заблокированы: приложение не завершилось штатно (возможно, активна задача или фоновый режим).")
}
