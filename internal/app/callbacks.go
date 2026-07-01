package app

import (
	"sync"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
)

type Callbacks struct {
	OpenDashboard          func()
	Quit                   func()
	DashboardUpdated       func(inventory.DashboardState)
	DashboardFooterUpdated func(DashboardFooterInfo)
	RuntimeStateUpdated    func(RuntimeStateInfo)
}

var callbacks = struct {
	sync.RWMutex
	value Callbacks
}{}

func SetCallbacks(next Callbacks) {
	callbacks.Lock()
	callbacks.value = next
	callbacks.Unlock()
}

func callOpenDashboard() {
	callbacks.RLock()
	fn := callbacks.value.OpenDashboard
	callbacks.RUnlock()
	if fn != nil {
		fn()
	}
}

func callQuit() {
	callbacks.RLock()
	fn := callbacks.value.Quit
	callbacks.RUnlock()
	if fn != nil {
		fn()
	}
}

func callDashboardUpdated(state inventory.DashboardState) {
	callbacks.RLock()
	fn := callbacks.value.DashboardUpdated
	callbacks.RUnlock()
	if fn != nil {
		go fn(state)
	}
}

func callDashboardFooterUpdated(info DashboardFooterInfo) {
	callbacks.RLock()
	fn := callbacks.value.DashboardFooterUpdated
	callbacks.RUnlock()
	if fn != nil {
		fn(info)
	}
}

func callRuntimeStateUpdated(info RuntimeStateInfo) {
	callbacks.RLock()
	fn := callbacks.value.RuntimeStateUpdated
	callbacks.RUnlock()
	if fn != nil {
		fn(info)
	}
}
