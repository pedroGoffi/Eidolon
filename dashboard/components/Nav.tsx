"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const NAV_ITEMS = [
  { href: "/", label: "Visão geral", eyebrow: "01" },
  { href: "/inspector", label: "Requisições", eyebrow: "02" },
  { href: "/security", label: "Segurança", eyebrow: "03" },
  { href: "/rules", label: "Rotas & portas", eyebrow: "04" },
  { href: "/settings", label: "Conexão", eyebrow: "05" },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <aside className="flex h-screen w-60 shrink-0 flex-col border-r border-eidolon-border bg-eidolon-surface">
      <div className="flex items-center gap-2 px-5 py-6">
        <span className="h-2 w-2 rounded-full bg-eidolon-accent shadow-glow" />
        <span className="font-mono text-sm tracking-widest text-eidolon-text">
          EIDOLON
        </span>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 px-3">
        {NAV_ITEMS.map((item) => {
          const active = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`group flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors ${
                active
                  ? "bg-eidolon-surface2 text-eidolon-text"
                  : "text-eidolon-muted hover:bg-eidolon-surface2 hover:text-eidolon-text"
              }`}
            >
              <span
                className={`font-mono text-[10px] ${
                  active ? "text-eidolon-accent" : "text-eidolon-faint"
                }`}
              >
                {item.eyebrow}
              </span>
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-eidolon-border px-5 py-4 font-mono text-[10px] text-eidolon-faint">
        analytics-core · painel
      </div>
    </aside>
  );
}
