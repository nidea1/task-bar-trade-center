package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/gameassets"
	"github.com/nidea1/task-bar-trade-center/internal/storage"
)

type Status struct {
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	ItemCount int    `json:"item_count"`
	Added     int    `json:"added"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type installedCache struct {
	Version     int          `json:"version"`
	Fingerprint string       `json:"fingerprint"`
	UpdatedAt   string       `json:"updated_at"`
	Items       []ItemConfig `json:"items"`
}

func LoadCached(path string, base []ItemConfig) ([]ItemConfig, Status, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, Status{}, err
	}
	if len(b) > 32<<20 {
		return nil, Status{}, fmt.Errorf("catalog cache exceeds size limit")
	}
	var cache installedCache
	if err := json.Unmarshal(b, &cache); err != nil {
		return nil, Status{}, err
	}
	if cache.Version != 1 || cache.Fingerprint == "" {
		return nil, Status{}, fmt.Errorf("unsupported catalog cache")
	}
	if err := validateCatalog(cache.Items, base); err != nil {
		return nil, Status{}, err
	}
	return cache.Items, Status{State: "cached", ItemCount: len(cache.Items), Added: len(cache.Items) - len(base), UpdatedAt: cache.UpdatedAt}, nil
}

// RefreshInstalled reads only the installation directory. The application owns
// cachePath; neither game files nor the embedded catalog are modified.
func RefreshInstalled(dataDir, cachePath string, base []ItemConfig) ([]ItemConfig, Status, error) {
	status := Status{State: "embedded", ItemCount: len(base)}
	dir := filepath.Join(dataDir, "StreamingAssets", "aa", "StandaloneWindows64")
	locales, err := filepath.Glob(filepath.Join(dir, "localization-string-tables-*.bundle"))
	if err != nil {
		return base, status, err
	}
	if len(locales) == 0 {
		return base, status, fmt.Errorf("game localization bundles were not found")
	}
	manifestPath := filepath.Join(dataDir, "StreamingAssets", "aa", "catalog.bin")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return base, status, fmt.Errorf("localization catalog: %w", err)
	}
	if len(manifest) > 16<<20 {
		return base, status, fmt.Errorf("localization catalog exceeds size limit")
	}
	activeLocales := locales[:0]
	for _, path := range locales {
		if bytes.Contains(manifest, []byte(filepath.Base(path))) {
			activeLocales = append(activeLocales, path)
		}
	}
	locales = activeLocales
	if len(locales) == 0 {
		return base, status, fmt.Errorf("no localization bundles referenced by the active game catalog")
	}
	sharedFiles, err := filepath.Glob(filepath.Join(dir, "localization-assets-shared_assets_all*.bundle"))
	if err != nil {
		return base, status, err
	}
	shared := ""
	for _, path := range sharedFiles {
		if bytes.Contains(manifest, []byte(filepath.Base(path))) {
			if shared != "" {
				return base, status, fmt.Errorf("multiple active ItemTable shared bundles")
			}
			shared = path
		}
	}
	if shared == "" {
		return base, status, fmt.Errorf("active ItemTable shared bundle not found")
	}
	assets := filepath.Join(dataDir, "sharedassets0.assets")
	files := append([]string{assets, shared, manifestPath}, locales...)
	h := sha256.New()
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return base, status, err
		}
		h.Write([]byte(filepath.Base(path)))
		n, copyErr := io.Copy(h, io.LimitReader(f, 128<<20+1))
		f.Close()
		if copyErr != nil {
			return base, status, copyErr
		}
		if n > 128<<20 {
			return base, status, fmt.Errorf("game asset exceeds size limit")
		}
	}
	fingerprint := hex.EncodeToString(h.Sum(nil))
	if raw, err := os.ReadFile(cachePath); err == nil && len(raw) <= 32<<20 {
		var cached installedCache
		if json.Unmarshal(raw, &cached) == nil && cached.Version == 1 && cached.Fingerprint == fingerprint && validateCatalog(cached.Items, base) == nil {
			return cached.Items, Status{State: "cached", ItemCount: len(cached.Items), Added: len(cached.Items) - len(base), UpdatedAt: cached.UpdatedAt}, nil
		}
	}
	texts, err := gameassets.TextAssets(assets)
	if err != nil {
		return base, status, err
	}

	names, err := gameassets.ItemNames(shared, locales)
	if err != nil {
		return base, status, err
	}
	items, err := mergeGameCSV(texts["ItemInfoData"], names, base)
	if err != nil {
		return base, status, err
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if cachePath != "" {
		if err := storage.WriteJSONAtomic(cachePath, installedCache{1, fingerprint, stamp, items}); err != nil {
			return base, status, fmt.Errorf("catalog cache: %w", err)
		}
	}
	return items, Status{State: "updated", ItemCount: len(items), Added: len(items) - len(base), UpdatedAt: stamp}, nil
}

func mergeGameCSV(raw []byte, names map[string]map[string]string, base []ItemConfig) ([]ItemConfig, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = 0
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("ItemInfoData header: %w", err)
	}
	columns := map[string]int{}
	for i, k := range header {
		columns[k] = i
	}
	for _, key := range []string{"ItemKey", "NameKey", "ITEMTYPE", "GRADE", "GEARTYPE", "IsCanExchangeMarketable", "IsDeletedInServer"} {
		if _, ok := columns[key]; !ok {
			return nil, fmt.Errorf("missing ItemInfoData column %s", key)
		}
	}
	items := append([]ItemConfig(nil), base...)
	indices := map[int]int{}
	for i, item := range items {
		indices[item.ID] = i
	}
	seen := map[int]bool{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		get := func(key string) string {
			index, ok := columns[key]
			if !ok {
				return ""
			}
			return strings.TrimSpace(row[index])
		}
		id, err := strconv.Atoi(get("ItemKey"))
		if err != nil || id <= 0 || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate game item ID %q", get("ItemKey"))
		}
		seen[id] = true
		index, exists := indices[id]
		var previous ItemConfig
		if exists {
			previous = items[index]
		}
		item := ItemConfig{ID: id, Grade: get("GRADE"), Type: get("ITEMTYPE"), MarketHashName: previous.MarketHashName, Marketable: previous.Marketable, Name: previous.Name}
		if gear := get("GEARTYPE"); gear != "" && gear != "NONE" {
			item.Gear = &gear
		}
		key := get("NameKey")
		if translated := names[key]; translated != nil {
			item.Name = make(map[string]string, len(translated))
			for locale, name := range previous.Name {
				item.Name[locale] = name
			}
			for locale, name := range translated {
				item.Name[locale] = name
			}
		} else if !strings.HasPrefix(key, "ItemName_") && key != "" {
			item.Name = map[string]string{"en-US": key}
		}
		if !exists && item.Name["en-US"] == "" {
			return nil, fmt.Errorf("English name missing for new item %d (%s)", id, key)
		}
		for _, key := range []string{"IsCanExchangeMarketable", "IsDeletedInServer"} {
			v := get(key)
			if v != "" && v != "True" && v != "False" {
				return nil, fmt.Errorf("invalid %s for item %d", key, id)
			}
		}
		item.Marketable = (previous.Marketable && (previous.Name["en-US"] != "" || previous.MarketHashName != "")) || get("IsCanExchangeMarketable") == "True"
		if get("IsDeletedInServer") == "True" {
			item.Marketable = false
		}
		if item.Grade == "" {
			return nil, fmt.Errorf("missing grade for item %d", id)
		}
		// Display names can change independently of existing Steam listings.
		// Preserve the previously verified listing name when refreshing labels.
		if previous.Marketable && previous.Name["en-US"] != "" && previous.Name["en-US"] != item.Name["en-US"] && item.MarketHashName == "" {
			item.MarketHashName = previous.Name["en-US"]
			if previous.Type != "MATERIAL" {
				item.MarketHashName += " (" + strings.ToUpper(previous.Grade[:1]) + strings.ToLower(previous.Grade[1:]) + ") A"
			}
		}
		if item.Name["en-US"] == "" {
			item.Marketable = false
		}
		if item.MarketHashName == "" && item.Type == "MATERIAL" {
			switch item.Name["en-US"] {
			case "Plague Fruit", "Contaminated Reliquary", "Contaminated Relic":
				item.MarketHashName = item.Name["en-US"] + " (" + strings.ToUpper(item.Grade[:1]) + strings.ToLower(item.Grade[1:]) + ")"
			}
		}
		if exists {
			items[index] = item
		} else {
			indices[id] = len(items)
			items = append(items, item)
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("game item table is empty")
	}
	if len(base) > 100 && len(seen) < len(base)*9/10 {
		return nil, fmt.Errorf("game item table is unexpectedly incomplete: %d rows", len(seen))
	}
	families := map[string]map[string]bool{}
	for _, item := range items {
		if item.Marketable && item.Type == "MATERIAL" && item.Name["en-US"] != "" {
			name := item.Name["en-US"]
			if families[name] == nil {
				families[name] = map[string]bool{}
			}
			families[name][item.Grade] = true
		}
	}
	for _, item := range items {
		if item.Marketable && item.Type == "MATERIAL" && item.MarketHashName == "" && len(families[item.Name["en-US"]]) > 1 {
			return nil, fmt.Errorf("Steam market naming needs verification for material %s", item.Name["en-US"])
		}
	}
	if err := validateCatalog(items, base); err != nil {
		return nil, err
	}
	return items, nil
}

func validateCatalog(items, base []ItemConfig) error {
	if len(items) == 0 || len(items) > 100000 {
		return fmt.Errorf("invalid catalog size")
	}
	seen := map[int]bool{}
	legacyUnnamed := map[int]bool{}
	for _, item := range base {
		if item.Name["en-US"] == "" {
			legacyUnnamed[item.ID] = true
		}
	}
	for _, item := range items {
		if item.ID <= 0 || seen[item.ID] {
			return fmt.Errorf("invalid catalog item %d", item.ID)
		}
		seen[item.ID] = true
		if item.Name["en-US"] == "" && !legacyUnnamed[item.ID] {
			return fmt.Errorf("English catalog name missing for %d", item.ID)
		}
		switch item.Type {
		case "GEAR", "MATERIAL", "STAGEBOX":
		default:
			return fmt.Errorf("unsupported item type %q", item.Type)
		}
		switch item.Grade {
		case "COMMON", "UNCOMMON", "RARE", "LEGENDARY", "IMMORTAL", "ARCANA", "BEYOND", "CELESTIAL", "DIVINE", "COSMIC":
		default:
			return fmt.Errorf("unsupported item grade %q", item.Grade)
		}
	}
	for _, item := range base {
		if !seen[item.ID] {
			return fmt.Errorf("catalog cache lost item %d", item.ID)
		}
	}
	return nil
}
