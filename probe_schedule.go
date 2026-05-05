package main

import "time"

func sleepUntilNextProbe(startedAt time.Time, checkInterval time.Duration) {
	delay := time.Until(startedAt.Add(checkInterval))
	if delay > 0 {
		time.Sleep(delay)
	}
}
