"use client";

import { FormEvent, useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { api, ApiError } from "@/lib/api";
import type { BanEntry } from "@/lib/types";

const DURATION_PRESETS = [
  { label: "1 hora", seconds: 3600 },
  { label: "24 horas", seconds: 86400 },
  { label: "7 dias", seconds: 604800 },
  { label: "Permanente", seconds: 0 },
];

export default function SecurityPage() {
  const [bans, setBans] = useState<BanEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [ip, setIp] = useState("");
  const [duration, setDuration] = useState(3600);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      setBans(await api.listBans());
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Não foi possível carregar a blacklist.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleBan(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    if (!ip.trim()) {
      setFormError("Informe um IP válido.");
      return;
    }
    setSubmitting(true);
    try {
      await api.ban(ip.trim(), duration);
      setIp("");
      await load();
    } catch (e) {
      setFormError(
        e instanceof ApiError
          ? e.status === 401
            ? "Token de admin inválido ou ausente — configure em Conexão."
            : e.message
          : "Falha ao banir o IP."
      );
    } finally {
      setSubmitting(false);
    }
  }

  async function handleUnban(targetIp: string) {
    try {
      await api.unban(targetIp);
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Falha ao desbloquear o IP.");
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="03 — SEGURANÇA"
        title="Blacklist de IPs"
        description="Bloqueios ativos no Security Layer, aplicados imediatamente pelo Core."
      />

      <div className="grid grid-cols-1 gap-6 p-8 lg:grid-cols-[1fr_320px]">
        <div className="overflow-hidden rounded-lg border border-eidolon-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-eidolon-surface text-[11px] uppercase tracking-wider text-eidolon-muted">
              <tr>
                <th className="px-4 py-3 font-medium">IP</th>
                <th className="px-4 py-3 font-medium">Expira em</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {error && (
                <tr>
                  <td colSpan={3} className="px-4 py-6 text-center text-eidolon-danger">
                    {error}
                  </td>
                </tr>
              )}
              {!error && !loading && bans.length === 0 && (
                <tr>
                  <td colSpan={3} className="px-4 py-10 text-center text-eidolon-muted">
                    Nenhum IP bloqueado no momento.
                  </td>
                </tr>
              )}
              {bans.map((ban) => (
                <tr key={ban.ip} className="border-t border-eidolon-border">
                  <td className="px-4 py-2.5 font-mono text-sm">{ban.ip}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {ban.expires_at
                      ? new Date(ban.expires_at).toLocaleString("pt-BR")
                      : "permanente"}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => handleUnban(ban.ip)}
                      className="text-xs text-eidolon-accent hover:underline"
                    >
                      Desbloquear
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <form
          onSubmit={handleBan}
          className="h-fit space-y-4 rounded-lg border border-eidolon-border bg-eidolon-surface p-5"
        >
          <div className="font-mono text-[11px] uppercase tracking-wider text-eidolon-muted">
            Bloquear IP
          </div>

          <div>
            <label className="mb-1 block text-xs text-eidolon-muted">Endereço IP</label>
            <input
              value={ip}
              onChange={(e) => setIp(e.target.value)}
              placeholder="203.0.113.42"
              className="w-full rounded-md border border-eidolon-border bg-eidolon-bg px-3 py-2 font-mono text-sm text-eidolon-text placeholder:text-eidolon-faint focus:border-eidolon-accent"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs text-eidolon-muted">Duração</label>
            <select
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value))}
              className="w-full rounded-md border border-eidolon-border bg-eidolon-bg px-3 py-2 text-sm text-eidolon-text focus:border-eidolon-accent"
            >
              {DURATION_PRESETS.map((preset) => (
                <option key={preset.label} value={preset.seconds}>
                  {preset.label}
                </option>
              ))}
            </select>
          </div>

          {formError && <p className="text-xs text-eidolon-danger">{formError}</p>}

          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-md bg-eidolon-danger/90 py-2 text-sm font-medium text-white transition-colors hover:bg-eidolon-danger disabled:opacity-50"
          >
            {submitting ? "Bloqueando..." : "Bloquear IP"}
          </button>
        </form>
      </div>
    </div>
  );
}
