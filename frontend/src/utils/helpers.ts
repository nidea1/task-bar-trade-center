import {
    DashboardState,
    DashboardItem,
    PriceMode,
    SortMode,
    DropdownOption,
    RarityMeta,
    ThemeMode
} from '../types';
import {
    RARITY_META,
    DEFAULT_RARITY_META
} from '../constants';
import {
    isTurkishLanguage,
    translateItemCategory,
    translateRarity
} from './translations';

export function formatPrice(
    value?: number,
    state?: DashboardState | null,
    item?: DashboardItem
): string {
    if (value === undefined || Number.isNaN(value)) return "N/A";
    let prefix = state?.price_prefix ?? item?.price_prefix ?? "$";
    let suffix = state?.price_suffix ?? item?.price_suffix ?? "";
    if (prefix === "" && suffix === "") {
        prefix = "$";
    }
    return `${prefix}${value.toFixed(2)}${suffix}`;
}

export function formatPricingUpdateText(template: string, completed: number, queued: number): string {
    return template.replace("%d", String(completed)).replace("%d", String(queued));
}

export function formatPricingEtaText(template: string, seconds: number, currentLanguage: string): string {
    return template.replace("%s", formatRemainingDuration(seconds, currentLanguage));
}

export function formatRemainingDuration(seconds: number, currentLanguage: string): string {
    const total = Math.max(1, Math.ceil(seconds));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const remainingSeconds = total % 60;
    const units = isTurkishLanguage(currentLanguage)
        ? { hour: "sa", minute: "dk", second: "sn" }
        : { hour: "h", minute: "m", second: "s" };

    if (hours > 0) {
        return `${hours}${units.hour} ${minutes}${units.minute}`;
    }
    if (minutes > 0) {
        return `${minutes}${units.minute} ${remainingSeconds.toString().padStart(2, "0")}${units.second}`;
    }
    return `${total}${units.second}`;
}

export function languageDisplayCode(language: string): string {
    const code = language.split(/[-_]/)[0] || language;
    return code.toUpperCase();
}

export function formatNumber(value?: number): string {
    if (value === undefined || Number.isNaN(value)) return "0";
    return new Intl.NumberFormat().format(value);
}

export function formatRelativeTime(
    value: string,
    t: (key: string, fallback: string) => string
): string {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return t("time.just_now", "just now");
    const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
    if (seconds < 5) return t("time.just_now", "just now");
    if (seconds < 60) return t("time.seconds_ago", "%ds ago").replace("%d", String(seconds));
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return t("time.minutes_ago", "%dm ago").replace("%d", String(minutes));
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return t("time.hours_ago", "%dh ago").replace("%d", String(hours));
    const days = Math.floor(hours / 24);
    return t("time.days_ago", "%dd ago").replace("%d", String(days));
}

export function formatLocation(location: string, state: DashboardState | null): string {
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

export function readStoredThemeMode(): ThemeMode {
    try {
        return window.localStorage.getItem("dashboard-theme") === "light" ? "light" : "dark";
    } catch {
        return "dark";
    }
}

export function itemUnitValue(item: DashboardItem, priceMode: PriceMode): number {
    return priceMode === "instant" ? item.instant : item.suggested;
}

export function itemTotalValue(item: DashboardItem, priceMode: PriceMode): number {
    return priceMode === "instant" ? item.total_instant : item.total_suggested;
}

export function itemHasPriceForMode(item: DashboardItem, priceMode: PriceMode): boolean {
    return item.has_price && itemUnitValue(item, priceMode) > 0;
}

export function equipmentFilterValue(item: DashboardItem): string {
    return item.gear || item.type || "";
}

export function itemCategoryOptions(
    values: string[],
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): DropdownOption[] {
    return Array.from(new Set(values.filter(Boolean)))
        .sort((a, b) => translateItemCategory(a, t, currentLanguage).localeCompare(translateItemCategory(b, t, currentLanguage)))
        .map((value) => ({ value, label: translateItemCategory(value, t, currentLanguage) }));
}

export function rarityTokenOptions(
    values: string[],
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): DropdownOption[] {
    return Array.from(new Set(values.filter(Boolean)))
        .sort((a, b) => rarityRank(a) - rarityRank(b) || formatTokenLabel(a).localeCompare(formatTokenLabel(b)))
        .map((value) => {
            return { value, label: translateRarity(value, t, currentLanguage), color: rarityMeta(value).color };
        });
}

export function filterAndSortItems(
    items: DashboardItem[],
    rarityFilter: string,
    equipmentFilter: string,
    sortMode: SortMode,
    priceMode: PriceMode,
    searchTerm: string
): DashboardItem[] {
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

export function rarityRank(grade: string): number {
    return rarityMeta(grade).rank;
}

export function rarityMeta(grade: string): RarityMeta {
    return RARITY_META[grade] || DEFAULT_RARITY_META;
}

export function formatTokenLabel(value: string): string {
    return value
        .toLowerCase()
        .split("_")
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
}
