package game

import (
	"sync"
	"time"
)

const LayoutPointerFailureAfter = 3 * time.Second

type PointerReadKind int

const (
	PointerReadHoveredItem PointerReadKind = iota
	PointerReadTooltip
)

type PointerReadHealth struct {
	mu                  sync.Mutex
	hoveredFailureSince time.Time
	incompatible        bool
	notified            bool
}

func (health *PointerReadHealth) Record(now time.Time, kind PointerReadKind, success bool) (bool, bool, bool) {
	if kind != PointerReadHoveredItem {
		return false, false, false
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	if success {
		health.hoveredFailureSince = time.Time{}
		if health.incompatible {
			health.incompatible = false
			return false, false, true
		}
		return false, false, false
	}

	if health.hoveredFailureSince.IsZero() {
		health.hoveredFailureSince = now
		return false, false, false
	}
	if !health.incompatible && now.Sub(health.hoveredFailureSince) >= LayoutPointerFailureAfter {
		health.incompatible = true
		if !health.notified {
			health.notified = true
			return true, true, false
		}
		return true, false, false
	}
	return false, false, false
}

func (health *PointerReadHealth) Reset() {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.hoveredFailureSince = time.Time{}
	health.incompatible = false
	health.notified = false
}

func (health *PointerReadHealth) SetIncompatibleForTest(value bool) {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.incompatible = value
}

func (health *PointerReadHealth) IncompatibleForTest() bool {
	health.mu.Lock()
	defer health.mu.Unlock()
	return health.incompatible
}
