import { useState } from "react";
import {
  craftParse,
  craftAdvise,
  type CraftItem,
  type CraftPlan,
  type Rating,
} from "../api";

export default function Craft() {
  const [text, setText] = useState("");
  const [item, setItem] = useState<CraftItem | null>(null);
  const [rating, setRating] = useState<Rating | null>(null);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [target, setTarget] = useState("");
  const [kind, setKind] = useState("");
  const [plan, setPlan] = useState<CraftPlan | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function parse() {
    setBusy(true);
    setError("");
    setPlan(null);
    try {
      const res = await craftParse(text);
      setItem(res.item);
      setRating(res.rating);
      setSuggestions(res.suggestions || []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function advise() {
    setBusy(true);
    setError("");
    try {
      const res = await craftAdvise(text, target, kind);
      setItem(res.item);
      setPlan(res.plan);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section>
      <h2>Craft Advisor</h2>
      <p className="muted">
        Paste an item from the game (Ctrl+C in-game). Enable “Advanced Mod Descriptions” for
        best accuracy (adds prefix/suffix + tier info).
      </p>
      <textarea
        rows={10}
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Item Class: ...&#10;Rarity: Rare&#10;..."
        style={{ width: "100%" }}
      />
      <div style={{ marginTop: "0.5rem" }}>
        <button onClick={parse} disabled={busy || !text.trim()}>
          Parse item
        </button>
      </div>
      {error && <p className="error">{error}</p>}

      {rating && <GradeCard r={rating} />}

      {item && (
        <div className="craft-item">
          <div className="craft-summary">
            <strong>
              {item.rarity} {item.itemClass}
            </strong>{" "}
            — {item.name || item.baseType} ({item.baseType}), ilvl {item.itemLevel}
            {item.corrupted && <span className="warn"> · Corrupted</span>}
            {!item.annotated && (
              <span className="muted"> · prefix/suffix inferred (no tier data)</span>
            )}
          </div>
          {!item.corrupted && item.rarity !== "Unique" && (
            <div className="muted">
              {item.prefixes}/3 prefixes, {item.suffixes}/3 suffixes — {item.openPrefix} open
              prefix, {item.openSuffix} open suffix
            </div>
          )}

          <div className="craft-controls">
            <input
              type="text"
              placeholder="Target mod (e.g. increased Physical Damage)"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              style={{ flex: 1 }}
            />
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="">auto</option>
              <option value="prefix">prefix</option>
              <option value="suffix">suffix</option>
            </select>
            <button onClick={advise} disabled={busy}>
              Get steps
            </button>
          </div>

          {suggestions.length > 0 && (
            <div className="chips">
              {suggestions.map((s) => (
                <button key={s} className="chip" onClick={() => setTarget(s)}>
                  {s}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {plan && <PlanView plan={plan} />}
    </section>
  );
}

function gradeColor(grade: string): string {
  const g = grade[0];
  if (g === "A") return "#7ec96b";
  if (g === "B") return "#b9c96b";
  if (g === "C") return "#c9a25a";
  if (g === "D") return "#d08a5a";
  if (g === "F") return "#d06a6a";
  return "#9a8f80"; // "—"
}

function Bar({ label, value }: { label: string; value: number }) {
  return (
    <div className="bar-row">
      <span className="bar-label">{label}</span>
      <div className="bar-track">
        <div className="bar-fill" style={{ width: `${value}%` }} />
      </div>
      <span className="bar-val">{value}</span>
    </div>
  );
}

function GradeCard({ r }: { r: Rating }) {
  return (
    <div className="grade-card">
      <div className="grade-letter" style={{ color: gradeColor(r.grade) }}>
        {r.grade}
      </div>
      <div className="grade-body">
        <div className="grade-verdict">{r.verdict}</div>
        <div className="muted grade-arch">
          for your {r.archetype.family}
          {r.archetype.weapon !== "unknown" ? `/${r.archetype.weapon}` : ""} build
          {r.grade !== "—" && ` · difficulty: ${r.difficultyTier}`}
        </div>
        {r.grade !== "—" && (
          <div className="grade-bars">
            <Bar label="Base" value={r.base} />
            <Bar label="Mods" value={r.mods} />
            <Bar label="Ease to finish" value={r.difficulty} />
          </div>
        )}
        {r.reasons.length > 0 && (
          <ul className="grade-reasons">
            {r.reasons.map((x, i) => (
              <li key={i}>{x}</li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function PlanView({ plan }: { plan: CraftPlan }) {
  return (
    <div className="plan">
      <p>{plan.summary}</p>
      {!plan.craftable && plan.reason && <p className="warn">{plan.reason}</p>}
      {plan.goal && (
        <p className="muted">
          Target: <strong>{plan.goal}</strong>
          {plan.targetKind ? ` (${plan.targetKind})` : ""}
        </p>
      )}
      {plan.reason && plan.craftable && <p className="muted">{plan.reason}</p>}

      {plan.steps && plan.steps.length > 0 && (
        <ol className="steps">
          {plan.steps.map((s, i) => (
            <li key={i}>
              <div className="step-action">{s.action}</div>
              {s.currencies && s.currencies.length > 0 && (
                <div className="step-curr">{s.currencies.join(" + ")}</div>
              )}
              {s.note && <div className="muted step-note">{s.note}</div>}
            </li>
          ))}
        </ol>
      )}

      {plan.risks && plan.risks.length > 0 && (
        <div className="risks">
          {plan.risks.map((r, i) => (
            <p key={i} className="warn">
              ⚠ {r}
            </p>
          ))}
        </div>
      )}

      {plan.sources && plan.sources.length > 0 && (
        <p className="muted sources">
          Sources:{" "}
          {plan.sources.map((s, i) => (
            <a key={i} href={s} target="_blank" rel="noreferrer">
              [{i + 1}]
            </a>
          ))}
        </p>
      )}
    </div>
  );
}
