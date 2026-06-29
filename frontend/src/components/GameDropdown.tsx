import React, { useState, useEffect, useRef } from 'react';
import { DropdownOption } from '../types';

interface GameDropdownProps {
    value: string;
    options: DropdownOption[];
    onChange: (val: string) => void;
    className?: string;
    prefix?: string;
    selectedLabel?: string;
    icon?: React.ReactNode;
    title?: string;
    ariaLabel?: string;
    iconOnly?: boolean;
}

export function GameDropdown({
    value,
    options,
    onChange,
    className = "",
    prefix,
    selectedLabel,
    icon,
    title,
    ariaLabel,
    iconOnly = false
}: GameDropdownProps) {
    const [isOpen, setIsOpen] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);
    
    const selectedOption = options.find(opt => opt.value === value);
    const displayLabel = selectedLabel || (selectedOption ? selectedOption.label : value);
    const displayTitle = title || (prefix ? `${prefix}: ${displayLabel}` : displayLabel);
    const optionStyle = (option: DropdownOption): React.CSSProperties | undefined => {
        if (!option.color) return undefined;
        return {
            '--raw-option-color': option.color,
            color: 'var(--option-color, var(--raw-option-color))',
            borderLeftColor: option.value === value ? 'var(--option-color, var(--raw-option-color))' : 'transparent'
        } as React.CSSProperties;
    };

    useEffect(() => {
        if (!isOpen) return;
        const handleClickOutside = (event: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, [isOpen]);

    return (
        <div
            ref={dropdownRef}
            className={`themed-tooltip-host relative inline-block text-left ${className}`}
            data-tooltip={isOpen ? undefined : displayTitle}
        >
            <button
                type="button"
                onClick={() => setIsOpen(!isOpen)}
                aria-label={ariaLabel || displayTitle}
                className={
                    iconOnly
                        ? "dashboard-icon-dropdown-button game-button cursor-pointer flex items-center justify-center"
                        : "game-button px-3 py-1.5 text-xs font-bold cursor-pointer flex items-center justify-between gap-2.5 min-w-[120px] tracking-wide"
                }
            >
                {iconOnly ? (
                    <span className="dashboard-dropdown-icon">{icon}</span>
                ) : (
                    <span className="dashboard-dropdown-content">
                        {icon && <span className="dashboard-dropdown-icon">{icon}</span>}
                        <span className="dashboard-dropdown-label">
                            {prefix && <span className="dashboard-dropdown-prefix">{prefix}</span>}
                            <span className="dashboard-dropdown-value">{displayLabel}</span>
                        </span>
                    </span>
                )}
                {!iconOnly && <span className="text-[8px] text-[#ffbe2d] shrink-0">▼</span>}
            </button>

            {isOpen && (
                <div
                    className="!absolute right-0 mt-1.5 z-50 min-w-[160px] max-h-[240px] game-panel game-dropdown-menu bg-[#0d0b12] border-2 border-[#463d30] shadow-2xl rounded"
                    style={{ position: 'absolute' }}
                    onWheel={(event) => event.stopPropagation()}
                >
                    <div className="py-0.5">
                        {options.map((opt) => (
                            <button
                                key={opt.value}
                                data-has-color={!!opt.color}
                                onClick={() => {
                                    onChange(opt.value);
                                    setIsOpen(false);
                                }}
                                style={optionStyle(opt)}
                                className={`w-full text-left px-3 py-1.5 text-xs block transition-all relative ${
                                    opt.value === value
                                        ? "text-[#ffbe2d] font-bold bg-[#1e150d] border-l-2 border-[#ffbe2d]"
                                        : "text-[#e1d5bf] hover:text-white hover:bg-[#1a1410] border-l-2 border-transparent"
                                }`}
                            >
                                {opt.label}
                            </button>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}

export default GameDropdown;
