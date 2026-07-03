"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { api, ApiError } from "@/lib/api";
import type { RequestLog } from "@/lib/types";

export default function InspectorPage() {
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [selected, setSelected] = useState<RequestLog | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listLogs(200);
      setLogs(data);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Não foi possível carregar os logs.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div>
      <PageHeader
        eyebrow="02 — REQUISIÇÕES"
        title="Request inspector"
        description="Últimas requisições processadas pelo Core, com detalhe completo por correlation ID."
        action={
          <button
            onClick={load}
            className="rounded-md border border-eidolon-border px-3 py-1.5 text-xs text-eidolon-muted hover:text-eidolon-text"
          >
            Atualizar
          </button>
        }
      />

      <div className="grid grid-cols-1 gap-6 p-8 lg:grid-cols-[1fr_360px]">
        <div className="overflow-hidden rounded-lg border border-eidolon-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-eidolon-surface text-[11px] uppercase tracking-wider text-eidolon-muted">
              <tr>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Método</th>
                <th className="px-4 py-3 font-medium">Path</th>
                <th className="px-4 py-3 font-medium">Subdomínio</th>
                <th className="px-4 py-3 font-medium">IP</th>
                <th className="px-4 py-3 font-medium">Latência</th>
                <th className="px-4 py-3 font-medium">Decisão</th>
              </tr>
            </thead>
            <tbody>
              {error && (
                <tr>
                  <td colSpan={7} className="px-4 py-6 text-center text-eidolon-danger">
                    {error}
                  </td>
                </tr>
              )}
              {!error && !loading && logs.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-10 text-center text-eidolon-muted">
                    Nenhuma requisição registrada ainda. Assim que o Nginx enviar
                    tráfego pro Core, elas aparecem aqui.
                  </td>
                </tr>
              )}
              {logs.map((log) => (
                <tr
                  key={log.correlation_id}
                  onClick={() => setSelected(log)}
                  className={`cursor-pointer border-t border-eidolon-border hover:bg-eidolon-surface2 ${
                    selected?.correlation_id === log.correlation_id
                      ? "bg-eidolon-surface2"
                      : ""
                  }`}
                >
                  <td className="px-4 py-2.5">
                    <StatusBadge code={log.status_code} />
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {log.method}
                  </td>
                  <td className="max-w-[220px] truncate px-4 py-2.5 font-mono text-xs">
                    {log.path}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {log.subdomain || "—"}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {log.ip}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {log.latency_ms}ms
                  </td>
                  <td className="px-4 py-2.5">
                    <DecisionBadge decision={log.security_decision} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="rounded-lg border border-eidolon-border bg-eidolon-surface p-4">
          <div className="mb-3 font-mono text-[11px] uppercase tracking-wider text-eidolon-muted">
            Timeline do request
          </div>
          {!selected && (
            <p className="text-sm text-eidolon-muted">
              Selecione uma requisição na tabela para ver o detalhe completo.
            </p>
          )}
          {selected && (
            <dl className="space-y-3 font-mono text-xs">
              <Field label="correlation_id" value={selected.correlation_id} />
              <Field label="timestamp" value={selected.timestamp} />
              <Field label="método" value={selected.method} />
              <Field label="path" value={selected.path} />
              <Field label="subdomínio" value={selected.subdomain || "—"} />
              <Field label="ip" value={selected.ip} />
              <Field label="user-agent" value={selected.user_agent} wrap />
              <Field label="body size" value={`${selected.body_size} bytes`} />
              <Field label="status" value={String(selected.status_code)} />
              <Field label="latência" value={`${selected.latency_ms} ms`} />
              <Field label="serviço destino" value={selected.service_target || "—"} />
              <Field label="decisão de segurança" value={selected.security_decision} />
              {selected.waf_rule_matched && (
                <Field label="regra WAF" value={selected.waf_rule_matched} />
              )}
            </dl>
          )}
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, wrap = false }: { label: string; value: string; wrap?: boolean }) {
  return (
    <div>
      <dt className="text-eidolon-faint">{label}</dt>
      <dd className={`text-eidolon-text ${wrap ? "break-words" : "truncate"}`}>{value}</dd>
    </div>
  );
}

function StatusBadge({ code }: { code: number }) {
  const tone =
    code >= 500
      ? "text-eidolon-danger"
      : code >= 400
        ? "text-eidolon-warn"
        : "text-eidolon-ok";
  return <span className={`font-mono text-xs font-medium ${tone}`}>{code}</span>;
}

function DecisionBadge({ decision }: { decision: string }) {
  const blocked = decision?.startsWith("blocked");
  return (
    <span
      className={`rounded px-2 py-0.5 font-mono text-[10px] uppercase tracking-wide ${
        blocked
          ? "bg-eidolon-danger/10 text-eidolon-danger"
          : "bg-eidolon-ok/10 text-eidolon-ok"
      }`}
    >
      {decision || "—"}
    </span>
  );
}
