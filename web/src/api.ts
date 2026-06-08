// Thin fetch wrapper for the poe-fissure JSON API. Same-origin in production;
// proxied to the Go server in dev (see vite.config.ts).

export async function api<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...opts,
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export interface ItemText {
  slot: string;
  name: string;
  baseType: string;
  text: string;
}

export const getSession = () =>
  api<{ authenticated: boolean }>("/api/session");
export const login = (password: string) =>
  api("/api/login", { method: "POST", body: JSON.stringify({ password }) });
export const logout = () => api("/api/logout", { method: "POST" });
export const getItems = () => api<ItemText[]>("/api/character/items");
export const refreshCharacter = () =>
  api("/api/character/refresh", { method: "POST" });

export interface Price {
  amount: number;
  currency: string;
}

export interface Listing {
  price: Price;
  seller: string;
  name: string;
  baseType: string;
  mods: string[];
}

export interface Upgrade {
  slot: string;
  equipped: string;
  score: number;
  rationale: string[] | null;
  tradeUrl: string;
  cheapest: Price | null;
  listings: Listing[] | null;
  total: number;
  unmappedStats: number;
}

export const getUpgrades = (all: boolean) =>
  api<Upgrade[]>(`/api/upgrades?all=${all}`);
export const getPrice = (slot: string) =>
  api<Upgrade>(`/api/price?slot=${encodeURIComponent(slot)}`);

export interface CraftMod {
  text: string;
  source: string;
  kind: string;
  tier?: number;
  name?: string;
  tags?: string[];
}

export interface CraftItem {
  itemClass: string;
  rarity: string;
  name: string;
  baseType: string;
  itemLevel: number;
  quality: number;
  corrupted: boolean;
  sockets: string;
  annotated: boolean;
  prefixes: number;
  suffixes: number;
  openPrefix: number;
  openSuffix: number;
  mods: CraftMod[];
}

export interface CraftStep {
  action: string;
  currencies?: string[];
  note?: string;
}

export interface CraftPlan {
  summary: string;
  craftable: boolean;
  reason?: string;
  goal?: string;
  targetKind?: string;
  steps?: CraftStep[];
  risks?: string[];
  suggestions?: string[];
  sources?: string[];
}

export interface Rating {
  grade: string;
  score: number;
  verdict: string;
  base: number;
  mods: number;
  difficulty: number;
  difficultyTier: string;
  archetype: { family: string; weapon: string };
  reasons: string[];
  sources: string[];
}

export const craftParse = (text: string) =>
  api<{ item: CraftItem; suggestions: string[]; rating: Rating }>(
    "/api/craft/parse",
    {
      method: "POST",
      body: JSON.stringify({ text }),
    },
  );

export const craftAdvise = (text: string, targetText: string, kind: string) =>
  api<{ item: CraftItem; plan: CraftPlan }>("/api/craft/advise", {
    method: "POST",
    body: JSON.stringify({ text, targetText, kind }),
  });
