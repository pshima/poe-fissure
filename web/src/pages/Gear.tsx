import { useEffect, useState } from "react";
import { getItems, refreshCharacter, type ItemText } from "../api";

export default function Gear() {
  const [items, setItems] = useState<ItemText[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    setError("");
    try {
      setItems(await getItems());
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function refresh() {
    setBusy(true);
    setError("");
    try {
      await refreshCharacter();
      await load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section>
      <div className="row">
        <h2>Gear</h2>
        <button onClick={refresh} disabled={busy}>
          {busy ? "Refreshing…" : "Refresh from PoE"}
        </button>
      </div>
      {error && <p className="error">{error}</p>}
      {!items && !error && <p className="muted">Loading…</p>}
      <div className="items">
        {items?.map((it) => (
          <div className="item" key={it.slot}>
            <div className="item-head">
              <span className="slot">{it.slot}</span>
              <span className="name">{it.name}</span>
            </div>
            <pre>{it.text}</pre>
          </div>
        ))}
      </div>
    </section>
  );
}
