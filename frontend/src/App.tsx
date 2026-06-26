import {useEffect, useMemo, useState} from 'react';
import './App.css';
import {
    GetInventoryDashboard,
    OpenMarketListing,
    RefreshInventoryPrices,
} from "../wailsjs/go/main/App";

type DashboardItem = {
    item_id: number;
    name: string;
    market_hash_name: string;
    market_url: string;
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
        priced_item_count: number;
        unknown_item_count: number;
        marketable_item_count: number;
        total_item_count: number;
    };
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
}

function App() {
    const [state, setState] = useState<DashboardState | null>(null);
    const [error, setError] = useState<string>("");
    const [refreshing, setRefreshing] = useState(false);

    const load = () => {
        GetInventoryDashboard()
            .then((nextState) => {
                setState(nextState as DashboardState);
                setError("");
            })
            .catch((err) => setError(String(err)));
    };

    useEffect(() => {
        load();
        const timer = window.setInterval(load, 5000);
        return () => window.clearInterval(timer);
    }, []);

    const totals = state?.totals;
    const staleText = useMemo(() => {
        if (!state?.updated_at) return "Waiting for inventory state";
        return `Updated ${formatRelativeTime(state.updated_at)}`;
    }, [state?.updated_at]);

    const refreshPrices = () => {
        setRefreshing(true);
        RefreshInventoryPrices()
            .then(load)
            .catch((err) => setError(String(err)))
            .finally(() => setRefreshing(false));
    };

    return (
        <main className="shell">
            <header className="topbar">
                <div>
                    <h1>Inventory Dashboard</h1>
                    <p>{state?.market_scope || "Task Bar Trade Center"} · {staleText}</p>
                </div>
                <button className="primary" disabled={refreshing} onClick={refreshPrices}>
                    {refreshing ? "Queued" : "Refresh prices"}
                </button>
            </header>

            {error && <div className="notice">{error}</div>}

            <section className="summary">
                <Metric label="Suggested value" value={formatPrice(totals?.suggested_listing_value, state)} />
                <Metric label="Instant sell" value={formatPrice(totals?.instant_sell_value, state)} />
                <Metric label="Stash" value={formatPrice(totals?.stash_value, state)} />
                <Metric label="Equipped" value={formatPrice(totals?.equipped_value, state)} />
                <Metric label="Gold" value={formatNumber(state?.gold)} />
                <Metric label="Priced" value={`${totals?.priced_item_count ?? 0}/${totals?.marketable_item_count ?? 0}`} />
            </section>

            {state?.refresh?.refreshing && (
                <div className="statusline">
                    Refreshing inventory prices: {state.refresh.completed} done, {state.refresh.queued} queued
                </div>
            )}
            {state?.refresh?.last_error && (
                <div className="statusline warning">{state.refresh.last_error}</div>
            )}

            <section className="grid">
                <ItemTable title="Most Valuable" items={state?.most_valuable || []} state={state} />
                <ItemTable title="Duplicates" items={state?.duplicates || []} state={state} />
                <ItemTable title="Equipped Items" items={state?.equipped || []} state={state} />
                <ItemTable title="Missing Prices" items={state?.missing_prices || []} state={state} />
            </section>
        </main>
    );
}

function Metric({label, value}: { label: string; value: string }) {
    return (
        <div className="metric">
            <span>{label}</span>
            <strong>{value}</strong>
        </div>
    );
}

function ItemTable({title, items, state}: { title: string; items: DashboardItem[]; state: DashboardState | null }) {
    return (
        <section className="panel">
            <div className="panel-title">
                <h2>{title}</h2>
                <span>{items.length}</span>
            </div>
            <div className="table">
                {items.length === 0 && <div className="empty">No items yet</div>}
                {items.map((item) => (
                    <button className="row" key={`${title}-${item.item_id}`} onClick={() => OpenMarketListing(item.item_id)}>
                        <span className="item-name">{item.name || item.market_hash_name || `Item ${item.item_id}`}</span>
                        <span className="muted">x{item.count}</span>
                        <span className="muted">{item.location}</span>
                        <span className="price">
                            {item.has_price ? formatPrice(item.total_suggested, state, item) : "Missing"}
                        </span>
                    </button>
                ))}
            </div>
        </section>
    );
}

function formatPrice(value?: number, state?: DashboardState | null, item?: DashboardItem) {
    if (value === undefined || Number.isNaN(value)) return "N/A";
    const prefix = item?.price_prefix || state?.price_prefix || "$";
    const suffix = item?.price_suffix || state?.price_suffix || "";
    return `${prefix}${value.toFixed(2)}${suffix}`;
}

function formatNumber(value?: number) {
    if (value === undefined || Number.isNaN(value)) return "0";
    return new Intl.NumberFormat().format(value);
}

function formatRelativeTime(value: string) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "recently";
    const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
    if (seconds < 60) return "just now";
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    return `${Math.floor(hours / 24)}d ago`;
}

export default App;
