import { RefreshCw } from 'lucide-react';

interface DashboardLoadingSkeletonProps {
    title: string;
}

export function DashboardLoadingSkeleton({ title }: DashboardLoadingSkeletonProps) {
    const metricSkeletons = [0, 1, 2, 3, 4];
    const heroSkeletons = [0, 1, 2, 3, 4, 5];
    const itemSkeletons = [0, 1, 2, 3, 4, 5];

    return (
        <div className="dashboard-loading-stack" role="status" aria-live="polite">
            <section className="game-panel dashboard-loading-panel">
                <div className="dashboard-loading-copy">
                    <RefreshCw className="w-4 h-4 animate-spin text-[#ffbe2d]" />
                    <span>{title}</span>
                </div>
                <div className="dashboard-loading-bars" aria-hidden="true">
                    <span className="dashboard-skeleton-line dashboard-loading-bar-wide" />
                    <span className="dashboard-skeleton-line dashboard-loading-bar" />
                </div>
            </section>

            <section className="metrics-grid" aria-hidden="true">
                {metricSkeletons.map((item) => (
                    <div key={item} className="game-metric metric-card dashboard-metric-skeleton">
                        <div className="dashboard-skeleton-stack">
                            <span className="dashboard-skeleton-line dashboard-skeleton-label" />
                            <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                        </div>
                        <span className="dashboard-skeleton-line dashboard-skeleton-tile" />
                    </div>
                ))}
            </section>

            <section className="space-y-2" aria-hidden="true">
                <span className="dashboard-skeleton-line dashboard-section-title-skeleton" />
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
                    {heroSkeletons.map((item) => (
                        <div key={item} className="game-metric dashboard-hero-skeleton">
                            <div className="dashboard-skeleton-stack">
                                <span className="dashboard-skeleton-line dashboard-skeleton-label" />
                                <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                            </div>
                            <span className="dashboard-skeleton-line dashboard-skeleton-avatar" />
                        </div>
                    ))}
                </div>
            </section>

            <section className="game-panel dashboard-items-skeleton" aria-hidden="true">
                <div className="game-header dashboard-items-skeleton-header">
                    <span className="dashboard-skeleton-line dashboard-items-heading-skeleton" />
                    <span className="dashboard-skeleton-line dashboard-items-badge-skeleton" />
                </div>
                <div className="game-accent-line" />
                <div className="dashboard-items-skeleton-grid">
                    {itemSkeletons.map((item) => (
                        <div key={item} className="inventory-card dashboard-item-skeleton-card">
                            <div className="dashboard-skeleton-line dashboard-item-icon-skeleton" />
                            <div className="dashboard-skeleton-stack dashboard-item-copy-skeleton">
                                <span className="dashboard-skeleton-line dashboard-item-title-skeleton" />
                                <span className="dashboard-skeleton-line dashboard-item-meta-skeleton" />
                                <span className="dashboard-skeleton-line dashboard-item-price-skeleton" />
                            </div>
                        </div>
                    ))}
                </div>
            </section>
        </div>
    );
}

export default DashboardLoadingSkeleton;
