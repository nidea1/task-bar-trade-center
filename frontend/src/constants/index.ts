import knightGif from '../assets/images/heroes/Hero_101.gif';
import rangerGif from '../assets/images/heroes/Hero_201.gif';
import sorcererGif from '../assets/images/heroes/Hero_301.gif';
import priestGif from '../assets/images/heroes/Hero_401.gif';
import hunterGif from '../assets/images/heroes/Hero_501.gif';
import slayerGif from '../assets/images/heroes/Hero_601.gif';
import appIcon from '../assets/images/appicon.png';
import { RarityMeta } from '../types';

export { appIcon };

export const HERO_CLASSES = [
    { id: 1, name: "Knight", key: "hero.knight", color: "text-[#e3a943]", gif: knightGif },
    { id: 2, name: "Ranger", key: "hero.ranger", color: "text-[#54dc5e]", gif: rangerGif },
    { id: 3, name: "Sorcerer", key: "hero.sorcerer", color: "text-[#c09ee6]", gif: sorcererGif },
    { id: 4, name: "Priest", key: "hero.priest", color: "text-[#ff8da1]", gif: priestGif },
    { id: 5, name: "Hunter", key: "hero.hunter", color: "text-[#e87e30]", gif: hunterGif },
    { id: 6, name: "Slayer", key: "hero.slayer", color: "text-[#f05046]", gif: slayerGif }
];

export const TURKISH_RARITY_LABELS: Record<string, string> = {
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

export const TURKISH_TYPE_LABELS: Record<string, string> = {
    GEAR: "Ekipman",
    MATERIAL: "Malzeme",
    STAGEBOX: "Aşama Kutusu",
};

export const TURKISH_GEAR_LABELS: Record<string, string> = {
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

export const DEFAULT_RARITY_META: RarityMeta = {
    rank: -1,
    color: "rgb(90, 90, 90)",
    labelKey: "rarity.UNKNOWN",
};

export const RARITY_META: Record<string, RarityMeta> = {
    COMMON: { rank: 0, color: "rgb(132, 132, 132)", labelKey: "rarity.COMMON" },
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
