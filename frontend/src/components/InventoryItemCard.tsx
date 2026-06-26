import React from 'react';
import { ExternalLink } from 'lucide-react';
import { DashboardItem, DashboardState, PriceMode } from '../types';
import { OpenMarketListing } from '../../wailsjs/go/main/App';
import {
    rarityMeta,
    formatPrice,
    formatLocation,
    itemHasPriceForMode,
    itemUnitValue,
    itemTotalValue
} from '../utils/helpers';
import {
    translateRarity,
    translateItemType,
    translateGear
} from '../utils/translations';

interface InventoryItemCardProps {
    item: DashboardItem;
    state: DashboardState | null;
    currentLanguage: string;
    priceMode: PriceMode;
    t: (key: string, fallback: string) => string;
}

export function InventoryItemCard({
    item,
    state,
    currentLanguage,
    priceMode,
    t
}: InventoryItemCardProps) {
    const meta = rarityMeta(item.grade);
    const rarityLabel = translateRarity(item.grade, t, currentLanguage);
    const typeLabel = translateItemType(item.type, t, currentLanguage);
    const gearLabel = item.gear ? translateGear(item.gear, t, currentLanguage) : "";
    const itemName = item.name || item.market_hash_name || `Item ${item.item_id}`;
    const hasPrice = itemHasPriceForMode(item, priceMode);
    const unitPriceText = hasPrice ? formatPrice(itemUnitValue(item, priceMode), state, item) : t("dashboard.missing", "Missing");
    const totalPriceText = hasPrice ? formatPrice(itemTotalValue(item, priceMode), state, item) : t("dashboard.missing", "Missing");
    const locationText = formatLocation(item.location, state);
    const style = { "--rarity-color": meta.color } as React.CSSProperties;

    return (
        <button
            type="button"
            className="inventory-card group"
            style={style}
            onClick={() => OpenMarketListing(item.item_id)}
            title={itemName}
        >
            <div className="inventory-card-top">
                {item.icon_url ? (
                    <img
                        src={item.icon_url}
                        alt={itemName}
                        className="inventory-card-icon object-contain"
                    />
                ) : (
                    <div className="inventory-card-icon flex items-center justify-center font-bold text-xs text-[#9a896f]">
                        {itemName.charAt(0).toLocaleUpperCase(currentLanguage)}
                    </div>
                )}
                <div className="min-w-0 flex-1">
                    <div className="inventory-card-title">
                        <span title={itemName}>{itemName}</span>
                        <ExternalLink className="w-3 h-3 text-[#7e5326] opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                    </div>
                    <div className="inventory-card-meta">
                        <span title={`x${item.count}`}>x{item.count}</span>
                        <span title={locationText}>{locationText}</span>
                    </div>
                </div>
            </div>

            <div className="inventory-card-tags">
                <span className="inventory-card-rarity" title={rarityLabel}>{rarityLabel}</span>
                {typeLabel && <span className="inventory-card-tag" title={typeLabel}>{typeLabel}</span>}
                {gearLabel && <span className="inventory-card-tag" title={gearLabel}>{gearLabel}</span>}
            </div>

            <div className="inventory-card-prices">
                <div>
                    <span>{t("dashboard.unit_price", "Unit")}</span>
                    <strong title={unitPriceText}>{unitPriceText}</strong>
                </div>
                <div>
                    <span>{t("dashboard.total_price", "Total")}</span>
                    <strong title={totalPriceText}>{totalPriceText}</strong>
                </div>
            </div>
        </button>
    );
}

export default InventoryItemCard;
