export function DashboardValueBarSkeleton() {
    return (
        <div className="dashboard-value-skeleton" aria-hidden="true">
            <div className="dashboard-skeleton-row">
                <span className="dashboard-skeleton-line dashboard-skeleton-icon" />
                <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                <span className="dashboard-skeleton-line dashboard-skeleton-label" />
            </div>
            <div className="dashboard-skeleton-row">
                <span className="dashboard-skeleton-line dashboard-skeleton-icon" />
                <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                <span className="dashboard-skeleton-line dashboard-skeleton-label" />
            </div>
            <div className="dashboard-skeleton-row">
                <span className="dashboard-skeleton-line dashboard-skeleton-icon" />
                <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                <span className="dashboard-skeleton-line dashboard-skeleton-label" />
            </div>
        </div>
    );
}

export default DashboardValueBarSkeleton;
