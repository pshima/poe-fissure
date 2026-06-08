import { useEffect, useState } from "react";
import { getSession, logout } from "./api";
import Login from "./pages/Login";
import Gear from "./pages/Gear";
import Upgrades from "./pages/Upgrades";
import Craft from "./pages/Craft";

type View = "gear" | "upgrades" | "craft";

export default function App() {
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [view, setView] = useState<View>("gear");

  useEffect(() => {
    getSession()
      .then((s) => setAuthed(s.authenticated))
      .catch(() => setAuthed(false));
  }, []);

  if (authed === null) return <div className="loading">Loading…</div>;
  if (!authed) return <Login onLogin={() => setAuthed(true)} />;

  return (
    <div className="app">
      <header>
        <h1>poe-fissure</h1>
        <nav>
          <button className={view === "gear" ? "active" : ""} onClick={() => setView("gear")}>
            Gear
          </button>
          <button className={view === "upgrades" ? "active" : ""} onClick={() => setView("upgrades")}>
            Upgrades
          </button>
          <button className={view === "craft" ? "active" : ""} onClick={() => setView("craft")}>
            Craft
          </button>
          <button className="logout" onClick={() => logout().then(() => setAuthed(false))}>
            Log out
          </button>
        </nav>
      </header>
      <main>
        {view === "gear" && <Gear />}
        {view === "upgrades" && <Upgrades />}
        {view === "craft" && <Craft />}
      </main>
    </div>
  );
}
