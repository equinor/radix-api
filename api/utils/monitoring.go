package utils

import "sync"

type Monitor struct {
	triggeredJobs map[string]int
}

var singleton *Monitor
var once sync.Once

// GetMonitor Gets the singleton
func GetMonitor() *Monitor {
	once.Do(func() {
		singleton = &Monitor{
			triggeredJobs: make(map[string]int),
		}
	})

	return singleton
}

func (mon *Monitor) AddJobTriggered(appName string) {
	if val, ok := mon.triggeredJobs[appName]; ok {
		mon.triggeredJobs[appName] = val + 1
	} else {
		mon.triggeredJobs[appName] = 1
	}
}

func (mon *Monitor) GetJobsTriggered() map[string]int {
	return mon.triggeredJobs
}
