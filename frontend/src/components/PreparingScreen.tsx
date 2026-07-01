import { Activity, AlertTriangle, CheckCircle, Circle, Database, Loader2, MinusCircle } from 'lucide-react';
import { appIcon } from '../constants';
import type { RuntimeStateInfo, RuntimeStepInfo, RuntimeStepState } from '../types';
import { localizedFallback } from '../utils/translations';

type Translator = (key: string, fallback?: string) => string;

interface PreparingScreenProps {
    runtimeState: RuntimeStateInfo | null;
    currentLanguage: string;
    t: Translator;
}

const fallbackSteps: RuntimeStepInfo[] = [
    { id: "app", label: "Application", state: "ok" },
    { id: "game_process", label: "TaskBarHero process", state: "pending" },
    { id: "game_assembly", label: "GameAssembly.dll", state: "pending" },
    { id: "game_layout", label: "Game memory layout", state: "pending" },
    { id: "inventory", label: "Inventory snapshot", state: "pending" },
    { id: "overlay", label: "Price overlay", state: "pending" },
];

function stepIcon(state: RuntimeStepState) {
    switch (state) {
        case "ok":
            return <CheckCircle className="preparing-step-icon preparing-step-icon-ok" />;
        case "running":
            return <Loader2 className="preparing-step-icon preparing-step-icon-running" />;
        case "degraded":
            return <MinusCircle className="preparing-step-icon preparing-step-icon-degraded" />;
        case "failed":
            return <AlertTriangle className="preparing-step-icon preparing-step-icon-failed" />;
        default:
            return <Circle className="preparing-step-icon preparing-step-icon-pending" />;
    }
}

function stepWeight(state: RuntimeStepState) {
    switch (state) {
        case "ok":
            return 1;
        case "degraded":
            return 0.85;
        case "running":
            return 0.45;
        default:
            return 0;
    }
}

function stepLabel(state: RuntimeStepState, t: Translator) {
    switch (state) {
        case "ok":
            return t("preparing.state_ok", "Ready");
        case "running":
            return t("preparing.state_running", "Running");
        case "degraded":
            return t("preparing.state_degraded", "Background");
        case "failed":
            return t("preparing.state_failed", "Retrying");
        default:
            return t("preparing.state_pending", "Pending");
    }
}

export default function PreparingScreen({ runtimeState, currentLanguage, t }: PreparingScreenProps) {
    const steps = runtimeState?.steps?.length ? runtimeState.steps : fallbackSteps;
    const completedSteps = steps.filter((step) => step.state === "ok" || step.state === "degraded").length;
    const weightedProgress = steps.reduce((sum, step) => sum + stepWeight(step.state), 0);
    const progress = steps.length > 0 ? Math.min(100, Math.round((weightedProgress / steps.length) * 100)) : 0;
    const activeStep = steps.find((step) => step.state === "running")
        || steps.find((step) => step.state === "failed")
        || steps.find((step) => step.state === "pending")
        || steps[steps.length - 1];
    const message = runtimeState?.message || t(
        "preparing.message",
        localizedFallback(currentLanguage, "TaskBarHero verileri hazırlanıyor...", "Preparing TaskBarHero data...")
    );

    return (
        <section className="preparing-screen game-panel" aria-live="polite">
            <div className="preparing-top">
                <div className="preparing-brand">
                    <div className="preparing-logo-frame">
                        <img src={appIcon} alt="" className="preparing-logo" />
                        <span className="preparing-logo-pulse" />
                    </div>
                    <div className="preparing-copy">
                        <p className="preparing-kicker">
                            <Activity className="preparing-kicker-icon" />
                            {runtimeState?.app_status_text || t("status.preparing", "Preparing...")}
                        </p>
                        <h2 className="preparing-title">
                            {t("preparing.title", localizedFallback(currentLanguage, "Hazırlanıyor", "Preparing"))}
                        </h2>
                        <p className="preparing-message">{message}</p>
                    </div>
                </div>

                <div className="preparing-current">
                    <div className="preparing-current-header">
                        <span>{t("preparing.current_step", localizedFallback(currentLanguage, "Aktif adım", "Current step"))}</span>
                        <Loader2 className="preparing-spinner" />
                    </div>
                    <strong title={activeStep?.label || ""}>{activeStep?.label || t("status.preparing", "Preparing...")}</strong>
                    <p title={activeStep?.message || ""}>
                        {activeStep?.message || stepLabel(activeStep?.state || "pending", t)}
                    </p>
                    <div className="preparing-progress" aria-hidden="true">
                        <span style={{ width: `${progress}%` }} />
                    </div>
                    <div className="preparing-progress-meta">
                        <span>{progress}%</span>
                        <span>{completedSteps}/{steps.length}</span>
                    </div>
                </div>
            </div>

            <div className="preparing-steps-header">
                <Database className="preparing-steps-header-icon" />
                <span>{t("preparing.checks_title", localizedFallback(currentLanguage, "Hazırlık kontrolleri", "Preparation checks"))}</span>
            </div>

            <div className="preparing-steps" role="list">
                {steps.map((step, index) => (
                    <div key={step.id} className={`preparing-step preparing-step-${step.state}`} role="listitem">
                        <span className="preparing-step-number">{String(index + 1).padStart(2, "0")}</span>
                        <div className="preparing-step-copy">
                            <span className="preparing-step-title">{step.label}</span>
                            {step.message ? <span className="preparing-step-message">{step.message}</span> : null}
                        </div>
                        {stepIcon(step.state)}
                        <span className="preparing-step-state">{stepLabel(step.state, t)}</span>
                    </div>
                ))}
            </div>
        </section>
    );
}
