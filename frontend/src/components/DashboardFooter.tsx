import { AlertTriangle, CheckCircle, RefreshCw } from 'lucide-react';
import { DashboardFooterInfo } from '../types';
import { appIcon } from '../constants';
import { localizedFallback } from '../utils/translations';

interface DashboardFooterProps {
    info: DashboardFooterInfo | null;
    isRefreshing: boolean;
    currentLanguage: string;
    t: (key: string, fallback: string) => string;
}

export function DashboardFooter({
    info,
    isRefreshing,
    currentLanguage,
    t
}: DashboardFooterProps) {
    const appShortName = info?.app_short_name || "TBTC";
    const creatorName = info?.creator_name || "nidea1";
    const version = info?.version || "";
    const updateText = info?.update_text || t("update.unknown", "Not checked yet");
    const refreshText = isRefreshing
        ? t("dashboard.footer_syncing", localizedFallback(currentLanguage, "Senkronize ediliyor", "Syncing"))
        : t("dashboard.footer_ready", localizedFallback(currentLanguage, "Hazir", "Ready"));

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
                <span>{t("dashboard.footer_created_by", localizedFallback(currentLanguage, "%s tarafindan", "by %s")).replace("%s", creatorName)}</span>
                {version && <span className="dashboard-footer-version">v{version}</span>}
            </div>

            <div className={`dashboard-footer-update themed-tooltip-host ${info?.update_available ? "has-update" : ""}`} data-tooltip={updateText}>
                {info?.update_available ? <AlertTriangle className="dashboard-footer-icon" /> : <CheckCircle className="dashboard-footer-icon" />}
                <span>{updateText}</span>
            </div>

            <div className="dashboard-footer-rule" aria-hidden="true" />

            <div className="dashboard-footer-sync themed-tooltip-host" data-tooltip={refreshText}>
                <span className={`dashboard-footer-state ${isRefreshing ? "is-refreshing" : ""}`} aria-hidden="true" />
                {isRefreshing && <RefreshCw className="dashboard-footer-icon animate-spin" />}
                <span className="dashboard-footer-sync-text">{refreshText}</span>
            </div>
        </footer>
    );
}

export default DashboardFooter;
