import { useEffect, useMemo, useState, useRef } from 'react';
import './App.css';
import {
    GetInventoryDashboard,
    RefreshInventoryPrices,
    ForceRefreshInventoryPrices,
    GetDisplayLanguages,
    GetMarketRegions,
    GetCurrentLanguage,
    GetCurrentMarketScope,
    SetDisplayLanguage,
    SetMarketScope,
    GetDashboardFooterInfo,
    GetMinRarityNotify,
    SetMinRarityNotify,
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
    Minus,
    X,
    Coins as GoldIcon,
    Bell
} from 'lucide-react';

import { HERO_CLASSES, appIcon } from './constants';
import {
    ThemeMode,
    PriceMode,
    SortMode,
    MarketableItemsTab,
    DashboardState,
    DashboardFooterInfo
} from './types';
import {
    readStoredThemeMode,
    formatPrice,
    rarityTokenOptions,
    rarityMeta,
    itemCategoryOptions,
    filterAndSortItems,
    equipmentFilterValue,
    formatPricingUpdateText,
    formatPricingEtaText,
    languageDisplayCode
} from './utils/helpers';
import {
    localizedFallback
} from './utils/translations';

// Components
import GameDropdown from './components/GameDropdown';
import DashboardValueBarSkeleton from './components/DashboardValueBarSkeleton';
import DashboardLoadingSkeleton from './components/DashboardLoadingSkeleton';
import MetricCard from './components/MetricCard';
import MarketableItemsTabsPanel from './components/MarketableItemsTabsPanel';
import MissingPricesPanel from './components/MissingPricesPanel';
import DashboardFooter from './components/DashboardFooter';

