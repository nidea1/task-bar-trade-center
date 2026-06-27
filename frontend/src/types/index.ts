export type ThemeMode = "dark" | "light";
export type PriceMode = "suggested" | "instant";
export type SortMode = "price_desc" | "price_asc" | "name_asc" | "count_desc" | "rarity_desc";
export type MarketableItemsTab = "best" | "all";

export interface DashboardSettings {
    theme_mode: ThemeMode;
    price_mode: PriceMode;
    rarity_filter: string;
    equipment_filter: string;
    sort_mode: SortMode;
    marketable_items_tab: MarketableItemsTab;
}

export interface RarityMeta {
    rank: number;
    color: string;
    labelKey: string;
}

export interface DashboardItem {
    item_id: number;
    name: string;
    market_hash_name: string;
    market_url: string;
    icon_url?: string;
    grade: string;
    type: string;
    gear: string;
    count: number;
    location: string;
    equipped: boolean;
    suggested: number;
    instant: number;
    total_suggested: number;
    total_instant: number;
    has_price: boolean;
    weekly_average_price: number;
    spread_percent: number;
    daily_sales_volume: number;
    buy_order_count: number;
    sell_order_count: number;
    has_weekly_average: boolean;
    has_spread: boolean;
    has_daily_sales: boolean;
    has_order_book: boolean;
    confidence: string;
    has_confidence: boolean;
    sell_score: number;
    sell_reasons?: string[];
    price_prefix: string;
    price_suffix: string;
    updated_at: string;
}

export interface DashboardFooterInfo {
    app_name: string;
    app_short_name: string;
    version: string;
    creator_name: string;
    update_status: number;
    update_text: string;
    update_available: boolean;
    release_url: string;
}

export interface DashboardState {
    updated_at: string;
    snapshot_read_at: string;
    market_scope: string;
    currency_code: string;
    price_prefix: string;
    price_suffix: string;
    gold: number;
    totals: {
        suggested_listing_value: number;
        instant_sell_value: number;
        inventory_value: number;
        stash_value: number;
        equipped_value: number;
        hero_equipped_values: Record<number, number>;
        stash_page_values: Record<number, number>;
        stash_page_counts?: Record<number, number>;
        stash_page_count?: number;
        priced_item_count: number;
        unknown_item_count: number;
        marketable_item_count: number;
        total_item_count: number;
    };
    items: DashboardItem[];
    most_valuable: DashboardItem[];
    best_to_sell_now?: DashboardItem[];
    duplicates: DashboardItem[];
    equipped: DashboardItem[];
    missing_prices: DashboardItem[];
    refresh: {
        refreshing: boolean;
        queued: number;
        completed: number;
        estimated_remaining_seconds?: number;
        backoff_until?: string;
        last_error?: string;
    };
    translations?: Record<string, string>;
}

export interface DropdownOption {
    value: string;
    label: string;
    color?: string;
}
