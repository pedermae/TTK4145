package watchdog

import (
	"time"
)

func Watchdog(seconds int, petWatchDogChan chan bool, watchDogStuckChan chan bool) {
	watchdogTimer := time.NewTimer(time.Duration(seconds) * time.Second)
	for {
		select {
		case <-petWatchDogChan:
			watchdogTimer.Reset(time.Duration(seconds) * time.Second)
		case <-watchdogTimer.C:
			watchDogStuckChan <- true
			watchdogTimer.Reset(time.Duration(seconds) * time.Second)
		}
	}
}