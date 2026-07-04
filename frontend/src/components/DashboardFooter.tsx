import { AlertTriangle, CheckCircle, RefreshCw } from 'lucide-react';
import { DashboardFooterInfo } from '../types';
import { appIcon } from '../constants';
import { localizedFallback } from '../utils/translations';

interface DashboardFooterProps {
    info: DashboardFooterInfo | null;
    isRefreshing: boolean;
    onCheckUpdates: () => void;
    currentLanguage: string;
    t: (key: string, fallback: string) => string;
    syncTimeText?: string;
}

export function DashboardFooter({
    info,
    isRefreshing,
    onCheckUpdates,
    currentLanguage,
    t,
    syncTimeText
}: DashboardFooterProps) {
    const appShortName = info?.app_short_name || "TBTC";
    const creatorName = info?.creator_name || "nidea1";
    const version = info?.version || "";
    const updateText = info?.update_text || t("update.unknown", "Not checked yet");
    const updateChecking = info?.update_status === 1;
    const updateDownloading = info?.update_status === 5;
    const updateBusy = updateChecking || updateDownloading;
    const updateActionText = updateChecking
        ? t("update.checking", localizedFallback(currentLanguage, "Güncellemeler denetleniyor...", "Checking for updates..."))
        : t("dashboard.footer_check_updates", localizedFallback(currentLanguage, "Güncellemeleri denetle", "Check for updates"));
    const refreshText = isRefreshing
        ? t("dashboard.footer_syncing", localizedFallback(currentLanguage, "Senkronize ediliyor", "Syncing"))
        : t("dashboard.footer_ready", localizedFallback(currentLanguage, "Hazır", "Ready"));

    return (
        <footer
            className="dashboard-footer no-drag"
            aria-label={t("dashboard.footer_label", localizedFallback(currentLanguage, "Durum", "Status"))}
        >
            <div className="dashboard-footer-brand">
                <img src={appIcon} alt="" className="dashboard-footer-logo" />
                <span>{appShortName}</span>
            </div>

            <div className="dashboard-footer-promo">
                <span>{t("dashboard.footer_created_by", localizedFallback(currentLanguage, "%s tarafından", "by %s")).replace("%s", creatorName)}</span>
                {version && <span className="dashboard-footer-version">v{version}</span>}
            </div>

            <button
                type="button"
                className={`dashboard-footer-update themed-tooltip-host ${info?.update_available ? "has-update" : ""} ${updateBusy ? "is-busy" : ""}`}
                onClick={onCheckUpdates}
                disabled={updateBusy}
                aria-label={updateActionText}
                data-tooltip={updateBusy ? updateText : updateActionText}
            >
                {updateBusy ? (
                    <RefreshCw className="dashboard-footer-icon animate-spin" />
                ) : info?.update_available ? (
                    <AlertTriangle className="dashboard-footer-icon" />
                ) : (
                    <CheckCircle className="dashboard-footer-icon" />
                )}
                <span>{updateText}</span>
            </button>

            <div className="dashboard-footer-rule" aria-hidden="true" />

            {syncTimeText && (
                <div className="dashboard-footer-synctime">
                    <span>{syncTimeText}</span>
                </div>
            )}

            <div className="dashboard-footer-sync themed-tooltip-host" data-tooltip={refreshText}>
                <span className={`dashboard-footer-state ${isRefreshing ? "is-refreshing" : ""}`} aria-hidden="true" />
                {isRefreshing && <RefreshCw className="dashboard-footer-icon animate-spin" />}
                <span className="dashboard-footer-sync-text">{refreshText}</span>
            </div>
        </footer>
    );
}

export default DashboardFooter;
