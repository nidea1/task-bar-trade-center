import {
    TURKISH_RARITY_LABELS,
    TURKISH_TYPE_LABELS,
    TURKISH_GEAR_LABELS
} from '../constants';
import { formatTokenLabel, rarityMeta } from './helpers';

export function isTurkishLanguage(currentLanguage: string): boolean {
    return currentLanguage.toLowerCase().startsWith("tr");
}

export function localizedFallback(currentLanguage: string, turkish: string, english: string): string {
    return isTurkishLanguage(currentLanguage) ? turkish : english;
}

export function tokenFallback(value: string, labels: Record<string, string>, currentLanguage: string): string {
    if (isTurkishLanguage(currentLanguage) && labels[value]) {
        return labels[value];
    }
    return formatTokenLabel(value);
}

export function translateRarity(
    value: string,
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): string {
    if (!value) return "";
    const meta = rarityMeta(value);
    return t(meta.labelKey, tokenFallback(value, TURKISH_RARITY_LABELS, currentLanguage));
}

export function translateItemCategory(
    value: string,
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): string {
    if (isItemTypeToken(value)) {
        return translateItemType(value, t, currentLanguage);
    }
    return translateGear(value, t, currentLanguage);
}

export function translateItemType(
    value: string,
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): string {
    if (!value) return "";
    return t(`type.${value}`, tokenFallback(value, TURKISH_TYPE_LABELS, currentLanguage));
}

export function translateGear(
    value: string,
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): string {
    if (!value) return "";
    return t(`gear.${value}`, tokenFallback(value, TURKISH_GEAR_LABELS, currentLanguage));
}

export function isItemTypeToken(value: string): boolean {
    return value === "GEAR" || value === "MATERIAL" || value === "STAGEBOX";
}

export function bestSellReasonLabel(
    reason: string,
    t: (key: string, fallback: string) => string,
    currentLanguage: string
): string {
    const fallback = (() => {
        switch (reason) {
            case "high_daily_sales":
                return localizedFallback(currentLanguage, "Yüksek günlük satış", "High daily sales");
            case "narrow_spread":
                return localizedFallback(currentLanguage, "Dar makas", "Narrow spread");
            case "high_buy_orders":
                return localizedFallback(currentLanguage, "Çok sayıda alış emri", "High buy orders");
            case "high_confidence":
                return localizedFallback(currentLanguage, "Yüksek güvenilirlik", "High confidence");
            case "above_weekly_average":
                return localizedFallback(currentLanguage, "Haftalık ortalamanın üstünde", "Above weekly average");
            default:
                return formatTokenLabel(reason);
        }
    })();
    return t(`dashboard.sell_reason.${reason}`, fallback);
}
