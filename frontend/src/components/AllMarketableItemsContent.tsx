import { Package } from 'lucide-react';
import { DashboardItem, DashboardState, PriceMode } from '../types';
import { InventoryItemCard } from './InventoryItemCard';

interface AllMarketableItemsContentProps {
    items: DashboardItem[];
    state: DashboardState | null;
    currentLanguage: string;
    priceMode: PriceMode;
    t: (key: string, fallback: string) => string;
}

export function AllMarketableItemsContent({
    items,
    state,
    currentLanguage,
    priceMode,
    t
}: AllMarketableItemsContentProps) {
    return (
        <div className="inventory-card-grid inventory-card-scroll bg-[#030304]">
            {items.length === 0 ? (
                <div className="inventory-empty">
                    <Package className="w-8 h-8 mb-2 opacity-30" />
                    <span>{t("dashboard.no_items", "No items available")}</span>
                </div>
            ) : (
                items.map((item) => (
                    <InventoryItemCard
                        key={`all-${item.item_id}`}
                        item={item}
                        state={state}
                        currentLanguage={currentLanguage}
                        priceMode={priceMode}
                        t={t}
                    />
                ))
            )}
        </div>
    );
}

export default AllMarketableItemsContent;
