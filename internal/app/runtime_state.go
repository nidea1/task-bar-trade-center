package app

const (
	runtimeStepPending  = "pending"
	runtimeStepRunning  = "running"
	runtimeStepOK       = "ok"
	runtimeStepDegraded = "degraded"
	runtimeStepFailed   = "failed"
)

const (
	runtimePhaseStarting  = "starting"
	runtimePhaseWaiting   = "waiting_game"
	runtimePhaseAttaching = "attaching"
	runtimePhasePreparing = "preparing"
	runtimePhaseReady     = "ready"
	runtimePhaseFailed    = "failed"
)

type RuntimeStepInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type RuntimeStateInfo struct {
	Ready         bool              `json:"ready"`
	Preparing     bool              `json:"preparing"`
	Phase         string            `json:"phase"`
	Message       string            `json:"message"`
	AppStatus     int32             `json:"app_status"`
	AppStatusText string            `json:"app_status_text"`
	Steps         []RuntimeStepInfo `json:"steps"`
}

func defaultRuntimeSteps() []RuntimeStepInfo {
	return []RuntimeStepInfo{
		{ID: "app", Label: trFallback("preparing.step_app", "Application"), State: runtimeStepPending},
		{ID: "game_process", Label: trFallback("preparing.step_game_process", "TaskBarHero process"), State: runtimeStepPending},
		{ID: "game_assembly", Label: trFallback("preparing.step_game_assembly", "GameAssembly.dll"), State: runtimeStepPending},
		{ID: "game_layout", Label: trFallback("preparing.step_game_layout", "Game memory layout"), State: runtimeStepPending},
		{ID: "inventory", Label: trFallback("preparing.step_inventory", "Inventory snapshot"), State: runtimeStepPending},
		{ID: "overlay", Label: trFallback("preparing.step_overlay", "Price overlay"), State: runtimeStepPending},
	}
}

func initializeRuntimeState() {
	setRuntimeState(RuntimeStateInfo{
		Ready:     false,
		Preparing: true,
		Phase:     runtimePhaseStarting,
		Message:   tr("status.starting"),
		Steps:     defaultRuntimeSteps(),
	})
}

func currentRuntimeState() RuntimeStateInfo {
	activeApp.runtimeStateMu.RLock()
	state := activeApp.runtimeState
	activeApp.runtimeStateMu.RUnlock()
	return normalizeRuntimeState(state)
}

func normalizeRuntimeState(state RuntimeStateInfo) RuntimeStateInfo {
	if state.Phase == "" {
		state.Phase = runtimePhaseStarting
		state.Message = tr("status.starting")
		state.Preparing = true
	}
	if len(state.Steps) == 0 {
		state.Steps = defaultRuntimeSteps()
	}
	state.AppStatus = activeApp.appStatus.Load()
	state.AppStatusText = appStatusText()
	return state
}

func setRuntimeState(next RuntimeStateInfo) {
	next = normalizeRuntimeState(next)
	activeApp.runtimeStateMu.Lock()
	activeApp.runtimeState = next
	activeApp.runtimeStateMu.Unlock()
	callRuntimeStateUpdated(next)
}

func updateRuntimeState(mutator func(*RuntimeStateInfo)) {
	activeApp.runtimeStateMu.Lock()
	state := normalizeRuntimeState(activeApp.runtimeState)
	mutator(&state)
	state = normalizeRuntimeState(state)
	activeApp.runtimeState = state
	activeApp.runtimeStateMu.Unlock()
	callRuntimeStateUpdated(state)
}

func setRuntimePhase(phase string, message string, ready bool, preparing bool) {
	updateRuntimeState(func(state *RuntimeStateInfo) {
		state.Phase = phase
		state.Message = message
		state.Ready = ready
		state.Preparing = preparing
	})
}

func setRuntimeStep(id string, stepState string, message string) {
	updateRuntimeState(func(state *RuntimeStateInfo) {
		for index := range state.Steps {
			if state.Steps[index].ID == id {
				state.Steps[index].State = stepState
				state.Steps[index].Message = message
				return
			}
		}
		state.Steps = append(state.Steps, RuntimeStepInfo{ID: id, Label: id, State: stepState, Message: message})
	})
}

func resetRuntimeForWaitingGame() {
	steps := defaultRuntimeSteps()
	steps[0].State = runtimeStepOK
	setRuntimeState(RuntimeStateInfo{
		Ready:     false,
		Preparing: true,
		Phase:     runtimePhaseWaiting,
		Message:   tr("status.waiting_game"),
		Steps:     steps,
	})
}

func resetRuntimeForGamePreparation() {
	steps := defaultRuntimeSteps()
	for index := range steps {
		switch steps[index].ID {
		case "app", "game_process", "game_assembly", "game_layout":
			steps[index].State = runtimeStepOK
		case "inventory":
			steps[index].State = runtimeStepRunning
			steps[index].Message = trFallback("preparing.inventory_message", "Resolving PlayerSaveData...")
		}
	}
	setRuntimeState(RuntimeStateInfo{
		Ready:     false,
		Preparing: true,
		Phase:     runtimePhasePreparing,
		Message:   trFallback("preparing.message", "Preparing TaskBarHero data..."),
		Steps:     steps,
	})
}

func markRuntimeReady() {
	updateRuntimeState(func(state *RuntimeStateInfo) {
		state.Ready = true
		state.Preparing = false
		state.Phase = runtimePhaseReady
		state.Message = tr("status.ready")
		for index := range state.Steps {
			if state.Steps[index].State == runtimeStepPending || state.Steps[index].State == runtimeStepRunning {
				if state.Steps[index].ID == "overlay" {
					state.Steps[index].State = runtimeStepDegraded
					state.Steps[index].Message = trFallback("preparing.overlay_background", "Finalized in the background")
					continue
				}
				state.Steps[index].State = runtimeStepOK
			}
		}
	})
}

func markRuntimeFailed(message string) {
	setRuntimePhase(runtimePhaseFailed, message, false, true)
}

func runtimeReady() bool {
	return currentRuntimeState().Ready
}

func GetRuntimeState() RuntimeStateInfo {
	return currentRuntimeState()
}
