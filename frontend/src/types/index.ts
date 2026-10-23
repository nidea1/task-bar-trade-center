export type ThemeMode = "dark" | "light";
export type PriceMode = "suggested" | "instant";
export type SortMode = "price_desc" | "price_asc" | "name_asc" | "count_desc" | "rarity_desc";
export type BestSortMode = "score_desc" | "score_asc" | "price_desc" | "price_asc" | "name_asc" | "rarity_desc";
export type BestOwnershipFilter = "all" | "equipped" | "unequipped";
export type MarketableItemsTab = "best" | "all";
export type NotificationSource = "box" | "craft" | "synthesis" | "offering";
export type RuntimeStepState = "pending" | "running" | "ok" | "degraded" | "failed" | string;

export interface RuntimeStepInfo {
    id: string;
    label: string;
    state: RuntimeStepState;
    message?: string;
}

export interface RuntimeStateInfo {
    ready: boolean;
    preparing: boolean;
    phase: string;
    message: string;
    app_status: number;
    app_status_text: string;
    steps: RuntimeStepInfo[];
}

export interface DashboardSettings {
	inventory_warning_minutes: number;
	notification_targets: NotificationTarget[];
    theme_mode: ThemeMode;
    price_mode: PriceMode;
    rarity_filter: string;
    equipment_filter: string;
    sort_mode: SortMode;
    best_rarity_filter: string;
    best_equipment_filter: string;
    best_ownership_filter: BestOwnershipFilter;
    best_sort_mode: BestSortMode;
    marketable_items_tab: MarketableItemsTab;
    notify_sources: string;
    hotkey_modifiers: number;
    hotkey_vk: number;
    game_scale: number;
}

export interface NotificationTarget { item_id: number; count: number; }

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
    steam_median_price: number;
    spread_percent: number;
    daily_sales_volume: number;
    steam_volume: number;
    lowest_sell_quantity: number;
    buy_order_count: number;
    sell_order_count: number;
    has_weekly_average: boolean;
    has_steam_median: boolean;
    has_spread: boolean;
    has_daily_sales: boolean;
    has_steam_volume: boolean;
    has_order_book: boolean;
    has_sell_through: boolean;
    sell_through_days: number;
    confidence: string;
    has_confidence: boolean;
    price_is_estimated: boolean;
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

export interface InventoryFillEstimate {
	next_common_open_seconds: number;
	next_stage_open_seconds: number;
	common_open_observed: boolean;
	stage_open_observed: boolean;
	region?: "normal" | "plaguelands";
    available: boolean;
    fills: boolean;
    fill_model: string;
    slot_count: number;
    used_slots: number;
    free_slots: number;
    common_drop_seconds: number;
    stage_drop_seconds: number;
    next_common_drop_seconds: number;
    next_stage_drop_seconds: number;
    common_auto_open_seconds: number;
    stage_auto_open_seconds: number;
    common_rune_reduction_seconds: number;
    stage_rune_reduction_seconds: number;
    net_fill_per_hour: number;
    time_to_full_seconds: number;
    rune_resolved: boolean;
    has_common_auto_open_rune: boolean;
    has_stage_auto_open_rune: boolean;
    common_drop_observed: boolean;
    stage_drop_observed: boolean;
    common_drop_ready: boolean;
    stage_drop_ready: boolean;
    common_chest_count: number;
    stage_chest_count: number;
}

export interface DashboardState {
	catalog?: { state: string; message?: string; item_count: number; added: number; updated_at?: string };
	notification_targets?: Array<{ item_id: number; name: string; count: number; goal: number }>;
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
        material_stash_value?: number;
        equipped_value: number;
        hero_equipped_values: Record<number, number>;
        stash_page_values: Record<number, number>;
        stash_page_counts?: Record<number, number>;
        stash_page_count?: number;
        has_material_stash?: boolean;
        material_stash_count?: number;
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
    trade_slots?: TradeShipSlot[];
    inventory_fill?: InventoryFillEstimate;
    chest_counts?: {
        normal_chests: number;
        stage_chests: number;
        total_chests: number;
    };
}

export interface TradeShipSlot {
    index: number;
    state: number;
    cooldown_until: string;
}

export interface DropdownOption {
    value: string;
    label: string;
    color?: string;
}
