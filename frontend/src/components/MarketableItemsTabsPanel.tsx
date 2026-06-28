import { Search, Layers, TrendingUp } from 'lucide-react';
import {
    DashboardItem,
    DashboardState,
    PriceMode,
    SortMode,
    BestOwnershipFilter,
    BestSortMode,
    MarketableItemsTab,
    DropdownOption
} from '../types';
import { GameDropdown } from './GameDropdown';
import { BestItemsToSellNowContent } from './BestItemsToSellNowContent';
import { AllMarketableItemsContent } from './AllMarketableItemsContent';

interface MarketableItemsTabsPanelProps {
    activeTab: MarketableItemsTab;
    onTabChange: (tab: MarketableItemsTab) => void;
    bestItems: DashboardItem[];
    bestTotalCount: number;
    items: DashboardItem[];
    totalCount: number;
    state: DashboardState | null;
    currentLanguage: string;
    priceMode: PriceMode;
    bestRarityFilter: string;
    bestEquipmentFilter: string;
    bestOwnershipFilter: BestOwnershipFilter;
    bestSortMode: BestSortMode;
    rarityFilter: string;
    equipmentFilter: string;
    sortMode: SortMode;
    bestRarityOptions: DropdownOption[];
    bestEquipmentOptions: DropdownOption[];
    rarityOptions: DropdownOption[];
    equipmentOptions: DropdownOption[];
    bestSearchTerm: string;
    searchTerm: string;
    onBestRarityChange: (value: string) => void;
    onBestEquipmentChange: (value: string) => void;
    onBestOwnershipChange: (value: string) => void;
    onBestSortChange: (value: string) => void;
    onBestSearchChange: (value: string) => void;
    onRarityChange: (value: string) => void;
    onEquipmentChange: (value: string) => void;
    onSortChange: (value: string) => void;
    onSearchChange: (value: string) => void;
    searchPlaceholder: string;
    t: (key: string, fallback: string) => string;
}