function App() {
    const [state, setState] = useState<DashboardState | null>(null);
    const [error, setError] = useState<string>("");
    const [refreshing, setRefreshing] = useState(false);
    const [forceRefreshing, setForceRefreshing] = useState(false);
    const [activeRefreshKind, setActiveRefreshKind] = useState<"smart" | "force" | null>(null);

    const [languages, setLanguages] = useState<{ code: string; name: string }[]>([]);
    const [regions, setRegions] = useState<{ country_code: string; name: string; currency_code: string }[]>([]);
    const [currentLanguage, setCurrentLanguage] = useState<string>("en-US");
    const [currentMarketScope, setCurrentMarketScope] = useState<{ currency_code: string; country_code: string } | null>(null);
    const [footerInfo, setFooterInfo] = useState<DashboardFooterInfo | null>(null);
    const [minRarityNotify, setMinRarityNotifyState] = useState<string>("COMMON");
    const [themeMode, setThemeMode] = useState<ThemeMode>(() => readStoredThemeMode());
    const [priceMode, setPriceMode] = useState<PriceMode>("suggested");
    const [rarityFilter, setRarityFilter] = useState("all");
    const [equipmentFilter, setEquipmentFilter] = useState("all");
    const [sortMode, setSortMode] = useState<SortMode>("price_desc");
    const [marketableItemsTab, setMarketableItemsTab] = useState<MarketableItemsTab>("all");
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

    const loadFooterInfo = () => {
        return GetDashboardFooterInfo()
            .then((info: DashboardFooterInfo | null) => {
                if (mountedRef.current) setFooterInfo(info);
            })
            .catch(() => {
                // Footer metadata is non-critical; keep the last known status.
            });
    };

    useEffect(() => {
        mountedRef.current = true;
        load();
        const timer = window.setInterval(load, 3000);
        loadFooterInfo();
        const footerTimer = window.setInterval(loadFooterInfo, 15000);

        GetDisplayLanguages().then((list: { code: string; name: string }[] | null | undefined) => {
            if (mountedRef.current) setLanguages(list || []);
        });
        GetMarketRegions().then((list: { country_code: string; name: string; currency_code: string }[] | null | undefined) => {
            if (mountedRef.current) setRegions(list || []);
        });
        GetCurrentLanguage().then((lang: string) => {
            if (mountedRef.current) setCurrentLanguage(lang);
        });
        GetCurrentMarketScope().then((scope: { currency_code: string; country_code: string } | null) => {
            if (mountedRef.current) setCurrentMarketScope(scope);
        });
        GetMinRarityNotify().then((grade: string) => {
            if (mountedRef.current) setMinRarityNotifyState(grade);
        });

        const unsubscribe = EventsOn("inventory-dashboard-updated", (nextState: unknown) => {
            if (!mountedRef.current) return;
            setState(nextState as unknown as DashboardState);
            setError("");
        });

        return () => {
            mountedRef.current = false;
            window.clearInterval(timer);
            window.clearInterval(footerTimer);
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

    useEffect(() => {
        if (!state?.refresh?.refreshing && !refreshing && !forceRefreshing) {
            setActiveRefreshKind(null);
        }
    }, [state?.refresh?.refreshing, refreshing, forceRefreshing]);

    const handleMinRarityNotifyChange = (grade: string) => {
        setMinRarityNotifyState(grade);
        SetMinRarityNotify(grade);
    };

    const rarityGrades = [
        "COMMON",
        "UNCOMMON",
        "RARE",
        "LEGENDARY",
        "IMMORTAL",
        "ARCANA",
        "BEYOND",
        "CELESTIAL",
        "DIVINE",
        "COSMIC"
    ];

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
    const controlRarityNotifyLabel = t("dashboard.rarity_notify_label", localizedFallback(currentLanguage, "Bildirim", "Notify"));
    const controlRarityNotifyTooltip = t(
        "dashboard.rarity_notify_tooltip",
        localizedFallback(
            currentLanguage,
            "Sadece seçilen nadirlik ve üzerindeki satılabilir eşyalar için bildirim gönderir.",
            "Notifies only for marketable items at or above the selected rarity."
        )
    );
    const selectedRarityNotifyLabel = t("rarity." + minRarityNotify, minRarityNotify) + "+";
    const rarityNotifyTitle = `${controlRarityNotifyLabel}: ${selectedRarityNotifyLabel}`;
    const bestSellItems = state?.best_to_sell_now || [];
    const activeMarketableItemsTab: MarketableItemsTab = marketableItemsTab === "best" && bestSellItems.length === 0
        ? "all"
        : marketableItemsTab;
    const syncTimeText = useMemo(() => {
        const inventoryReadAt = state?.snapshot_read_at || state?.updated_at;
        if (!inventoryReadAt) return "";
        try {
            const d = new Date(inventoryReadAt);
            if (isNaN(d.getTime())) return "";
            const pad = (n: number) => n.toString().padStart(2, '0');
            const hours = pad(d.getHours());
            const minutes = pad(d.getMinutes());
            const seconds = pad(d.getSeconds());
            const timeStr = `${hours}:${minutes}:${seconds}`;
            const prefix = localizedFallback(currentLanguage, "Son Senkronizasyon", "Last Sync");
            return `${prefix}: ${timeStr}`;
        } catch {
            return "";
        }
    }, [state?.snapshot_read_at, state?.updated_at, currentLanguage]);

    const smartRefreshLabel = t(
        "dashboard.smart_refresh_prices",
        localizedFallback(currentLanguage, "Eksik/Eski Fiyatlar", "Smart Refresh")
    );
    const smartRefreshTooltip = t(
        "dashboard.smart_refresh_tooltip",
        localizedFallback(
            currentLanguage,
            "Sadece eksik, eski veya icon bilgisi haftalık eski olan fiyatları yeniler.",
            "Refreshes only missing, stale, or weekly stale icon/price entries."
        )
    );
    const forceRefreshLabel = t("dashboard.force_refresh_prices", "Force Refresh");
    const forceRefreshTooltip = t(
        "dashboard.force_refresh_tooltip",
        localizedFallback(
            currentLanguage,
            "Cache durumuna bakmadan tüm satılabilir eşyaları yeniden fiyatlandırır.",
            "Refetches every marketable item regardless of cache freshness."
        )
    );
    const refreshPrices = (force = false) => {
        setActiveRefreshKind(force ? "force" : "smart");
        if (force) {
            setForceRefreshing(true);
        } else {
            setRefreshing(true);
        }
        const refresh = force ? ForceRefreshInventoryPrices : RefreshInventoryPrices;
        refresh()
            .then(load)
            .catch((err: unknown) => {
                if (mountedRef.current) {
                    setError(String(err));
                }
            })
            .finally(() => {
                if (mountedRef.current) {
                    if (force) {
                        setForceRefreshing(false);
                    } else {
                        setRefreshing(false);
                    }
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

    const refreshQueueRunning = !!state?.refresh?.refreshing;
    const displayedRefreshKind = activeRefreshKind || "smart";
    const isCurrentlyRefreshing = refreshQueueRunning || refreshing || forceRefreshing;
    const normalRefreshBusy = refreshing || (refreshQueueRunning && displayedRefreshKind === "smart");
    const forceRefreshBusy = forceRefreshing || (refreshQueueRunning && displayedRefreshKind === "force");
    const isWaitingForInventory = !state?.updated_at;

    return (
        <main lang={currentLanguage.split('-')[0].toLowerCase()} className={`dashboard-shell theme-${themeMode} h-screen bg-[#030304] text-[#e1d5bf] flex flex-col select-none overflow-hidden`}>
            {/* ═══ Window Drag Bar (Borderless Window Header) ═══ */}
            <div className="w-full flex items-center justify-between px-4 py-2 window-drag-bar drag-area shrink-0">
                <div className="flex items-center gap-2">
                    <img src={appIcon} alt="App Icon" className="w-4 h-4 object-contain" />
                    <span className="text-[10px] font-bold tracking-wider text-[#9a896f] uppercase">Task Bar Trade Center</span>
                </div>
                <div className="flex items-center gap-1.5 no-drag">
                    <button
                        onClick={WindowMinimise}
                        className="themed-tooltip-host p-1 hover:bg-[#1a1410] rounded text-[#9a896f] hover:text-[#ffbe2d] transition-colors cursor-pointer"
                        aria-label="Minimize"
                        data-tooltip="Minimize"
                    >
                        <Minus className="w-3.5 h-3.5" />
                    </button>
                    <button
                        onClick={WindowHide}
                        className="themed-tooltip-host p-1 hover:bg-[#601b18] hover:text-white rounded text-[#9a896f] transition-colors cursor-pointer"
                        aria-label="Close"
                        data-tooltip="Close"
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
                                    className="dashboard-theme-toggle themed-tooltip-host game-button flex items-center justify-center"
                                    aria-label={`${controlThemeLabel}: ${themeLabel}`}
                                    data-tooltip={`${controlThemeLabel}: ${themeLabel}`}
                                >
                                    {themeMode === "dark" ? <Moon className="dashboard-theme-icon" /> : <Sun className="dashboard-theme-icon" />}
                                </button>

                                <GameDropdown
                                    value={minRarityNotify}
                                    options={rarityGrades.map((grade) => ({
                                        value: grade,
                                        label: t("rarity." + grade, grade) + "+",
                                        color: rarityMeta(grade).color
                                    }))}
                                    onChange={handleMinRarityNotifyChange}
                                    className="dashboard-notify-toggle"
                                    icon={<Bell className="w-3.5 h-3.5" />}
                                    title={rarityNotifyTitle}
                                    ariaLabel={`${rarityNotifyTitle}. ${controlRarityNotifyTooltip}`}
                                    iconOnly
                                />

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
                                    prefix={controlCurrencyLabel}
                                    selectedLabel={currentMarketScope?.currency_code || controlCurrencyLabel}
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
                                <button
                                    disabled={isCurrentlyRefreshing}
                                    onClick={() => refreshPrices(false)}
                                    className="dashboard-refresh-button themed-tooltip-host game-button flex items-center justify-center gap-2 px-4 py-1.5 font-medium text-xs uppercase cursor-pointer"
                                    data-tooltip={smartRefreshTooltip}
                                    aria-label={smartRefreshTooltip}
                                >
                                    <RefreshCw className={`w-3.5 h-3.5 ${normalRefreshBusy ? 'animate-spin' : ''}`} />
                                    {normalRefreshBusy ? t("dashboard.refreshing", "Refreshing...") : smartRefreshLabel}
                                </button>
                                <button
                                    disabled={isCurrentlyRefreshing}
                                    onClick={() => refreshPrices(true)}
                                    className="dashboard-refresh-button dashboard-force-refresh-button themed-tooltip-host game-button flex items-center justify-center gap-2 px-4 py-1.5 font-medium text-xs uppercase cursor-pointer"
                                    data-tooltip={forceRefreshTooltip}
                                    aria-label={forceRefreshTooltip}
                                >
                                    <RefreshCw className={`w-3.5 h-3.5 ${forceRefreshBusy ? 'animate-spin' : ''}`} />
                                    {forceRefreshBusy ? t("dashboard.refreshing", "Refreshing...") : forceRefreshLabel}
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
                        <Archive className="w-5 h-5 shrink-0 mt-0.5 text-[#f05046]" />
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
                        <span>
                            {formatPricingUpdateText(t("dashboard.updating_pricing_data", "Updating pricing data: %d done, %d remaining"), state.refresh.completed, state.refresh.queued)}
                            {state.refresh.estimated_remaining_seconds ? (
                                <span className="pricing-refresh-eta">
                                    {formatPricingEtaText(
                                        t("dashboard.estimated_remaining", localizedFallback(currentLanguage, "Tahmini kalan: %s", "ETA %s")),
                                        state.refresh.estimated_remaining_seconds,
                                        currentLanguage
                                    )}
                                </span>
                            ) : null}
                        </span>
                    </div>
                )}

                {state?.refresh?.last_error && (
                    <div className="game-alert-warning flex items-start gap-3 p-4 rounded">
                        <Shield className="w-5 h-5 shrink-0 mt-0.5 text-[#ffbe2d]" />
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
                            <MarketableItemsTabsPanel
                                activeTab={activeMarketableItemsTab}
                                onTabChange={setMarketableItemsTab}
                                bestItems={bestSellItems}
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
                                <MissingPricesPanel
                                    title={t("dashboard.missing_prices", "Missing Prices")}
                                    items={state?.missing_prices || []}
                                    state={state}
                                    currentLanguage={currentLanguage}
                                />
                            )}
                        </section>
                    </>
                )}
            </div>
            <DashboardFooter
                info={footerInfo}
                isRefreshing={isCurrentlyRefreshing}
                currentLanguage={currentLanguage}
                t={t}
                syncTimeText={syncTimeText}
            />
        </main>
    );
}

export default App;
