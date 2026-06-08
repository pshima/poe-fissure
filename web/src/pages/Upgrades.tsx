import { useState } from "react";
import { getUpgrades, type Upgrade } from "../api";

function priceLabel(u: Upgrade): string {
  if (!u.cheapest) return "no listings";
  return `${u.cheapest.amount} ${u.cheapest.currency}`;
}

export default function Upgrades() {
  const [rows, setRows] = useState<Upgrade[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function run(all: boolean) {
    setBusy(true);
    setError("");
    try {
      setRows(await getUpgrades(all));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section>
      <div className="row">
        <h2>Upgrades</h2>
        <div>
          <button onClick={() => run(false)} disabled={busy}>
            Top slot
          </button>{" "}
          <button onClick={() => run(true)} disabled={busy}>
            Price all slots
          </button>
        </div>
      </div>
      <p className="muted">
        On-demand only — each priced slot makes ~2 trade requests. Sorted cheapest first.
      </p>
      {busy && <p className="muted">Checking trade…</p>}
      {error && <p className="error">{error}</p>}

      <div className="upgrades">
        {rows?.map((u) => (
          <div className="upgrade" key={u.slot}>
            <div className="upgrade-head">
              <span className="slot">{u.slot}</span>
              <span className="equipped">{u.equipped}</span>
              <span className="price">{priceLabel(u)}</span>
            </div>
            <div className="upgrade-meta">
              {u.rationale && u.rationale.length > 0 && (
                <span>{u.rationale.join("; ")}</span>
              )}
              {u.total > 0 && <span className="muted"> · {u.total} listings</span>}
              {u.total === 0 && (
                <span className="muted"> · no matching listings</span>
              )}
              {u.unmappedStats > 0 && (
                <span className="muted"> · {u.unmappedStats} stat(s) unmapped</span>
              )}{" "}
              <a href={u.tradeUrl} target="_blank" rel="noreferrer">
                open on trade ↗
              </a>
            </div>
            {u.listings && u.listings.length > 0 && (
              <ul className="listings">
                {u.listings.slice(0, 5).map((l, i) => (
                  <li key={i}>
                    <span className="price">
                      {l.price.amount} {l.price.currency}
                    </span>{" "}
                    <span className="name">{l.name || l.baseType}</span>{" "}
                    <span className="muted">{l.baseType}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}
