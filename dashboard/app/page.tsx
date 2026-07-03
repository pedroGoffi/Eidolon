"use client";

import { useMemo, useState, useEffect } from "react";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { Pulse } from "@/components/Pulse";
import { useRealtimeEvents } from "@/lib/useRealtimeEvents";

const BUCKET_MS = 1000;
const BUCKET_COUNT = 60;

export default function OverviewPage() {
  const { events, connected } = useRealtimeEvents(500);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  const buckets = useMemo(() => {
    const arr = new Array(BUCKET_COUNT).fill(0);
    for (const ev of events) {
      const ts = new Date(ev.timestamp).getTime();
      const diff = now - ts;
      const bucketIndex = BUCKET_COUNT - 1 - Math.floor(diff / BUCKET_MS);
      if (bucketIndex >= 0 && bucketIndex < BUCKET_COUNT) {
        arr[bucketIndex]++;
      }
    }
    return arr;
  }, [events, now]);

  const last10s = events.filter(
    (ev) => now - new Date(ev.timestamp).getTime() < 10_000
  );
  const requestsPerSecond = (last10s.length / 10).toFixed(1);

  const errorCount = last10s.filter((ev) => ev.status_code >= 400).length;
  const errorRate = last10s.length
    ? ((errorCount / last10s.length) * 100).toFixed(1)
    : "0.0";

  const avgLatency = last10s.length
    ? Math.round(
        last10s.reduce((sum, ev) => sum + ev.latency_ms, 0) / last10s.length
      )
    : 0;

  const topSubdomains = useMemo(() => {
    const counts = new Map<string, number>();
    for (const ev of events) {
      counts.set(ev.subdomain, (counts.get(ev.subdomain) || 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5);
  }, [events]);

  const topIps = useMemo(() => {
    const counts = new Map<string, number>();
    for (const ev of events) {
      counts.set(ev.ip, (counts.get(ev.ip) || 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5);
  }, [events]);

  return (
    <div>
      <PageHeader
        eyebrow="01 — VISÃO GERAL"
        title="Pulso do tráfego"
        description="Métricas ao vivo, direto do stream de eventos do Core."
        action={
          <div className="flex items-center gap-2 font-mono text-xs text-eidolon-muted">
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                connected ? "bg-eidolon-ok" : "bg-eidolon-danger"
              }`}
            />
            {connected ? "conectado" : "desconectado"}
          </div>
        }
      />

      <div className="space-y-6 p-8">
        <div className="rounded-lg border border-eidolon-border bg-eidolon-surface p-6">
          <Pulse values={buckets} />
        </div>

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <StatCard label="Requests / s" value={requestsPerSecond} />
          <StatCard
            label="Taxa de erro (10s)"
            value={`${errorRate}%`}
            tone={Number(errorRate) > 5 ? "danger" : "default"}
          />
          <StatCard label="Latência média" value={`${avgLatency} ms`} />
          <StatCard label="Eventos na janela" value={String(events.length)} />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Panel title="Top subdomínios">
            <RankedList
              items={topSubdomains}
              empty="Nenhum evento ainda — gere tráfego pelo Nginx/Core."
            />
          </Panel>
          <Panel title="Top IPs">
            <RankedList
              items={topIps}
              empty="Nenhum evento ainda — gere tráfego pelo Nginx/Core."
            />
          </Panel>
        </div>
      </div>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-eidolon-border bg-eidolon-surface p-4">
      <div className="mb-3 font-mono text-[11px] uppercase tracking-wider text-eidolon-muted">
        {title}
      </div>
      {children}
    </div>
  );
}

function RankedList({ items, empty }: { items: [string, number][]; empty: string }) {
  if (items.length === 0) {
    return <div className="py-6 text-center text-sm text-eidolon-muted">{empty}</div>;
  }
  const max = items[0][1];
  return (
    <ul className="space-y-2">
      {items.map(([key, count]) => (
        <li key={key} className="flex items-center gap-3">
          <span className="w-32 truncate font-mono text-xs text-eidolon-text">
            {key || "(raiz)"}
          </span>
          <div className="h-1.5 flex-1 rounded-full bg-eidolon-surface2">
            <div
              className="h-1.5 rounded-full bg-eidolon-accent"
              style={{ width: `${(count / max) * 100}%` }}
            />
          </div>
          <span className="w-8 text-right font-mono text-xs text-eidolon-muted">
            {count}
          </span>
        </li>
      ))}
    </ul>
  );
}
