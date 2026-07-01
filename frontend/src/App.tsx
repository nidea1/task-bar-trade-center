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
    InstallAvailableUpdate,
    GetDashboardSettings,
    GetRuntimeState,
    GetTranslations,
    GetMinRarityNotify,
    SetDashboardSettings,
    SetMinRarityNotify,
    DisableDashboardHotkey,
    EnableDashboardHotkey,
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
    Bell,
    ListChecks,
    PackageOpen,
    Hammer,
    FlaskConical,
    HandHeart,
    Keyboard,
    Settings,
    Maximize,
    AlertTriangle,
    Clock,
    Download
} from 'lucide-react';

import { HERO_CLASSES, appIcon } from './constants';
import {
    ThemeMode,
    PriceMode,
    SortMode,
    BestOwnershipFilter,
    BestSortMode,
    MarketableItemsTab,
    NotificationSource,
    DashboardSettings,
    DashboardState,
    DashboardFooterInfo,
    RuntimeStateInfo
} from './types';
import {
    readStoredThemeMode,
    formatPrice,
    rarityTokenOptions,
    rarityMeta,
    itemCategoryOptions,
    filterAndSortItems,
    filterAndSortBestItems,
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
import PreparingScreen from './components/PreparingScreen';

const notificationSourceOrder: NotificationSource[] = ["box", "craft", "synthesis", "offering"];
const allNotificationSources = notificationSourceOrder.join(",");
const noNotificationSources = "none";
const updateStatusDownloading = 5;

const defaultDashboardSettings: DashboardSettings = {
    theme_mode: "dark",
    price_mode: "suggested",
    rarity_filter: "all",
    equipment_filter: "all",
    sort_mode: "price_desc",
    best_rarity_filter: "all",
    best_equipment_filter: "all",
    best_ownership_filter: "all",
    best_sort_mode: "score_desc",
    marketable_items_tab: "all",
    notify_sources: allNotificationSources,
    hotkey_modifiers: 0,
    hotkey_vk: 0x71,
    game_scale: 100,
};

type DashboardSettingsInput = {
    theme_mode?: string;
    price_mode?: string;
    rarity_filter?: string;
    equipment_filter?: string;
    sort_mode?: string;
    best_rarity_filter?: string;
    best_equipment_filter?: string;
    best_ownership_filter?: string;
    best_sort_mode?: string;
    marketable_items_tab?: string;
    notify_sources?: string;
    hotkey_modifiers?: number;
    hotkey_vk?: number;
    game_scale?: number;
};

function normalizeNotifySources(value?: string | null): string {
    if (!value || !value.trim()) return allNotificationSources;
    const tokens = value.split(",").map((token) => token.trim().toLowerCase());
    if (tokens.includes(noNotificationSources)) return noNotificationSources;
    const enabled = notificationSourceOrder.filter((source) => tokens.includes(source));
    return enabled.length > 0 ? enabled.join(",") : noNotificationSources;
}

function notifySourceSet(value: string): Set<NotificationSource> {
    const normalized = normalizeNotifySources(value);
    if (normalized === noNotificationSources) return new Set();
    return new Set(normalized.split(",") as NotificationSource[]);
}

function getVKName(vk: number): string {
    if (vk >= 0x70 && vk <= 0x87) {
        return "F" + (vk - 0x70 + 1);
    }
    if (vk >= 0x30 && vk <= 0x39) {
        return String.fromCharCode(vk);
    }
    if (vk >= 0x41 && vk <= 0x5A) {
        return String.fromCharCode(vk);
    }
    switch (vk) {
        case 0x20: return "Space";
        case 0x09: return "Tab";
        case 0x1B: return "Esc";
        case 0x0D: return "Enter";
        case 0x08: return "Backspace";
        case 0x2E: return "Delete";
        case 0x2D: return "Insert";
        case 0x24: return "Home";
        case 0x23: return "End";
        case 0x21: return "PgUp";
        case 0x22: return "PgDn";
        default: return "Key 0x" + vk.toString(16).toUpperCase();
    }
}

function formatHotkey(modifiers: number, vk: number): string {
    const parts: string[] = [];
    if (modifiers & 2) parts.push("Ctrl");
    if (modifiers & 1) parts.push("Alt");
    if (modifiers & 4) parts.push("Shift");
    if (modifiers & 8) parts.push("Win");
    
    parts.push(getVKName(vk));
    return parts.join(" + ");
}

function normalizeDashboardSettings(settings?: DashboardSettingsInput | null): DashboardSettings {
    const sortMode = settings?.sort_mode;
    const bestOwnershipFilter = settings?.best_ownership_filter;
    const bestSortMode = settings?.best_sort_mode;
    const gameScale = settings?.game_scale;
    return {
        theme_mode: settings?.theme_mode === "light" ? "light" : defaultDashboardSettings.theme_mode,
        price_mode: settings?.price_mode === "instant" ? "instant" : defaultDashboardSettings.price_mode,
        rarity_filter: settings?.rarity_filter || defaultDashboardSettings.rarity_filter,
        equipment_filter: settings?.equipment_filter || defaultDashboardSettings.equipment_filter,
        sort_mode: (
            sortMode === "price_asc"
            || sortMode === "name_asc"
            || sortMode === "count_desc"
            || sortMode === "rarity_desc"
        ) ? sortMode : defaultDashboardSettings.sort_mode,
        best_rarity_filter: settings?.best_rarity_filter || defaultDashboardSettings.best_rarity_filter,
        best_equipment_filter: settings?.best_equipment_filter || defaultDashboardSettings.best_equipment_filter,
        best_ownership_filter: (
            bestOwnershipFilter === "equipped"
            || bestOwnershipFilter === "unequipped"
        ) ? bestOwnershipFilter : defaultDashboardSettings.best_ownership_filter,
        best_sort_mode: (
            bestSortMode === "score_asc"
            || bestSortMode === "price_desc"
            || bestSortMode === "price_asc"
            || bestSortMode === "name_asc"
            || bestSortMode === "rarity_desc"
        ) ? bestSortMode : defaultDashboardSettings.best_sort_mode,
        marketable_items_tab: settings?.marketable_items_tab === "best" ? "best" : defaultDashboardSettings.marketable_items_tab,
        notify_sources: normalizeNotifySources(settings?.notify_sources),
        hotkey_modifiers: settings?.hotkey_modifiers ?? 0,
        hotkey_vk: settings?.hotkey_vk ?? 0x71,
        game_scale: (gameScale === 100 || gameScale === 125 || gameScale === 150) ? gameScale : defaultDashboardSettings.game_scale,
    };
}

function dashboardSettingsKey(settings: DashboardSettings): string {
    return JSON.stringify(settings);
}

function App() {
    const [state, setState] = useState<DashboardState | null>(null);
    const [runtimeState, setRuntimeState] = useState<RuntimeStateInfo | null>(null);
    const [translations, setTranslations] = useState<Record<string, string>>({});
    const [error, setError] = useState<string>("");
    const [refreshing, setRefreshing] = useState(false);
    const [forceRefreshing, setForceRefreshing] = useState(false);
    const [activeRefreshKind, setActiveRefreshKind] = useState<"smart" | "force" | null>(null);

    const [languages, setLanguages] = useState<{ code: string; name: string }[]>([]);
    const [regions, setRegions] = useState<{ country_code: string; name: string; currency_code: string }[]>([]);
    const [currentLanguage, setCurrentLanguage] = useState<string>("en-US");
    const [currentMarketScope, setCurrentMarketScope] = useState<{ currency_code: string; country_code: string } | null>(null);
    const [footerInfo, setFooterInfo] = useState<DashboardFooterInfo | null>(null);
    const [dismissedUpdatePromptKey, setDismissedUpdatePromptKey] = useState("");
    const [installingUpdate, setInstallingUpdate] = useState(false);
    const [minRarityNotify, setMinRarityNotifyState] = useState<string>("COMMON");
    const [themeMode, setThemeMode] = useState<ThemeMode>(() => readStoredThemeMode());
    const [priceMode, setPriceMode] = useState<PriceMode>("suggested");
    const [rarityFilter, setRarityFilter] = useState("all");
    const [equipmentFilter, setEquipmentFilter] = useState("all");
    const [sortMode, setSortMode] = useState<SortMode>("price_desc");
    const [bestRarityFilter, setBestRarityFilter] = useState("all");
    const [bestEquipmentFilter, setBestEquipmentFilter] = useState("all");
    const [bestOwnershipFilter, setBestOwnershipFilter] = useState<BestOwnershipFilter>("all");
    const [bestSortMode, setBestSortMode] = useState<BestSortMode>("score_desc");
    const [marketableItemsTab, setMarketableItemsTab] = useState<MarketableItemsTab>("all");
    const [notifySources, setNotifySources] = useState<string>(allNotificationSources);
    const [notifySourceMenuOpen, setNotifySourceMenuOpen] = useState(false);
    const [itemSearch, setItemSearch] = useState("");
    const [bestItemSearch, setBestItemSearch] = useState("");
    const [hotkeyModifiers, setHotkeyModifiers] = useState<number>(0);
    const [hotkeyVK, setHotkeyVK] = useState<number>(0x71); // default F2
    const [isListeningHotkey, setIsListeningHotkey] = useState<boolean>(false);
    const [settingsMenuOpen, setSettingsMenuOpen] = useState(false);
    const [gameScale, setGameScale] = useState<number>(100);
    const settingsMenuRef = useRef<HTMLDivElement>(null);
    const mountedRef = useRef(false);
    const loadInFlightRef = useRef(false);
    const dashboardSettingsLoadedRef = useRef(false);
    const dashboardSettingsSnapshotRef = useRef("");
    const notifySourceMenuRef = useRef<HTMLDivElement>(null);

    const t = (key: string, fallback?: string) => {
        if (translations[key]) {
            return translations[key];
        }
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

    const loadRuntimeState = () => {
        return GetRuntimeState()
            .then((info: RuntimeStateInfo | null) => {
                if (mountedRef.current) setRuntimeState(info);
            })
            .catch(() => {
                // The dashboard can continue polling inventory state if runtime state is temporarily unavailable.
            });
    };

    const loadTranslations = () => {
        return GetTranslations()
            .then((nextTranslations: Record<string, string> | null | undefined) => {
                if (mountedRef.current) setTranslations(nextTranslations || {});
            })
            .catch(() => {
                // Fallback strings keep the preparing screen usable.
            });
    };

    useEffect(() => {
        mountedRef.current = true;
        load();
        const timer = window.setInterval(load, 3000);
        loadRuntimeState();
        loadTranslations();
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
        GetDashboardSettings()
            .then((settings: DashboardSettingsInput | null) => {
                if (!mountedRef.current) return;
                const normalized = normalizeDashboardSettings(settings);
                const storedThemeMode = readStoredThemeMode();
                const nextThemeMode = normalized.theme_mode === "dark" && storedThemeMode === "light"
                    ? storedThemeMode
                    : normalized.theme_mode;
                const hydratedSettings: DashboardSettings = {
                    ...normalized,
                    theme_mode: nextThemeMode,
                };
                dashboardSettingsSnapshotRef.current = dashboardSettingsKey(hydratedSettings);
                setThemeMode(nextThemeMode);
                setPriceMode(normalized.price_mode);
                setRarityFilter(normalized.rarity_filter);
                setEquipmentFilter(normalized.equipment_filter);
                setSortMode(normalized.sort_mode);
                setBestRarityFilter(normalized.best_rarity_filter);
                setBestEquipmentFilter(normalized.best_equipment_filter);
                setBestOwnershipFilter(normalized.best_ownership_filter);
                setBestSortMode(normalized.best_sort_mode);
                setMarketableItemsTab(normalized.marketable_items_tab);
                setNotifySources(normalized.notify_sources);
                setHotkeyModifiers(normalized.hotkey_modifiers);
                setHotkeyVK(normalized.hotkey_vk);
                setGameScale(normalized.game_scale);
                dashboardSettingsLoadedRef.current = true;
            })
            .catch(() => {
                // Dashboard preferences are non-critical; current defaults remain usable.
            });

        const unsubscribe = EventsOn("inventory-dashboard-updated", (nextState: unknown) => {
            if (!mountedRef.current) return;
            setState(nextState as unknown as DashboardState);
            setError("");
        });
        const unsubscribeFooter = EventsOn("dashboard-footer-updated", (info: unknown) => {
            if (!mountedRef.current) return;
            setFooterInfo(info as DashboardFooterInfo);
        });
        const unsubscribeRuntime = EventsOn("runtime-state-updated", (info: unknown) => {
            if (!mountedRef.current) return;
            setRuntimeState(info as RuntimeStateInfo);
        });

        return () => {
            mountedRef.current = false;
            window.clearInterval(timer);
            window.clearInterval(footerTimer);
            unsubscribe();
            unsubscribeFooter();
            unsubscribeRuntime();
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
        if (!notifySourceMenuOpen) return;
        const handleClickOutside = (event: MouseEvent) => {
            if (notifySourceMenuRef.current && !notifySourceMenuRef.current.contains(event.target as Node)) {
                setNotifySourceMenuOpen(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, [notifySourceMenuOpen]);

    useEffect(() => {
        if (!settingsMenuOpen) return;
        const handleClickOutside = (event: MouseEvent) => {
            if (settingsMenuRef.current && !settingsMenuRef.current.contains(event.target as Node)) {
                if (isListeningHotkey) return;
                setSettingsMenuOpen(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, [settingsMenuOpen, isListeningHotkey]);

    useEffect(() => {
        if (!settingsMenuOpen && isListeningHotkey) {
            setIsListeningHotkey(false);
            EnableDashboardHotkey().catch(() => {});
        }
    }, [settingsMenuOpen, isListeningHotkey]);

    useEffect(() => {
        if (!dashboardSettingsLoadedRef.current) return;
        const nextSettings: DashboardSettings = {
            theme_mode: themeMode,
            price_mode: priceMode,
            rarity_filter: rarityFilter,
            equipment_filter: equipmentFilter,
            sort_mode: sortMode,
            best_rarity_filter: bestRarityFilter,
            best_equipment_filter: bestEquipmentFilter,
            best_ownership_filter: bestOwnershipFilter,
            best_sort_mode: bestSortMode,
            marketable_items_tab: marketableItemsTab,
            notify_sources: notifySources,
            hotkey_modifiers: hotkeyModifiers,
            hotkey_vk: hotkeyVK,
            game_scale: gameScale,
        };
        const nextKey = dashboardSettingsKey(nextSettings);
        if (nextKey === dashboardSettingsSnapshotRef.current) return;
        SetDashboardSettings(nextSettings).then((saved) => {
            dashboardSettingsSnapshotRef.current = dashboardSettingsKey(normalizeDashboardSettings(saved as DashboardSettingsInput));
        }).catch(() => {
            // Local UI state can continue even if settings persistence fails.
        });
    }, [themeMode, priceMode, rarityFilter, equipmentFilter, sortMode, bestRarityFilter, bestEquipmentFilter, bestOwnershipFilter, bestSortMode, marketableItemsTab, notifySources, hotkeyModifiers, hotkeyVK, gameScale]);

    useEffect(() => {
        if (!isListeningHotkey) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            e.preventDefault();
            e.stopPropagation();

            const standaloneModifiers = [16, 17, 18, 91, 92, 93, 224];
            if (standaloneModifiers.includes(e.keyCode)) {
                return;
            }

            if (e.keyCode === 27 && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
                setIsListeningHotkey(false);
                EnableDashboardHotkey().catch(() => {});
                return;
            }

            let modifiers = 0;
            if (e.altKey) modifiers |= 1;
            if (e.ctrlKey) modifiers |= 2;
            if (e.shiftKey) modifiers |= 4;
            if (e.metaKey) modifiers |= 8;

            const vk = e.keyCode;

            setHotkeyModifiers(modifiers);
            setHotkeyVK(vk);
            setIsListeningHotkey(false);
        };

        window.addEventListener("keydown", handleKeyDown, true);
        return () => {
            window.removeEventListener("keydown", handleKeyDown, true);
        };
    }, [isListeningHotkey]);

    useEffect(() => {
        if (!state?.refresh?.refreshing && !refreshing && !forceRefreshing) {
            setActiveRefreshKind(null);
        }
    }, [state?.refresh?.refreshing, refreshing, forceRefreshing]);

    useEffect(() => {
        if (footerInfo && footerInfo.update_status !== updateStatusDownloading) {
            setInstallingUpdate(false);
        }
    }, [footerInfo?.update_status]);

    const handleMinRarityNotifyChange = (grade: string) => {
        setMinRarityNotifyState(grade);
        SetMinRarityNotify(grade);
    };

    const handleClose = () => {
        if (isListeningHotkey) {
            setIsListeningHotkey(false);
            EnableDashboardHotkey().catch(() => {});
        }
        WindowHide();
    };

    const handleMinimize = () => {
        if (isListeningHotkey) {
            setIsListeningHotkey(false);
            EnableDashboardHotkey().catch(() => {});
        }
        WindowMinimise();
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
    const bestSellItems = state?.best_to_sell_now || [];
    const bestRarityOptions = useMemo(
        () => rarityTokenOptions(bestSellItems.map((item) => item.grade), t, currentLanguage),
        [bestSellItems, state?.translations, currentLanguage]
    );
    const bestEquipmentOptions = useMemo(
        () => itemCategoryOptions(bestSellItems.map((item) => equipmentFilterValue(item)).filter(Boolean), t, currentLanguage),
        [bestSellItems, state?.translations, currentLanguage]
    );
    const filteredBestSellItems = useMemo(
        () => filterAndSortBestItems(bestSellItems, bestRarityFilter, bestEquipmentFilter, bestOwnershipFilter, bestSortMode, priceMode, bestItemSearch),
        [bestSellItems, bestRarityFilter, bestEquipmentFilter, bestOwnershipFilter, bestSortMode, priceMode, bestItemSearch]
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
    const controlGameScaleLabel = t("dashboard.control_game_scale", localizedFallback(currentLanguage, "Oyun Pencere Ölçeği", "Game Window Scale"));
    const gameScaleLabel = `${gameScale / 100}x`;
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
    const notifySourceOptions: Array<{ value: NotificationSource; label: string; icon: JSX.Element }> = [
        { value: "box", label: t("dashboard.notify_source_box", localizedFallback(currentLanguage, "Sandık", "Box")), icon: <PackageOpen className="w-3.5 h-3.5" /> },
        { value: "craft", label: t("dashboard.notify_source_craft", localizedFallback(currentLanguage, "Üretim", "Craft")), icon: <Hammer className="w-3.5 h-3.5" /> },
        { value: "synthesis", label: t("dashboard.notify_source_synthesis", localizedFallback(currentLanguage, "Sentez", "Synthesis")), icon: <FlaskConical className="w-3.5 h-3.5" /> },
        { value: "offering", label: t("dashboard.notify_source_offering", localizedFallback(currentLanguage, "Adak", "Offering")), icon: <HandHeart className="w-3.5 h-3.5" /> },
    ];
    const enabledNotifySources = notifySourceSet(notifySources);
    const enabledNotifySourceLabels = notifySourceOptions
        .filter((option) => enabledNotifySources.has(option.value))
        .map((option) => option.label);
    const notifySourcesLabel = t("dashboard.notify_sources_label", localizedFallback(currentLanguage, "Kaynaklar", "Sources"));
    const notifySourcesSummary = enabledNotifySourceLabels.length === notificationSourceOrder.length
        ? t("dashboard.notify_sources_all", localizedFallback(currentLanguage, "Tümü", "All"))
        : enabledNotifySourceLabels.length === 0
            ? t("dashboard.notify_sources_none", localizedFallback(currentLanguage, "Kapalı", "Off"))
            : enabledNotifySourceLabels.join(", ");
    const notifySourcesTitle = `${notifySourcesLabel}: ${notifySourcesSummary}`;
    const toggleNotifySource = (source: NotificationSource) => {
        const next = notifySourceSet(notifySources);
        if (next.has(source)) {
            next.delete(source);
        } else {
            next.add(source);
        }
        const normalized = notificationSourceOrder.filter((value) => next.has(value));
        setNotifySources(normalized.length > 0 ? normalized.join(",") : noNotificationSources);
    };
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
            loadTranslations();
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
    const runtimeReady = runtimeState?.ready === true;
    const displayedRefreshKind = activeRefreshKind || "smart";
    const isCurrentlyRefreshing = refreshQueueRunning || refreshing || forceRefreshing;
    const normalRefreshBusy = refreshing || (refreshQueueRunning && displayedRefreshKind === "smart");
    const forceRefreshBusy = forceRefreshing || (refreshQueueRunning && displayedRefreshKind === "force");
    const isWaitingForInventory = !runtimeReady || !state?.updated_at;
    const updatePromptKey = footerInfo?.update_available
        ? (footerInfo.release_url || footerInfo.update_text || "available")
        : "";
    const showUpdatePrompt = !!footerInfo?.update_available
        && updatePromptKey !== dismissedUpdatePromptKey
        && !installingUpdate;
    const dismissUpdatePrompt = () => {
        setDismissedUpdatePromptKey(updatePromptKey);
    };
    const installUpdateNow = () => {
        setInstallingUpdate(true);
        InstallAvailableUpdate()
            .then((started: boolean) => {
                if (!mountedRef.current) return;
                if (!started) {
                    setInstallingUpdate(false);
                    setError(t(
                        "dialog.update_available.install_failed",
                        localizedFallback(currentLanguage, "Güncelleme başlatılamadı.", "Could not start the update.")
                    ));
                    loadFooterInfo();
                }
            })
            .catch((err: unknown) => {
                if (!mountedRef.current) return;
                setInstallingUpdate(false);
                setError(String(err));
            });
    };

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
                        onClick={handleMinimize}
                        className="themed-tooltip-host p-1 hover:bg-[#1a1410] rounded text-[#9a896f] hover:text-[#ffbe2d] transition-colors cursor-pointer"
                        aria-label="Minimize"
                        data-tooltip="Minimize"
                    >
                        <Minus className="w-3.5 h-3.5" />
                    </button>
                    <button
                        onClick={handleClose}
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
                {!runtimeReady ? (
                    <PreparingScreen
                        runtimeState={runtimeState}
                        currentLanguage={currentLanguage}
                        t={t}
                    />
                ) : (
                    <>

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
                            <div className="dashboard-selector-row flex items-center gap-2 relative">
                                <button
                                    disabled={!runtimeReady || isCurrentlyRefreshing}
                                    onClick={() => refreshPrices(false)}
                                    className="dashboard-refresh-button themed-tooltip-host game-button flex items-center justify-center gap-2 px-4 py-1.5 font-medium text-xs uppercase cursor-pointer shrink-0"
                                    data-tooltip={smartRefreshTooltip}
                                    aria-label={smartRefreshTooltip}
                                >
                                    <RefreshCw className={`w-3.5 h-3.5 ${normalRefreshBusy ? 'animate-spin' : ''}`} />
                                    {normalRefreshBusy ? t("dashboard.refreshing", "Refreshing...") : smartRefreshLabel}
                                </button>
                                <button
                                    disabled={!runtimeReady || isCurrentlyRefreshing}
                                    onClick={() => refreshPrices(true)}
                                    className="dashboard-refresh-button dashboard-force-refresh-button themed-tooltip-host game-button flex items-center justify-center gap-2 px-4 py-1.5 font-medium text-xs uppercase cursor-pointer shrink-0"
                                    data-tooltip={forceRefreshTooltip}
                                    aria-label={forceRefreshTooltip}
                                >
                                    <RefreshCw className={`w-3.5 h-3.5 ${forceRefreshBusy ? 'animate-spin' : ''}`} />
                                    {forceRefreshBusy ? t("dashboard.refreshing", "Refreshing...") : forceRefreshLabel}
                                </button>

                                <div className="relative inline-block text-left" ref={settingsMenuRef}>
                                    <button
                                        type="button"
                                        onClick={() => setSettingsMenuOpen((open) => !open)}
                                        className={`dashboard-settings-toggle themed-tooltip-host game-button flex items-center justify-center w-8 h-[30px] p-0 cursor-pointer ${settingsMenuOpen ? 'is-active' : ''}`}
                                        aria-label={t("dashboard.control_settings", "Settings")}
                                        data-tooltip={t("dashboard.control_settings", "Settings")}
                                    >
                                        <Settings className="w-4 h-4" />
                                    </button>

                                    {settingsMenuOpen && (
                                        <div
                                            className="dashboard-settings-popover game-panel bg-[#0c0a0f]/98 border-2 border-[#54493b] shadow-2xl rounded p-4 space-y-4 no-drag"
                                            role="menu"
                                        >
                                            <h3 className="text-xs font-bold uppercase tracking-wider gold-text border-b border-[#463d30]/60 pb-2 mb-3">
                                                {t("dashboard.settings_title", "Settings")}
                                            </h3>
                                            
                                            <div className="space-y-3">
                                                {/* Hotkey */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.control_hotkey", "Hotkey")}:</span>
                                                    <button
                                                        type="button"
                                                        onClick={() => {
                                                            setIsListeningHotkey(true);
                                                            DisableDashboardHotkey().catch(() => {});
                                                        }}
                                                        className={`dashboard-hotkey-toggle themed-tooltip-host game-button flex items-center justify-between gap-2.5 px-3 py-1.5 text-xs font-bold ${isListeningHotkey ? 'is-listening animate-pulse border-[#ffbe2d] text-[#ffbe2d]' : ''}`}
                                                        aria-label={t("dashboard.hotkey_tooltip", "System-wide hotkey to toggle the dashboard")}
                                                    >
                                                        <span className="dashboard-dropdown-content">
                                                            <Keyboard className="w-3.5 h-3.5" />
                                                            <span className="dashboard-dropdown-label">
                                                                <span className="dashboard-dropdown-value">
                                                                    {isListeningHotkey ? t("dashboard.hotkey_listening", "Press Key...") : formatHotkey(hotkeyModifiers, hotkeyVK)}
                                                                </span>
                                                            </span>
                                                        </span>
                                                        <span className="text-[8px] text-[#ffbe2d]/60">⚙</span>
                                                    </button>
                                                </div>

                                                {/* Theme */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.control_theme", "Theme")}:</span>
                                                    <GameDropdown
                                                        value={themeMode}
                                                        options={[
                                                            { value: "dark", label: t("dashboard.theme_dark", "Dark") },
                                                            { value: "light", label: t("dashboard.theme_light", "Light") },
                                                        ]}
                                                        onChange={(val) => setThemeMode(val as ThemeMode)}
                                                        className="dashboard-control dashboard-theme-toggle"
                                                        selectedLabel={themeLabel}
                                                        icon={themeMode === "dark" ? <Moon className="w-3.5 h-3.5" /> : <Sun className="w-3.5 h-3.5 text-[#ffbe2d]" />}
                                                        title={`${controlThemeLabel}: ${themeLabel}`}
                                                    />
                                                </div>

                                                {/* Game Scale */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{controlGameScaleLabel}:</span>
                                                    <GameDropdown
                                                        value={String(gameScale)}
                                                        options={[
                                                            { value: "100", label: "1x" },
                                                            { value: "125", label: "1.25x" },
                                                            { value: "150", label: "1.5x" },
                                                        ]}
                                                        onChange={(val) => setGameScale(Number(val))}
                                                        className="dashboard-control dashboard-scale-toggle"
                                                        selectedLabel={gameScaleLabel}
                                                        icon={<Maximize className="w-3.5 h-3.5" />}
                                                        title={`${controlGameScaleLabel}: ${gameScaleLabel}`}
                                                    />
                                                </div>

                                                {/* Language */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.control_language", "Language")}:</span>
                                                    <GameDropdown
                                                        value={currentLanguage}
                                                        options={languages.map((lang) => ({ value: lang.code, label: lang.name }))}
                                                        onChange={handleLanguageChange}
                                                        className="dashboard-control dashboard-control-language"
                                                        selectedLabel={selectedLanguageName}
                                                        icon={<Languages className="w-3.5 h-3.5" />}
                                                        title={`${controlLanguageLabel}: ${selectedLanguageName}`}
                                                    />
                                                </div>

                                                {/* Region/Currency */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.control_currency", "Currency")}:</span>
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
                                                        selectedLabel={currentMarketScope?.currency_code || ""}
                                                        icon={<GoldIcon className="w-3.5 h-3.5" />}
                                                        title={marketDisplayName}
                                                    />
                                                </div>

                                                {/* Price Type */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.control_price", "Price")}:</span>
                                                    <GameDropdown
                                                        value={priceMode}
                                                        options={[
                                                            { value: "suggested", label: t("dashboard.price_lowest_sell", "Lowest Sell") },
                                                            { value: "instant", label: t("dashboard.price_highest_buy", "Highest Buy") },
                                                        ]}
                                                        onChange={(val) => setPriceMode(val as PriceMode)}
                                                        className="dashboard-control dashboard-control-price"
                                                        selectedLabel={compactPriceLabel}
                                                        icon={<Tag className="w-3.5 h-3.5" />}
                                                        title={`${controlPriceLabel}: ${fullPriceLabel}`}
                                                    />
                                                </div>

                                                {/* Notify Threshold */}
                                                <div className="dashboard-settings-row flex items-center justify-between gap-4">
                                                    <span className="text-[11px] font-bold text-[#e1d5bf]">{t("dashboard.rarity_notify_label", "Notify")}:</span>
                                                    <GameDropdown
                                                        value={minRarityNotify}
                                                        options={rarityGrades.map((grade) => ({
                                                            value: grade,
                                                            label: t("rarity." + grade, grade) + "+",
                                                            color: rarityMeta(grade).color
                                                        }))}
                                                        onChange={handleMinRarityNotifyChange}
                                                        className="dashboard-control dashboard-notify-toggle"
                                                        selectedLabel={selectedRarityNotifyLabel}
                                                        icon={<Bell className="w-3.5 h-3.5" />}
                                                        title={rarityNotifyTitle}
                                                        ariaLabel={`${rarityNotifyTitle}. ${controlRarityNotifyTooltip}`}
                                                    />
                                                </div>

                                                {/* Notify Sources */}
                                                <div className="flex flex-col gap-1.5 pt-2 border-t border-[#463d30]/40">
                                                    <span className="text-[11px] font-bold text-[#caa66a] uppercase tracking-wider">{t("dashboard.notify_sources_label", "Sources")}:</span>
                                                    <div className="grid grid-cols-2 gap-2">
                                                        {notifySourceOptions.map((option) => {
                                                            const enabled = enabledNotifySources.has(option.value);
                                                            return (
                                                                <button
                                                                    key={option.value}
                                                                    type="button"
                                                                    role="menuitemcheckbox"
                                                                    aria-checked={enabled}
                                                                    onClick={() => toggleNotifySource(option.value)}
                                                                    className={`dashboard-notify-source-option flex items-center gap-2 p-1.5 rounded border border-[#463d30]/40 text-left transition-colors hover:bg-[#1a1620] cursor-pointer ${enabled ? "is-enabled text-[#ffbe2d] border-[#ffbe2d]/60" : "text-[#9a896f]"}`}
                                                                >
                                                                    <span className="dashboard-notify-source-icon text-xs">{option.icon}</span>
                                                                    <span className="text-[10px] font-bold tracking-wide">{option.label}</span>
                                                                </button>
                                                            );
                                                        })}
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </div>
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
                {runtimeReady && error && (
                    <div className="game-alert-error flex items-start gap-3 p-4 rounded">
                        <Archive className="w-5 h-5 shrink-0 mt-0.5 text-[#f05046]" />
                        <div>
                            <h4 className="font-bold text-xs text-[#ffbe2d] uppercase">Connection Error</h4>
                            <p className="text-[11px] mt-1 text-[#e1d5bf]">{error}</p>
                        </div>
                    </div>
                )}

                {/* ═══ Syncing Status Bar ═══ */}
                {runtimeReady && state?.refresh?.refreshing && (
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

                {runtimeReady && state?.refresh?.last_error && (
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
                                bestItems={filteredBestSellItems}
                                bestTotalCount={bestSellItems.length}
                                items={filteredItems}
                                totalCount={allItems.length}
                                state={state}
                                currentLanguage={currentLanguage}
                                priceMode={priceMode}
                                bestRarityFilter={bestRarityFilter}
                                bestEquipmentFilter={bestEquipmentFilter}
                                bestOwnershipFilter={bestOwnershipFilter}
                                bestSortMode={bestSortMode}
                                rarityFilter={rarityFilter}
                                equipmentFilter={equipmentFilter}
                                sortMode={sortMode}
                                bestRarityOptions={bestRarityOptions}
                                bestEquipmentOptions={bestEquipmentOptions}
                                rarityOptions={rarityOptions}
                                equipmentOptions={equipmentOptions}
                                bestSearchTerm={bestItemSearch}
                                searchTerm={itemSearch}
                                onBestRarityChange={setBestRarityFilter}
                                onBestEquipmentChange={setBestEquipmentFilter}
                                onBestOwnershipChange={(value) => setBestOwnershipFilter(value as BestOwnershipFilter)}
                                onBestSortChange={(value) => setBestSortMode(value as BestSortMode)}
                                onBestSearchChange={setBestItemSearch}
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
                    </>
                )}
            </div>
            {showUpdatePrompt && (
                <div className="update-prompt-backdrop no-drag" role="dialog" aria-modal="true" aria-labelledby="update-prompt-title">
                    <div className="update-prompt-panel game-panel">
                        <div className="game-header update-prompt-header">
                            <AlertTriangle className="update-prompt-header-icon" />
                            <div className="min-w-0">
                                <h2 id="update-prompt-title" className="update-prompt-title">
                                    {t("dialog.update_available.title", localizedFallback(currentLanguage, "Güncelleme var", "Update available"))}
                                </h2>
                                <p className="update-prompt-version" title={footerInfo?.update_text || ""}>{footerInfo?.update_text}</p>
                            </div>
                        </div>
                        <div className="update-prompt-body">
                            <p>
                                {t(
                                    "dialog.update_available.dashboard_body",
                                    localizedFallback(
                                        currentLanguage,
                                        "Yeni TBTC sürümü hazır. Şimdi yükleyebilir ya da bu sürümle devam edip daha sonra tray menüsünden güncelleyebilirsin.",
                                        "A new TBTC version is ready. Install it now or keep using this version and update later from the tray menu."
                                    )
                                )}
                            </p>
                            <div className="update-prompt-actions">
                                <button type="button" className="game-button update-prompt-secondary" onClick={dismissUpdatePrompt}>
                                    <Clock className="update-prompt-button-icon" />
                                    <span>{t("dialog.update_available.later", localizedFallback(currentLanguage, "Sonra Güncelle", "Update Later"))}</span>
                                </button>
                                <button type="button" className="game-button update-prompt-primary" onClick={installUpdateNow}>
                                    <Download className="update-prompt-button-icon" />
                                    <span>{t("dialog.update_available.now", localizedFallback(currentLanguage, "Şimdi Güncelle", "Update Now"))}</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
            <DashboardFooter
                info={footerInfo}
                isRefreshing={!runtimeReady || isCurrentlyRefreshing}
                currentLanguage={currentLanguage}
                t={t}
                syncTimeText={syncTimeText}
            />
        </main>
    );
}

export default App;
