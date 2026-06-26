import React from 'react';
import { Package, ExternalLink } from 'lucide-react';
import { DashboardItem, DashboardState } from '../types';
import { OpenMarketListing } from '../../wailsjs/go/main/App';
import { rarityMeta, formatPrice, formatNumber } from '../utils/helpers';
import { bestSellReasonLabel } from '../utils/translations';

interface BestItemsToSellNowContentProps {
    items: DashboardItem[];
    state: DashboardState | null;
    currentLanguage: string;
    t: (key: string, fallback: string) => string;
}

export function BestItemsToSellNowContent({
    items,
    state,
    currentLanguage,
    t
}: BestItemsToSellNowContentProps) {
    return (
        <div className="best-sell-grid">
            {items.length === 0 ? (
                <div className="inventory-empty">
                    <Package className="w-8 h-8 mb-2 opacity-30" />
                    <span>{t("dashboard.no_items", "No items available")}</span>
                </div>
            ) : (
                items.map((item) => {
                    const meta = rarityMeta(item.grade);
                    const itemName = item.name || item.market_hash_name || `Item ${item.item_id}`;
                    const score = Math.round(item.sell_score || 0);
                    const spreadText = item.has_spread ? `${item.spread_percent.toFixed(1)}%` : "N/A";
                    const dailySalesText = item.has_daily_sales ? formatNumber(item.daily_sales_volume) : "N/A";
                    const buyOrdersText = item.has_order_book ? formatNumber(item.buy_order_count) : "N/A";
                    const weeklyAverageText = item.has_weekly_average ? formatPrice(item.weekly_average_price, state, item) : "N/A";
                    const style = { "--rarity-color": meta.color } as React.CSSProperties;

                    return (
                        <button
                            key={`best-sell-${item.item_id}`}
                            type="button"
                            className="best-sell-card"
                            style={style}
                            onClick={() => OpenMarketListing(item.item_id)}
                            aria-label={itemName}
                        >
                            <div className="best-sell-item-main">
                                {item.icon_url ? (
                                    <img src={item.icon_url} alt={itemName} className="best-sell-icon" />
                                ) : (
                                    <div className="best-sell-icon best-sell-icon-fallback">
                                        {itemName.charAt(0).toLocaleUpperCase(currentLanguage)}
                                    </div>
                                )}
                                <div className="best-sell-copy">
                                    <div className="best-sell-name">
                                        <span>{itemName}</span>
                                        <ExternalLink className="w-3 h-3" />
                                    </div>
                                    <div className="best-sell-price">
                                        <span>{formatPrice(item.suggested, state, item)}</span>
                                        <span>{t("dashboard.weekly_avg", "Weekly avg")} {weeklyAverageText}</span>
                                    </div>
                                </div>
                                <div className="best-sell-score">
                                    <span>{score}</span>
                                    <small>{t("dashboard.score", "Score")}</small>
                                </div>
                            </div>

                            <div className="best-sell-signals">
                                <span>{t("dashboard.daily_sales", "Daily sales")} <strong>{dailySalesText}</strong></span>
                                <span>{t("dashboard.spread", "Spread")} <strong>{spreadText}</strong></span>
                                <span>{t("dashboard.buy_orders", "Buy orders")} <strong>{buyOrdersText}</strong></span>
                            </div>

                            {(item.sell_reasons || []).length > 0 && (
                                <div className="best-sell-reasons">
                                    {(item.sell_reasons || []).slice(0, 4).map((reason) => (
                                        <span key={`${item.item_id}-${reason}`}>{bestSellReasonLabel(reason, t, currentLanguage)}</span>
                                    ))}
                                </div>
                            )}
                        </button>
                    );
                })
            )}
        </div>
    );
}

export default BestItemsToSellNowContent;
