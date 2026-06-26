import {useEffect, useMemo, useState, useRef} from 'react';
import './App.css';
import {
    GetInventoryDashboard,
    OpenMarketListing,
    RefreshInventoryPrices,
    GetDisplayLanguages,
    GetMarketRegions,
    GetCurrentLanguage,
    GetCurrentMarketScope,
    SetDisplayLanguage,
    SetMarketScope,
} from "../wailsjs/go/main/App";
import {
    WindowMinimise,
    WindowHide,
    EventsOn
} from "../wailsjs/runtime";
import {
    TrendingUp,
    Zap,
    Archive,
    Shield,
    CheckCircle,
    RefreshCw,
    Languages,
    Sun,
    Moon,
    Tag,
    Search,
    AlertCircle,
    ExternalLink,
    Package,
    Layers,
    AlertTriangle,
    Minus,
    X,
    Coins as GoldIcon,
    Target,
    Flame,
    Heart,
    Compass,
    Swords
} from 'lucide-react';

import knightGif from './assets/images/heroes/Hero_101.gif';
import rangerGif from './assets/images/heroes/Hero_201.gif';
import sorcererGif from './assets/images/heroes/Hero_301.gif';
import priestGif from './assets/images/heroes/Hero_401.gif';
import hunterGif from './assets/images/heroes/Hero_501.gif';
import slayerGif from './assets/images/heroes/Hero_601.gif';
import appIcon from './assets/images/appicon.png';

const HERO_CLASSES = [
    { id: 1, name: "Knight", key: "hero.knight", color: "text-[#e3a943]", gif: knightGif },
    { id: 2, name: "Ranger", key: "hero.ranger", color: "text-[#54dc5e]", gif: rangerGif },
    { id: 3, name: "Sorcerer", key: "hero.sorcerer", color: "text-[#c09ee6]", gif: sorcererGif },
    { id: 4, name: "Priest", key: "hero.priest", color: "text-[#ff8da1]", gif: priestGif },
    { id: 5, name: "Hunter", key: "hero.hunter", color: "text-[#e87e30]", gif: hunterGif },
    { id: 6, name: "Slayer", key: "hero.slayer", color: "text-[#f05046]", gif: slayerGif }
];

type ThemeMode = "dark" | "light";
type PriceMode = "suggested" | "instant";
type SortMode = "price_desc" | "price_asc" | "name_asc" | "count_desc" | "rarity_desc";

const TURKISH_RARITY_LABELS: Record<string, string> = {
    COMMON: "Yaygın",
    UNCOMMON: "Yaygın Olmayan",
    RARE: "Nadir",
    LEGENDARY: "Efsanevi",
    IMMORTAL: "Ölümsüz",
    ARCANA: "Gizemli",
    BEYOND: "Ötesi",
    CELESTIAL: "Göksel",
    DIVINE: "İlahi",
    COSMIC: "Kozmik",
};

const TURKISH_TYPE_LABELS: Record<string, string> = {
    GEAR: "Ekipman",
    MATERIAL: "Malzeme",
    STAGEBOX: "Aşama Kutusu",
};

const TURKISH_GEAR_LABELS: Record<string, string> = {
    AMULET: "Muska",
    ARMOR: "Zırh",
    ARROW: "Ok",
    AXE: "Balta",
    BOLT: "Arbalet Oku",
    BOOTS: "Çizme",
    BOW: "Yay",
    BRACER: "Bileklik",
    CROSSBOW: "Arbalet",
    EARING: "Küpe",
    GLOVES: "Eldiven",
    HATCHET: "Balta",
    HELMET: "Miğfer",
    ORB: "Küre",
    RING: "Yüzük",
    SCEPTER: "Asa",
    SHIELD: "Kalkan",
    STAFF: "Değnek",
    SWORD: "Kılıç",
    TOME: "Kitap",
};

type RarityMeta = {
    rank: number;
    color: string;
    labelKey: string;
};

const DEFAULT_RARITY_META: RarityMeta = {
    rank: -1,
    color: "rgb(90, 90, 90)",
    labelKey: "rarity.UNKNOWN",
};

const RARITY_META: Record<string, RarityMeta> = {
    COMMON: { rank: 0, color: "rgb(63, 63, 63)", labelKey: "rarity.COMMON" },
    UNCOMMON: { rank: 1, color: "rgb(100, 171, 67)", labelKey: "rarity.UNCOMMON" },
    RARE: { rank: 2, color: "rgb(68, 127, 207)", labelKey: "rarity.RARE" },
    LEGENDARY: { rank: 3, color: "rgb(200, 109, 28)", labelKey: "rarity.LEGENDARY" },
    IMMORTAL: { rank: 4, color: "rgb(205, 67, 67)", labelKey: "rarity.IMMORTAL" },
    ARCANA: { rank: 5, color: "rgb(172, 93, 212)", labelKey: "rarity.ARCANA" },
    BEYOND: { rank: 6, color: "rgb(235, 83, 134)", labelKey: "rarity.BEYOND" },
    CELESTIAL: { rank: 7, color: "rgb(163, 218, 235)", labelKey: "rarity.CELESTIAL" },
    DIVINE: { rank: 8, color: "rgb(241, 228, 191)", labelKey: "rarity.DIVINE" },
    COSMIC: { rank: 9, color: "rgb(37, 150, 190)", labelKey: "rarity.COSMIC" },
};

