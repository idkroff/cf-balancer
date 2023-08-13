package timer

import "time"

func SetInterval(f func(), milliseconds int) chan bool {
	interval := time.Duration(milliseconds) * time.Millisecond

	ticker := time.NewTicker(interval)
	clear := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				go f()
			case <-clear:
				ticker.Stop()
				return
			}

		}
	}()

	return clear
}
