import React from 'react';

interface MetricCardProps {
    label: string;
    value: string;
    icon: React.ReactNode;
    iconColor: string;
    tooltip?: React.ReactNode;
    tooltipDirection?: "up" | "down";
}

export function MetricCard({
    label,
    value,
    icon,
    iconColor,
    tooltip,
    tooltipDirection = "up"
}: MetricCardProps) {
    const tooltipClass = tooltipDirection === "down"
        ? "metric-tooltip-down"
        : "metric-tooltip-up";

    return (
        <div className="game-metric metric-card p-2.5 flex items-center justify-between gap-2.5 group relative">
            <div className="space-y-0.5 min-w-0">
                <span className="text-[9px] text-[#9a896f] font-bold block uppercase tracking-wider" title={label}>{label}</span>
                <strong className="text-xs font-bold text-[#e1d5bf] tracking-wide block truncate" title={value}>{value}</strong>
            </div>
            <div className={`p-1.5 rounded bg-[#030304] border border-[#2d261f] shrink-0 ${iconColor}`}>
                {icon}
            </div>

            {tooltip && (
                <div className={`metric-tooltip ${tooltipClass}`}>
                    <div className="metric-tooltip-arrow metric-tooltip-arrow-outer" />
                    <div className="metric-tooltip-arrow metric-tooltip-arrow-inner" />
                    {tooltip}
                </div>
            )}
        </div>
    );
}
export default MetricCard;