const getHeroIcon = (id: number) => {
    switch (id) {
        case 1: return <Shield className="w-4 h-4" />;
        case 2: return <Target className="w-4 h-4" />;
        case 3: return <Flame className="w-4 h-4" />;
        case 4: return <Heart className="w-4 h-4" />;
        case 5: return <Compass className="w-4 h-4" />;
        case 6: return <Swords className="w-4 h-4" />;
        default: return <Shield className="w-4 h-4" />;
    }
};

type DashboardItem = {
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
    price_prefix: string;
    price_suffix: string;
    updated_at: string;
}

type DashboardState = {
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
    duplicates: DashboardItem[];
    equipped: DashboardItem[];
    missing_prices: DashboardItem[];
    refresh: {
        refreshing: boolean;
        queued: number;
        completed: number;
        backoff_until?: string;
        last_error?: string;
    };
    translations?: Record<string, string>;
}

function App() {
    const [state, setState] = useState<DashboardState | null>(null);
    const [error, setError] = useState<string>("");
    const [refreshing, setRefreshing] = useState(false);

    const [languages, setLanguages] = useState<{code: string; name: string}[]>([]);
    const [regions, setRegions] = useState<{country_code: string; name: string; currency_code: string}[]>([]);
    const [currentLanguage, setCurrentLanguage] = useState<string>("en-US");
    const [currentMarketScope, setCurrentMarketScope] = useState<{currency_code: string; country_code: string} | null>(null);
    const [themeMode, setThemeMode] = useState<ThemeMode>(() => readStoredThemeMode());
    const [priceMode, setPriceMode] = useState<PriceMode>("suggested");
    const [rarityFilter, setRarityFilter] = useState("all");
    const [equipmentFilter, setEquipmentFilter] = useState("all");
    const [sortMode, setSortMode] = useState<SortMode>("price_desc");
    const [itemSearch, setItemSearch] = useState("");
    const mountedRef = useRef(false);
    const loadInFlightRef = useRef(false);

    const t = (key: string, fallback?: string) => {
        if (state?.translations && state.translations[key]) {
            return state.translations[key];
        }
        return fallback !== undefined ? fallback : key;
    };

    const load = () => {
        if (loadInFlightRef.current) {
            return Promise.resolve();
        }
        loadInFlightRef.current = true;
        return GetInventoryDashboard()
            .then((nextState: unknown) => {
                if (!mountedRef.current) return;
                setState(nextState as unknown as DashboardState);
                setError("");
            })
            .catch((err: unknown) => {
                if (mountedRef.current) {
                    setError(String(err));
                }
            })
            .finally(() => {
                loadInFlightRef.current = false;
            });
    };

    useEffect(() => {
        mountedRef.current = true;
        load();
        const timer = window.setInterval(load, 10000);
        
        GetDisplayLanguages().then((list: {code: string; name: string}[] | null | undefined) => {
            if (mountedRef.current) setLanguages(list || []);
        });
        GetMarketRegions().then((list: {country_code: string; name: string; currency_code: string}[] | null | undefined) => {
            if (mountedRef.current) setRegions(list || []);
        });
        GetCurrentLanguage().then((lang: string) => {
            if (mountedRef.current) setCurrentLanguage(lang);
        });
        GetCurrentMarketScope().then((scope: {currency_code: string; country_code: string} | null) => {
            if (mountedRef.current) setCurrentMarketScope(scope);
        });

        const unsubscribe = EventsOn("inventory-dashboard-updated", (nextState: unknown) => {
            if (!mountedRef.current) return;
            setState(nextState as unknown as DashboardState);
            setError("");
        });

        return () => {
            mountedRef.current = false;
            window.clearInterval(timer);
            unsubscribe();
        };
    }, []);

    useEffect(() => {
        if (currentLanguage) {
            const langCode = currentLanguage.split('-')[0].toLowerCase();
            document.documentElement.lang = langCode;
        }
    }, [currentLanguage]);

    useEffect(() => {
        document.documentElement.dataset.theme = themeMode;
        window.localStorage.setItem("dashboard-theme", themeMode);
    }, [themeMode]);

    const totals = state?.totals;
    const allItems = state?.items || [];
    const rarityOptions = useMemo(
        () => rarityTokenOptions(allItems.map((item) => item.grade), t, currentLanguage),
        [allItems, state?.translations, currentLanguage]
    );
    const equipmentOptions = useMemo(
        () => itemCategoryOptions(allItems.map((item) => equipmentFilterValue(item)).filter(Boolean), t, currentLanguage),
        [allItems, state?.translations, currentLanguage]
    );
    const filteredItems = useMemo(
        () => filterAndSortItems(allItems, rarityFilter, equipmentFilter, sortMode, priceMode, itemSearch),
        [allItems, rarityFilter, equipmentFilter, sortMode, priceMode, itemSearch]
    );
    const displayedTotalValue = priceMode === "instant"
        ? totals?.instant_sell_value
        : totals?.suggested_listing_value;
    const themeLabel = themeMode === "dark"
        ? t("dashboard.theme_dark", "Dark")
        : t("dashboard.theme_light", "Light");
    const controlLanguageLabel = t("dashboard.control_language", localizedFallback(currentLanguage, "Dil", "Language"));
    const controlCurrencyLabel = t("dashboard.control_currency", localizedFallback(currentLanguage, "Para", "Currency"));
    const controlPriceLabel = t("dashboard.control_price", localizedFallback(currentLanguage, "Fiyat", "Price"));
    const controlThemeLabel = t("dashboard.control_theme", localizedFallback(currentLanguage, "Tema", "Theme"));
    const selectedLanguageName = languages.find((lang) => lang.code === currentLanguage)?.name || currentLanguage;
    const selectedLanguageCode = languageDisplayCode(currentLanguage);
    const fullPriceLabel = priceMode === "instant"
        ? t("dashboard.price_highest_buy", "Highest Buy")
        : t("dashboard.price_lowest_sell", "Lowest Sell");
    const compactPriceLabel = priceMode === "instant"
        ? t("dashboard.price_buy_short", localizedFallback(currentLanguage, "Alış", "Buy"))
        : t("dashboard.price_sell_short", localizedFallback(currentLanguage, "Satış", "Sell"));
    const marketDisplayName = state?.market_scope || "STEAM MARKET OVERLAY";
    const searchPlaceholder = t("dashboard.search_items", localizedFallback(currentLanguage, "Eşya ara", "Search items"));
    const staleText = useMemo(() => {
        if (!state?.updated_at) return t("dashboard.waiting_state", "Waiting for inventory state...");
        return t("dashboard.updated_relative", "Updated %s").replace("%s", formatRelativeTime(state.updated_at, t));
    }, [state?.updated_at, state?.translations]);

    const refreshPrices = () => {
        setRefreshing(true);
        RefreshInventoryPrices()
            .then(load)
            .catch((err: unknown) => {
                if (mountedRef.current) {
                    setError(String(err));
                }
            })
            .finally(() => {
                if (mountedRef.current) {
                    setRefreshing(false);
                }
            });
    };

    const handleLanguageChange = (code: string) => {
        SetDisplayLanguage(code).then(() => {
            if (!mountedRef.current) return;
            setCurrentLanguage(code);
            load();
        });
    };

    const handleRegionChange = (currencyCode: string, countryCode: string) => {
        SetMarketScope(currencyCode, countryCode).then(() => {
            if (!mountedRef.current) return;
            setCurrentMarketScope({ currency_code: currencyCode, country_code: countryCode });
            load();
        });
    };

    const isCurrentlyRefreshing = state?.refresh?.refreshing || refreshing;
    const isWaitingForInventory = !state?.updated_at;

    return (
        <main className={`dashboard-shell theme-${themeMode} h-screen bg-[#030304] text-[#e1d5bf] flex flex-col select-none overflow-hidden`}>
            {/* ═══ Window Drag Bar (Borderless Window Header) ═══ */}
            <div className="w-full flex items-center justify-between px-4 py-2 window-drag-bar drag-area shrink-0">
                <div className="flex items-center gap-2">
                    <img src={appIcon} alt="App Icon" className="w-4 h-4 object-contain" />
                    <span className="text-[10px] font-bold tracking-wider text-[#9a896f] uppercase">Task Bar Trade Center</span>
                </div>
                <div className="flex items-center gap-1.5 no-drag">
                    <button 
                        onClick={WindowMinimise} 
                        className="p-1 hover:bg-[#1a1410] rounded text-[#9a896f] hover:text-[#ffbe2d] transition-colors cursor-pointer"
                        title="Minimize"
                    >
                        <Minus className="w-3.5 h-3.5" />
                    </button>
                    <button 
                        onClick={WindowHide} 
                        className="p-1 hover:bg-[#601b18] hover:text-white rounded text-[#9a896f] transition-colors cursor-pointer"
                        title="Close"
                    >
                        <X className="w-3.5 h-3.5" />
                    </button>
                </div>
            </div>

            {/* ═══ Main Content Area (Scrollable) ═══ */}
            <div className="flex-1 overflow-y-auto p-4 md:p-6 space-y-6">
                
                {/* ═══ Top Dashboard Header Panel ═══ */}
                <header className="game-panel !overflow-visible">
                    <div className="game-header dashboard-header">
                        <div className="dashboard-brand">
                            <div className="dashboard-logo-frame">
                                <img src={appIcon} alt="App Logo" className="dashboard-logo" />
                            </div>
                            <div className="min-w-0">
                                <h1 className="dashboard-title text-lg font-bold tracking-wider gold-text uppercase">
                                    <span className="dashboard-title-full">Task Bar Trade Center</span>
                                    <span className="dashboard-title-short">TBTC</span>
                                </h1>
                                <p className="dashboard-market text-[10px] text-[#9a896f] font-medium tracking-wide">
                                    {marketDisplayName}
                                </p>
                            </div>
                        </div>

                        <div className="dashboard-controls no-drag">
                            <div className="dashboard-selector-row">
                                <button
                                    type="button"
                                    onClick={() => setThemeMode(themeMode === "dark" ? "light" : "dark")}
                                    className="dashboard-theme-toggle game-button flex items-center justify-center"
                                    aria-label={`${controlThemeLabel}: ${themeLabel}`}
                                    title={`${controlThemeLabel}: ${themeLabel}`}
                                >
                                    {themeMode === "dark" ? <Moon className="dashboard-theme-icon" /> : <Sun className="dashboard-theme-icon" />}
                                </button>

                                <GameDropdown
                                    value={currentLanguage}
                                    options={languages.map((lang) => ({ value: lang.code, label: lang.name }))}
                                    onChange={handleLanguageChange}
                                    className="dashboard-control dashboard-control-language"
                                    prefix={controlLanguageLabel}
                                    selectedLabel={selectedLanguageCode}
                                    icon={<Languages className="w-3.5 h-3.5" />}
                                    title={`${controlLanguageLabel}: ${selectedLanguageName}`}
                                />

                                <GameDropdown
                                    value={currentMarketScope ? `${currentMarketScope.currency_code}:${currentMarketScope.country_code}` : ""}
                                    options={regions.map((reg) => ({
                                        value: `${reg.currency_code}:${reg.country_code}`,
                                        label: `${reg.currency_code} - ${reg.name}`
                                    }))}
                                    onChange={(val) => {
                                        const [currency, country] = val.split(":");
                                        handleRegionChange(currency, country);
                                    }}
                                    className="dashboard-control dashboard-control-region"
                                    selectedLabel={controlCurrencyLabel}
                                    icon={<GoldIcon className="w-3.5 h-3.5" />}
                                    title={marketDisplayName}
                                />

                                <GameDropdown
                                    value={priceMode}
                                    options={[
                                        { value: "suggested", label: t("dashboard.price_lowest_sell", "Lowest Sell") },
                                        { value: "instant", label: t("dashboard.price_highest_buy", "Highest Buy") },
                                    ]}
                                    onChange={(val) => setPriceMode(val as PriceMode)}
                                    className="dashboard-control dashboard-control-price"
                                    prefix={controlPriceLabel}
                                    selectedLabel={compactPriceLabel}
                                    icon={<Tag className="w-3.5 h-3.5" />}
                                    title={`${controlPriceLabel}: ${fullPriceLabel}`}
                                />
                            </div>

                            <div className="dashboard-refresh-row">
                                <span className="dashboard-updated text-[11px] text-[#9a896f] font-mono">
                                    {staleText}
                                </span>
                                <button
                                    disabled={isCurrentlyRefreshing}
                                    onClick={refreshPrices}
                                    className="dashboard-refresh-button game-button flex items-center justify-center gap-2 px-4 py-1.5 font-medium text-xs uppercase cursor-pointer"
                                >
                                    <RefreshCw className={`w-3.5 h-3.5 ${isCurrentlyRefreshing ? 'animate-spin' : ''}`} />
                                    {isCurrentlyRefreshing ? t("dashboard.refreshing", "Refreshing...") : t("dashboard.refresh_prices", "Refresh Prices")}
                                </button>
                            </div>
                        </div>
                    </div>

                    {/* Gold accent line */}
                    <div className="game-accent-line" />

                    {/* Stash/Inventory, Hero Gear and Total display bar */}
                    {isWaitingForInventory ? (
                        <DashboardValueBarSkeleton />
                    ) : (
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between px-4 py-2.5 bg-[#08080a] text-xs gap-3">
                        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                            <div className="flex items-center gap-2">
                                <Archive className="w-4 h-4 text-[#ffbe2d]" />
                                <span className="font-bold text-[#e1d5bf]" title={formatPrice((totals?.stash_value ?? 0) + (totals?.inventory_value ?? 0), state)}>{formatPrice((totals?.stash_value ?? 0) + (totals?.inventory_value ?? 0), state)}</span>
                                <span className="text-[10px] text-[#9a896f] uppercase tracking-wider">{t("dashboard.stash_inventory", "Stash/Inventory")}</span>
                            </div>
                            <span className="text-[#463d30] hidden sm:inline">│</span>
                            <div className="flex items-center gap-2">
                                <Shield className="w-4 h-4 text-[#c09ee6]" />
                                <span className="font-bold text-[#e1d5bf]" title={formatPrice(totals?.equipped_value, state)}>{formatPrice(totals?.equipped_value, state)}</span>
                                <span className="text-[10px] text-[#9a896f] uppercase tracking-wider">{t("dashboard.hero_gear", "Hero Gear")}</span>
                            </div>
                            <span className="text-[#463d30] hidden sm:inline">│</span>
                            <div className="flex items-center gap-2">
                                <TrendingUp className="w-4 h-4 text-[#54dc5e]" />
                                <span className="font-bold text-[#e1d5bf]" title={formatPrice(displayedTotalValue, state)}>{formatPrice(displayedTotalValue, state)}</span>
                                <span className="text-[10px] text-[#9a896f] uppercase tracking-wider">{t("dashboard.total_value", "Total Value")}</span>
                            </div>
                        </div>
                        <div className="flex items-center gap-4 text-[10px] text-[#9a896f]">
                            <span>{totals?.total_item_count ?? 0} {t("dashboard.items_uppercase", "ITEMS")}</span>
                            <span className="text-[#463d30]">│</span>
                            <span>{totals?.priced_item_count ?? 0}/{totals?.marketable_item_count ?? 0} {t("dashboard.priced_uppercase", "PRICED")}</span>
                        </div>
                    </div>
                    )}
                </header>

                {/* ═══ Error Notifications ═══ */}
                {error && (
                    <div className="game-alert-error flex items-start gap-3 p-4 rounded">
                        <AlertCircle className="w-5 h-5 shrink-0 mt-0.5 text-[#f05046]" />
                        <div>
                            <h4 className="font-bold text-xs text-[#ffbe2d] uppercase">Connection Error</h4>
                            <p className="text-[11px] mt-1 text-[#e1d5bf]">{error}</p>
                        </div>
                    </div>
                )}

                {/* ═══ Syncing Status Bar ═══ */}
                {state?.refresh?.refreshing && (
                    <div className="game-panel p-3 bg-[#08080a] flex items-center gap-2.5 text-xs text-[#ffbe2d] animate-pulse">
                        <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#ffbe2d]" />
                        <span>{formatPricingUpdateText(t("dashboard.updating_pricing_data", "Updating pricing data: %d done, %d remaining"), state.refresh.completed, state.refresh.queued)}</span>
                    </div>
                )}

                {state?.refresh?.last_error && (
                    <div className="game-alert-warning flex items-start gap-3 p-4 rounded">
                        <AlertTriangle className="w-5 h-5 shrink-0 mt-0.5 text-[#ffbe2d]" />
                        <div>
                            <h4 className="font-bold text-xs text-[#ffbe2d] uppercase">API Notice</h4>
                            <p className="text-[11px] mt-1 text-[#e1d5bf]">{state.refresh.last_error}</p>
                        </div>
                    </div>
                )}

                {/* ═══ Core Metrics Grid ═══ */}
                {isWaitingForInventory ? (
                    <DashboardLoadingSkeleton title={t("dashboard.waiting_state", "Waiting for inventory state...")} />
                ) : (
                <>
                <section className="metrics-grid">
                    <MetricCard
                        label={t("dashboard.suggested_value", "Suggested Value")}
                        value={formatPrice(totals?.suggested_listing_value, state)}
                        icon={<TrendingUp className="w-4 h-4" />}
                        iconColor="text-[#54dc5e]"
                    />
                    <MetricCard
                        label={t("dashboard.instant_sell", "Instant Sell")}
                        value={formatPrice(totals?.instant_sell_value, state)}
                        icon={<Zap className="w-4 h-4" />}
                        iconColor="text-[#e87e30]"
                    />
                    <MetricCard
                        label={t("dashboard.stash_value", "Stash Value")}
                        value={formatPrice(totals?.stash_value, state)}
                        icon={<Archive className="w-4 h-4" />}
                        iconColor="text-[#ffbe2d]"
                    />
                    <MetricCard
                        label={t("dashboard.equipped_gear", "Equipped Gear")}
                        value={formatPrice(totals?.equipped_value, state)}
                        icon={<Shield className="w-4 h-4" />}
                        iconColor="text-[#c09ee6]"
                    />
                    <MetricCard
                        label={t("dashboard.items_priced", "Items Priced")}
                        value={`${totals?.priced_item_count ?? 0}/${totals?.marketable_item_count ?? 0}`}
                        icon={<CheckCircle className="w-4 h-4" />}
                        iconColor="text-[#ffbe2d]"
                    />
                </section>
 
                {/* ═══ Equipped Gear by Hero ═══ */}
                <section className="space-y-2">
                    <h3 className="text-[10px] font-bold text-[#9a896f] uppercase tracking-wider">{t("dashboard.equipped_by_hero", "Equipped Gear by Hero")}</h3>
                    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
                        {HERO_CLASSES.map((hero) => {
                            const val = totals?.hero_equipped_values?.[hero.id] || 0;
                            return (
                                <div key={hero.id} className="game-metric p-2.5 flex items-center justify-between gap-2.5">
                                    <div className="space-y-0.5 min-w-0">
                                        <span className="text-[9px] text-[#9a896f] font-bold block uppercase tracking-wider" title={t(hero.key, hero.name)}>{t(hero.key, hero.name)}</span>
                                        <strong className="text-xs font-bold text-[#e1d5bf] tracking-wide block truncate" title={formatPrice(val, state)}>
                                            {formatPrice(val, state)}
                                        </strong>
                                    </div>
                                    <div className="w-8 h-8 rounded bg-[#030304] border border-[#2d261f] shrink-0 overflow-hidden flex items-center justify-center p-0.5">
                                        <img src={hero.gif} alt={hero.name} className="w-full h-full object-contain" />
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                </section>
 
                <section className={`dashboard-items-layout ${(state?.missing_prices?.length || 0) > 0 ? "has-missing-prices" : ""}`}>
                    <AllMarketableItemsCards
                        items={filteredItems}
                        totalCount={allItems.length}
                        state={state}
                        currentLanguage={currentLanguage}
                        priceMode={priceMode}
                        rarityFilter={rarityFilter}
                        equipmentFilter={equipmentFilter}
                        sortMode={sortMode}
                        rarityOptions={rarityOptions}
                        equipmentOptions={equipmentOptions}
                        searchTerm={itemSearch}
                        onRarityChange={setRarityFilter}
                        onEquipmentChange={setEquipmentFilter}
                        onSortChange={(value) => setSortMode(value as SortMode)}
                        onSearchChange={setItemSearch}
                        searchPlaceholder={searchPlaceholder}
                        t={t}
                    />
                    {(state?.missing_prices?.length || 0) > 0 && (
                        <MissingPricesPanel title={t("dashboard.missing_prices", "Missing Prices")} items={state?.missing_prices || []} state={state} currentLanguage={currentLanguage} />
                    )}
                </section>
                </>
                )}
            </div>
        </main>
    );
}

function DashboardValueBarSkeleton() {
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

function DashboardLoadingSkeleton({ title }: { title: string }) {
    const metricSkeletons = [0, 1, 2, 3, 4];
    const heroSkeletons = [0, 1, 2, 3, 4, 5];
    const itemSkeletons = [0, 1, 2, 3, 4, 5];

    return (
        <div className="dashboard-loading-stack" role="status" aria-live="polite">
            <section className="game-panel dashboard-loading-panel">
                <div className="dashboard-loading-copy">
                    <RefreshCw className="w-4 h-4 animate-spin text-[#ffbe2d]" />
                    <span>{title}</span>
                </div>
                <div className="dashboard-loading-bars" aria-hidden="true">
                    <span className="dashboard-skeleton-line dashboard-loading-bar-wide" />
                    <span className="dashboard-skeleton-line dashboard-loading-bar" />
                </div>
            </section>

            <section className="metrics-grid" aria-hidden="true">
                {metricSkeletons.map((item) => (
                    <div key={item} className="game-metric metric-card dashboard-metric-skeleton">
                        <div className="dashboard-skeleton-stack">
                            <span className="dashboard-skeleton-line dashboard-skeleton-label" />
                            <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                        </div>
                        <span className="dashboard-skeleton-line dashboard-skeleton-tile" />
                    </div>
                ))}
            </section>

            <section className="space-y-2" aria-hidden="true">
                <span className="dashboard-skeleton-line dashboard-section-title-skeleton" />
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
                    {heroSkeletons.map((item) => (
                        <div key={item} className="game-metric dashboard-hero-skeleton">
                            <div className="dashboard-skeleton-stack">
                                <span className="dashboard-skeleton-line dashboard-skeleton-label" />
                                <span className="dashboard-skeleton-line dashboard-skeleton-value" />
                            </div>
                            <span className="dashboard-skeleton-line dashboard-skeleton-avatar" />
                        </div>
                    ))}
                </div>
            </section>

            <section className="game-panel dashboard-items-skeleton" aria-hidden="true">
                <div className="game-header dashboard-items-skeleton-header">
                    <span className="dashboard-skeleton-line dashboard-items-heading-skeleton" />
                    <span className="dashboard-skeleton-line dashboard-items-badge-skeleton" />
                </div>
                <div className="game-accent-line" />
                <div className="dashboard-items-skeleton-grid">
                    {itemSkeletons.map((item) => (
                        <div key={item} className="inventory-card dashboard-item-skeleton-card">
                            <div className="dashboard-skeleton-line dashboard-item-icon-skeleton" />
                            <div className="dashboard-skeleton-stack dashboard-item-copy-skeleton">
                                <span className="dashboard-skeleton-line dashboard-item-title-skeleton" />
                                <span className="dashboard-skeleton-line dashboard-item-meta-skeleton" />
                                <span className="dashboard-skeleton-line dashboard-item-price-skeleton" />
                            </div>
                        </div>
                    ))}
                </div>
            </section>
        </div>
    );
}

function MetricCard({
    label,
    value,
    icon,
    iconColor,
    tooltip,
    tooltipDirection = "up"
}: {
    label: string;
    value: string;
    icon: React.ReactNode;
    iconColor: string;
    tooltip?: React.ReactNode;
    tooltipDirection?: "up" | "down";
}) {
    const tooltipClass = tooltipDirection === "down"
        ? "top-full mt-2.5"
        : "bottom-full mb-2.5";
    const outerArrowClass = tooltipDirection === "down"
        ? "bottom-full border-b-[#463d30]"
        : "top-full border-t-[#463d30]";
    const innerArrowClass = tooltipDirection === "down"
        ? "bottom-[calc(100%-2px)] border-b-[#0d0b12]"
        : "top-[calc(100%-2px)] border-t-[#0d0b12]";

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
                <div className={`hidden group-hover:block absolute left-1/2 -translate-x-1/2 ${tooltipClass} p-3 bg-[#0d0b12] border-2 border-[#463d30] shadow-2xl rounded z-50 min-w-[200px] pointer-events-none transition-all duration-200`}>
                    <div className={`absolute left-1/2 -translate-x-1/2 border-8 border-transparent ${outerArrowClass}`} />
                    <div className={`absolute left-1/2 -translate-x-1/2 border-6 border-transparent ${innerArrowClass}`} />
                    {tooltip}
                </div>
            )}
        </div>
    );
}

function MissingPricesPanel({
    title,
    items,
    state,
    currentLanguage
}: {
    title: string;
    items: DashboardItem[];
    state: DashboardState | null;
    currentLanguage: string;
}) {
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

function AllMarketableItemsCards({
    items,
    totalCount,
    state,
    currentLanguage,
    priceMode,
    rarityFilter,
    equipmentFilter,
    sortMode,
    rarityOptions,
    equipmentOptions,
    searchTerm,
    onRarityChange,
    onEquipmentChange,
    onSortChange,
    onSearchChange,
    searchPlaceholder,
    t
}: {
    items: DashboardItem[];
    totalCount: number;
    state: DashboardState | null;
    currentLanguage: string;
    priceMode: PriceMode;
    rarityFilter: string;
    equipmentFilter: string;
    sortMode: SortMode;
    rarityOptions: DropdownOption[];
    equipmentOptions: DropdownOption[];
    searchTerm: string;
    onRarityChange: (value: string) => void;
    onEquipmentChange: (value: string) => void;
    onSortChange: (value: string) => void;
    onSearchChange: (value: string) => void;
    searchPlaceholder: string;
    t: (key: string, fallback: string) => string;
}) {
    return (
        <section className="game-panel flex flex-col min-h-0">
            <div className="game-header flex flex-col lg:flex-row lg:items-center justify-between gap-3">
                <div className="flex items-center justify-between gap-3">
                    <h2 className="text-xs font-bold text-[#ffbe2d] uppercase tracking-wider flex items-center gap-2">
                        <Layers className="w-3.5 h-3.5" />
                        {t("dashboard.all_marketable_items", "All Marketable Items")}
                    </h2>
                    <span className="game-badge text-[10px] font-bold px-2 py-0.5 rounded">
                        {items.length}/{totalCount}
                    </span>
                </div>
                <div className="inventory-filter-row no-drag">
                    <label className="inventory-search">
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
                        className="min-w-[128px]"
                    />
                    <GameDropdown
                        value={equipmentFilter}
                        options={[{ value: "all", label: t("dashboard.all_equipment", "All Types") }, ...equipmentOptions]}
                        onChange={onEquipmentChange}
                        className="min-w-[128px]"
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
                        className="min-w-[142px]"
                    />
                </div>
            </div>

            <div className="game-accent-line" />

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
        </section>
    );
}

function InventoryItemCard({
    item,
    state,
    currentLanguage,
    priceMode,
    t
}: {
    item: DashboardItem;
    state: DashboardState | null;
    currentLanguage: string;
    priceMode: PriceMode;
    t: (key: string, fallback: string) => string;
}) {
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

function formatPrice(value?: number, state?: DashboardState | null, item?: DashboardItem) {
    if (value === undefined || Number.isNaN(value)) return "N/A";
    let prefix = state?.price_prefix ?? item?.price_prefix ?? "$";
    let suffix = state?.price_suffix ?? item?.price_suffix ?? "";
    if (prefix === "" && suffix === "") {
        prefix = "$";
    }
    return `${prefix}${value.toFixed(2)}${suffix}`;
}

function formatPricingUpdateText(template: string, completed: number, queued: number) {
    return template.replace("%d", String(completed)).replace("%d", String(queued));
}

function languageDisplayCode(language: string) {
    const code = language.split(/[-_]/)[0] || language;
    return code.toUpperCase();
}

function formatNumber(value?: number) {
    if (value === undefined || Number.isNaN(value)) return "0";
    return new Intl.NumberFormat().format(value);
}

function formatRelativeTime(value: string, t: (key: string, fallback: string) => string) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return t("time.just_now", "just now");
    const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
    if (seconds < 60) return t("time.just_now", "just now");
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return t("time.minutes_ago", "%dm ago").replace("%d", String(minutes));
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return t("time.hours_ago", "%dh ago").replace("%d", String(hours));
    const days = Math.floor(hours / 24);
    return t("time.days_ago", "%dd ago").replace("%d", String(days));
}

function formatLocation(location: string, state: DashboardState | null) {
    if (!location) return "";
    return location
        .split(", ")
        .map((loc) => {
            const trimmed = loc.trim().toLowerCase();
            const key = `location.${trimmed}`;
            return state?.translations?.[key] || loc;
        })
        .join(", ");
}

function readStoredThemeMode(): ThemeMode {
    try {
        return window.localStorage.getItem("dashboard-theme") === "light" ? "light" : "dark";
    } catch {
        return "dark";
    }
}

function itemUnitValue(item: DashboardItem, priceMode: PriceMode) {
    return priceMode === "instant" ? item.instant : item.suggested;
}

function itemTotalValue(item: DashboardItem, priceMode: PriceMode) {
    return priceMode === "instant" ? item.total_instant : item.total_suggested;
}

function itemHasPriceForMode(item: DashboardItem, priceMode: PriceMode) {
    return item.has_price && itemUnitValue(item, priceMode) > 0;
}

function equipmentFilterValue(item: DashboardItem) {
    return item.gear || item.type || "";
}

function itemCategoryOptions(values: string[], t: (key: string, fallback: string) => string, currentLanguage: string) {
    return Array.from(new Set(values.filter(Boolean)))
        .sort((a, b) => translateItemCategory(a, t, currentLanguage).localeCompare(translateItemCategory(b, t, currentLanguage)))
        .map((value) => ({ value, label: translateItemCategory(value, t, currentLanguage) }));
}

function rarityTokenOptions(values: string[], t: (key: string, fallback: string) => string, currentLanguage: string) {
    return Array.from(new Set(values.filter(Boolean)))
        .sort((a, b) => rarityRank(a) - rarityRank(b) || formatTokenLabel(a).localeCompare(formatTokenLabel(b)))
        .map((value) => {
            return { value, label: translateRarity(value, t, currentLanguage) };
        });
}

function filterAndSortItems(
    items: DashboardItem[],
    rarityFilter: string,
    equipmentFilter: string,
    sortMode: SortMode,
    priceMode: PriceMode,
    searchTerm: string
) {
    const query = searchTerm.trim().toLowerCase();
    const filtered = items.filter((item) => {
        if (rarityFilter !== "all" && item.grade !== rarityFilter) {
            return false;
        }
        if (equipmentFilter !== "all" && equipmentFilterValue(item) !== equipmentFilter) {
            return false;
        }
        if (query) {
            const searchable = [
                item.name,
                item.market_hash_name,
                String(item.item_id),
            ].join(" ").toLowerCase();
            if (!searchable.includes(query)) {
                return false;
            }
        }
        return true;
    });

    return [...filtered].sort((a, b) => {
        switch (sortMode) {
            case "price_asc":
                return itemUnitValue(a, priceMode) - itemUnitValue(b, priceMode)
                    || itemTotalValue(a, priceMode) - itemTotalValue(b, priceMode);
            case "name_asc":
                return (a.name || a.market_hash_name).localeCompare(b.name || b.market_hash_name);
            case "count_desc":
                return b.count - a.count || itemUnitValue(b, priceMode) - itemUnitValue(a, priceMode);
            case "rarity_desc":
                return rarityRank(b.grade) - rarityRank(a.grade) || itemUnitValue(b, priceMode) - itemUnitValue(a, priceMode);
            case "price_desc":
            default:
                return itemUnitValue(b, priceMode) - itemUnitValue(a, priceMode)
                    || itemTotalValue(b, priceMode) - itemTotalValue(a, priceMode);
        }
    });
}

function rarityRank(grade: string) {
    return rarityMeta(grade).rank;
}

function rarityMeta(grade: string) {
    return RARITY_META[grade] || DEFAULT_RARITY_META;
}

function translateRarity(value: string, t: (key: string, fallback: string) => string, currentLanguage: string) {
    if (!value) return "";
    const meta = rarityMeta(value);
    return t(meta.labelKey, tokenFallback(value, TURKISH_RARITY_LABELS, currentLanguage));
}

function translateItemCategory(value: string, t: (key: string, fallback: string) => string, currentLanguage: string) {
    if (isItemTypeToken(value)) {
        return translateItemType(value, t, currentLanguage);
    }
    return translateGear(value, t, currentLanguage);
}

function translateItemType(value: string, t: (key: string, fallback: string) => string, currentLanguage: string) {
    if (!value) return "";
    return t(`type.${value}`, tokenFallback(value, TURKISH_TYPE_LABELS, currentLanguage));
}

function translateGear(value: string, t: (key: string, fallback: string) => string, currentLanguage: string) {
    if (!value) return "";
    return t(`gear.${value}`, tokenFallback(value, TURKISH_GEAR_LABELS, currentLanguage));
}

function isItemTypeToken(value: string) {
    return value === "GEAR" || value === "MATERIAL" || value === "STAGEBOX";
}

function tokenFallback(value: string, labels: Record<string, string>, currentLanguage: string) {
    if (isTurkishLanguage(currentLanguage) && labels[value]) {
        return labels[value];
    }
    return formatTokenLabel(value);
}

function localizedFallback(currentLanguage: string, turkish: string, english: string) {
    return isTurkishLanguage(currentLanguage) ? turkish : english;
}

function isTurkishLanguage(currentLanguage: string) {
    return currentLanguage.toLowerCase().startsWith("tr");
}

function formatTokenLabel(value: string) {
    return value
        .toLowerCase()
        .split("_")
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
}

interface DropdownOption {
    value: string;
    label: string;
}

function GameDropdown({
    value,
    options,
    onChange,
    className = "",
    prefix,
    selectedLabel,
    icon,
    title,
    ariaLabel
}: {
    value: string;
    options: DropdownOption[];
    onChange: (val: string) => void;
    className?: string;
    prefix?: string;
    selectedLabel?: string;
    icon?: React.ReactNode;
    title?: string;
    ariaLabel?: string;
}) {
    const [isOpen, setIsOpen] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);
    
    const selectedOption = options.find(opt => opt.value === value);
    const displayLabel = selectedLabel || (selectedOption ? selectedOption.label : value);
    const displayTitle = title || (prefix ? `${prefix}: ${displayLabel}` : displayLabel);

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
        <div ref={dropdownRef} className={`relative inline-block text-left ${className}`}>
            <button
                type="button"
                onClick={() => setIsOpen(!isOpen)}
                title={displayTitle}
                aria-label={ariaLabel || displayTitle}
                className="game-button px-3 py-1.5 text-xs font-bold cursor-pointer flex items-center justify-between gap-2.5 min-w-[120px] tracking-wide"
            >
                <span className="dashboard-dropdown-content">
                    {icon && <span className="dashboard-dropdown-icon">{icon}</span>}
                    <span className="dashboard-dropdown-label">
                        {prefix && <span className="dashboard-dropdown-prefix">{prefix}</span>}
                        <span className="dashboard-dropdown-value" title={displayTitle}>{displayLabel}</span>
                    </span>
                </span>
                <span className="text-[8px] text-[#ffbe2d] shrink-0">▼</span>
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
                                onClick={() => {
                                    onChange(opt.value);
                                    setIsOpen(false);
                                }}
                                className={`w-full text-left px-3 py-1.5 text-xs block transition-all relative ${
                                    opt.value === value
                                        ? "text-[#ffbe2d] font-bold bg-[#1e150d] border-l-2 border-[#ffbe2d]"
                                        : "text-[#e1d5bf] hover:text-white hover:bg-[#1a1410] border-l-2 border-transparent"
                                }`}
                                title={opt.label}
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

export default App;
