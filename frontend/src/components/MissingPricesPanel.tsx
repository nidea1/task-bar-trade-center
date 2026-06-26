import React from 'react';
import { Package } from 'lucide-react';
import { DashboardItem, DashboardState } from '../types';
import { OpenMarketListing } from '../../wailsjs/go/main/App';
import { rarityMeta, formatLocation } from '../utils/helpers';

interface MissingPricesPanelProps {
    title: string;
    items: DashboardItem[];
    state: DashboardState | null;
    currentLanguage: string;
}

export function MissingPricesPanel({
    title,
    items,
    state,
    currentLanguage
}: MissingPricesPanelProps) {
    return (
        <section className="game-panel missing-prices-panel flex flex-col min-h-0">
            {/* Panel Header */}
            <div className="game-header flex items-center justify-between">
                <h2 className="text-xs font-bold text-[#ffbe2d] uppercase tracking-wider flex items-center gap-2" title={title}>
                    {title}
                </h2>
                <span className="game-badge text-[10px] font-bold px-2 py-0.5 rounded" title={String(items.length)}>
                    {items.length}
                </span>
            </div>

            {/* Gold accent line */}
            <div className="game-accent-line" />

            {/* Item List */}
            <div className="missing-prices-list flex-1 min-h-0 overflow-y-auto bg-[#030304] flex flex-col">
                {items.length === 0 ? (
                    <div className="flex-1 flex flex-col items-center justify-center py-12 px-4 text-center text-[#9a896f]">
                        <Package className="w-8 h-8 mb-2 opacity-30" />
                        <span className="text-[10px] font-bold uppercase tracking-wider">
                            {state?.translations?.["dashboard.no_items"] || "No items available"}
                        </span>
                    </div>
                ) : (
                    items.map((item) => {
                        const meta = rarityMeta(item.grade);
                        const itemName = item.name || item.market_hash_name || `Item ${item.item_id}`;
                        const style = { "--rarity-color": meta.color } as React.CSSProperties;

                        return (
                            <button
                                key={`${title}-${item.item_id}`}
                                onClick={() => OpenMarketListing(item.item_id)}
                                className="missing-price-row game-item-row w-full flex items-center gap-2.5 px-3 py-2 text-left group relative cursor-pointer bg-transparent"
                                style={style}
                                title={itemName}
                            >
                                {/* Item Left: Icon & Name */}
                                <div className="flex items-center gap-3 min-w-0 flex-1">
                                    {item.icon_url ? (
                                        <img
                                            src={item.icon_url}
                                            alt={itemName}
                                            className="game-icon-slot rarity-icon-slot w-8 h-8 object-contain rounded p-0.5 shrink-0"
                                        />
                                    ) : (
                                        <div className="game-icon-slot rarity-icon-slot w-8 h-8 rounded shrink-0 flex items-center justify-center font-bold text-xs text-[#9a896f]">
                                            {itemName ? itemName.charAt(0).toLocaleUpperCase(currentLanguage) : '?'}
                                        </div>
                                    )}
                                    <div className="min-w-0 flex-1">
                                        <h4 className="rarity-item-name text-[11px] font-bold truncate group-hover:text-white transition-colors flex items-center gap-1.5 uppercase tracking-wide" title={itemName}>
                                            {itemName}
                                        </h4>
                                        <div className="flex items-center gap-2 mt-0.5 min-w-0">
                                            <span className="text-[10px] text-[#9a896f] font-mono shrink-0" title={`x${item.count}`}>x{item.count}</span>
                                            <span className="text-[8px] text-[#463d30]">•</span>
                                            <span className="text-[10px] text-[#9a896f] capitalize truncate" title={formatLocation(item.location, state)}>{formatLocation(item.location, state)}</span>
                                        </div>
                                    </div>
                                </div>
                            </button>
                        );
                    })
                )}
            </div>
        </section>
    );
}

export default MissingPricesPanel;