export function MarketableItemsTabsPanel({
    activeTab,
    onTabChange,
    bestItems,
    bestTotalCount,
    items,
    totalCount,
    state,
    currentLanguage,
    priceMode,
    bestRarityFilter,
    bestEquipmentFilter,
    bestOwnershipFilter,
    bestSortMode,
    rarityFilter,
    equipmentFilter,
    sortMode,
    bestRarityOptions,
    bestEquipmentOptions,
    rarityOptions,
    equipmentOptions,
    bestSearchTerm,
    searchTerm,
    onBestRarityChange,
    onBestEquipmentChange,
    onBestOwnershipChange,
    onBestSortChange,
    onBestSearchChange,
    onRarityChange,
    onEquipmentChange,
    onSortChange,
    onSearchChange,
    searchPlaceholder,
    t
}: MarketableItemsTabsPanelProps) {
    return (
        <section className="game-panel marketable-tabs-panel flex flex-col min-h-0">
            <div className="game-header marketable-tabs-header">
                <div className="marketable-tabs-list" role="tablist" aria-label={t("dashboard.all_marketable_items", "All Marketable Items")}>
                    <button
                        type="button"
                        role="tab"
                        aria-selected={activeTab === "best"}
                        disabled={bestTotalCount === 0}
                        onClick={() => onTabChange("best")}
                        className={`marketable-tab ${activeTab === "best" ? "is-active" : ""}`}
                    >
                        <TrendingUp className="w-3.5 h-3.5" />
                        <span>{t("dashboard.best_items_to_sell_now", "Best Items to Sell Now")}</span>
                        <strong>{bestItems.length === bestTotalCount ? bestTotalCount : `${bestItems.length}/${bestTotalCount}`}</strong>
                    </button>
                    <button
                        type="button"
                        role="tab"
                        aria-selected={activeTab === "all"}
                        onClick={() => onTabChange("all")}
                        className={`marketable-tab ${activeTab === "all" ? "is-active" : ""}`}
                    >
                        <Layers className="w-3.5 h-3.5" />
                        <span>{t("dashboard.all_marketable_items", "All Marketable Items")}</span>
                        <strong>{items.length}/{totalCount}</strong>
                    </button>
                </div>

                {activeTab === "best" && (
                    <div className="inventory-filter-row inventory-filter-row-best no-drag">
                        <label className="inventory-search inventory-filter-search">
                            <Search className="w-3.5 h-3.5" />
                            <input
                                value={bestSearchTerm}
                                onChange={(event) => onBestSearchChange(event.target.value)}
                                placeholder={searchPlaceholder}
                            />
                        </label>
                        <GameDropdown
                            value={bestRarityFilter}
                            options={[{ value: "all", label: t("dashboard.all_rarities", "All Rarities") }, ...bestRarityOptions]}
                            onChange={onBestRarityChange}
                            className="inventory-filter-control"
                        />
                        <GameDropdown
                            value={bestEquipmentFilter}
                            options={[{ value: "all", label: t("dashboard.all_equipment", "All Types") }, ...bestEquipmentOptions]}
                            onChange={onBestEquipmentChange}
                            className="inventory-filter-control"
                        />
                        <GameDropdown
                            value={bestOwnershipFilter}
                            options={[
                                { value: "all", label: t("dashboard.all_equipped_states", "All States") },
                                { value: "equipped", label: t("dashboard.equipped", "Equipped") },
                                { value: "unequipped", label: t("dashboard.unequipped", "Unequipped") },
                            ]}
                            onChange={onBestOwnershipChange}
                            className="inventory-filter-control"
                        />
                        <GameDropdown
                            value={bestSortMode}
                            options={[
                                { value: "score_desc", label: t("dashboard.sort_score_desc", "Score High-Low") },
                                { value: "score_asc", label: t("dashboard.sort_score_asc", "Score Low-High") },
                                { value: "price_desc", label: t("dashboard.sort_price_desc", "Unit Price High-Low") },
                                { value: "price_asc", label: t("dashboard.sort_price_asc", "Unit Price Low-High") },
                                { value: "name_asc", label: t("dashboard.sort_name_asc", "Name A-Z") },
                                { value: "rarity_desc", label: t("dashboard.sort_rarity_desc", "Rarity High-Low") },
                            ]}
                            onChange={onBestSortChange}
                            className="inventory-filter-control inventory-filter-sort"
                        />
                    </div>
                )}

                {activeTab === "all" && (
                    <div className="inventory-filter-row inventory-filter-row-all no-drag">
                        <label className="inventory-search inventory-filter-search">
                            <Search className="w-3.5 h-3.5" />
                            <input
                                value={searchTerm}
                                onChange={(event) => onSearchChange(event.target.value)}
                                placeholder={searchPlaceholder}
                            />
                        </label>
                        <GameDropdown
                            value={rarityFilter}
                            options={[{ value: "all", label: t("dashboard.all_rarities", "All Rarities") }, ...rarityOptions]}
                            onChange={onRarityChange}
                            className="inventory-filter-control"
                        />
                        <GameDropdown
                            value={equipmentFilter}
                            options={[{ value: "all", label: t("dashboard.all_equipment", "All Types") }, ...equipmentOptions]}
                            onChange={onEquipmentChange}
                            className="inventory-filter-control"
                        />
                        <GameDropdown
                            value={sortMode}
                            options={[
                                { value: "price_desc", label: t("dashboard.sort_price_desc", "Unit Price High-Low") },
                                { value: "price_asc", label: t("dashboard.sort_price_asc", "Unit Price Low-High") },
                                { value: "name_asc", label: t("dashboard.sort_name_asc", "Name A-Z") },
                                { value: "count_desc", label: t("dashboard.sort_count_desc", "Count High-Low") },
                                { value: "rarity_desc", label: t("dashboard.sort_rarity_desc", "Rarity High-Low") },
                            ]}
                            onChange={onSortChange}
                            className="inventory-filter-control inventory-filter-sort"
                        />
                    </div>
                )}
            </div>

            <div className="game-accent-line" />

            {activeTab === "best" ? (
                <BestItemsToSellNowContent
                    items={bestItems}
                    state={state}
                    currentLanguage={currentLanguage}
                    priceMode={priceMode}
                    t={t}
                />
            ) : (
                <AllMarketableItemsContent
                    items={items}
                    state={state}
                    currentLanguage={currentLanguage}
                    priceMode={priceMode}
                    t={t}
                />
            )}
        </section>
    );
}

export default MarketableItemsTabsPanel;
