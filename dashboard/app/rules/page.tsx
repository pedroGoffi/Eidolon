"use client";

import { FormEvent, useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { api, ApiError } from "@/lib/api";
import type { RoutingRule } from "@/lib/types";

const emptyForm = {
  id: "",
  subdomain: "",
  path_pattern: "/*",
  destination_service: "",
  destination_addr: "",
  priority: 0,
};

export default function RulesPage() {
  const [rules, setRules] = useState<RoutingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      setRules(await api.listRules());
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Não foi possível carregar as rotas.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  function startEdit(rule: RoutingRule) {
    setEditingId(rule.id);
    setForm({
      id: rule.id,
      subdomain: rule.subdomain,
      path_pattern: rule.path_pattern,
      destination_service: rule.destination_service,
      destination_addr: rule.destination_addr,
      priority: rule.priority,
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setForm(emptyForm);
    setFormError(null);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError(null);

    if (!form.subdomain.trim() || !form.path_pattern.trim() || !form.destination_addr.trim()) {
      setFormError("Subdomínio, path pattern e host:porta de destino são obrigatórios.");
      return;
    }
    if (!/^[^:]+:\d+$/.test(form.destination_addr.trim())) {
      setFormError("Destino deve estar no formato host:porta, ex: 127.0.0.1:9001");
      return;
    }

    setSubmitting(true);
    try {
      await api.upsertRule({
        id: editingId || undefined,
        subdomain: form.subdomain.trim(),
        path_pattern: form.path_pattern.trim(),
        destination_service: form.destination_service.trim(),
        destination_addr: form.destination_addr.trim(),
        priority: Number(form.priority) || 0,
        enabled: true,
      });
      cancelEdit();
      await load();
    } catch (e) {
      setFormError(
        e instanceof ApiError
          ? e.status === 401
            ? "Token de admin inválido ou ausente — configure em Conexão."
            : e.message
          : "Falha ao salvar a rota."
      );
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: string) {
    try {
      await api.deleteRule(id);
      if (editingId === id) cancelEdit();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Falha ao remover a rota.");
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="04 — ROTAS & PORTAS"
        title="Roteamento interno"
        description="Mapeia subdomínio + path para o host:porta do serviço interno que deve atender. Aplicado em tempo real, sem reiniciar o Core."
      />

      <div className="grid grid-cols-1 gap-6 p-8 lg:grid-cols-[1fr_360px]">
        <div className="overflow-hidden rounded-lg border border-eidolon-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-eidolon-surface text-[11px] uppercase tracking-wider text-eidolon-muted">
              <tr>
                <th className="px-4 py-3 font-medium">Subdomínio</th>
                <th className="px-4 py-3 font-medium">Path</th>
                <th className="px-4 py-3 font-medium">Serviço</th>
                <th className="px-4 py-3 font-medium">Destino (porta)</th>
                <th className="px-4 py-3 font-medium">Prioridade</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {error && (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-eidolon-danger">
                    {error}
                  </td>
                </tr>
              )}
              {!error && !loading && rules.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-eidolon-muted">
                    Nenhuma rota configurada ainda.
                  </td>
                </tr>
              )}
              {rules.map((rule) => (
                <tr key={rule.id} className="border-t border-eidolon-border">
                  <td className="px-4 py-2.5 font-mono text-sm">{rule.subdomain}</td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {rule.path_pattern}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-eidolon-muted">
                    {rule.destination_service || "—"}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-accent">
                    {rule.destination_addr}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-eidolon-muted">
                    {rule.priority}
                  </td>
                  <td className="space-x-3 px-4 py-2.5 text-right">
                    <button
                      onClick={() => startEdit(rule)}
                      className="text-xs text-eidolon-accent hover:underline"
                    >
                      Editar
                    </button>
                    <button
                      onClick={() => handleDelete(rule.id)}
                      className="text-xs text-eidolon-danger hover:underline"
                    >
                      Remover
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <form
          onSubmit={handleSubmit}
          className="h-fit space-y-4 rounded-lg border border-eidolon-border bg-eidolon-surface p-5"
        >
          <div className="font-mono text-[11px] uppercase tracking-wider text-eidolon-muted">
            {editingId ? "Editar rota" : "Nova rota"}
          </div>

          <TextField
            label="Subdomínio"
            placeholder="app"
            value={form.subdomain}
            onChange={(v) => setForm({ ...form, subdomain: v })}
          />
          <TextField
            label="Path pattern"
            placeholder="/api/*"
            value={form.path_pattern}
            onChange={(v) => setForm({ ...form, path_pattern: v })}
          />
          <TextField
            label="Nome do serviço"
            placeholder="api-service"
            value={form.destination_service}
            onChange={(v) => setForm({ ...form, destination_service: v })}
          />
          <TextField
            label="Destino (host:porta)"
            placeholder="127.0.0.1:9001"
            value={form.destination_addr}
            onChange={(v) => setForm({ ...form, destination_addr: v })}
            mono
          />
          <TextField
            label="Prioridade"
            placeholder="0"
            value={String(form.priority)}
            onChange={(v) => setForm({ ...form, priority: Number(v) || 0 })}
          />

          {formError && <p className="text-xs text-eidolon-danger">{formError}</p>}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={submitting}
              className="flex-1 rounded-md bg-eidolon-accent py-2 text-sm font-medium text-eidolon-bg transition-opacity hover:opacity-90 disabled:opacity-50"
            >
              {submitting ? "Salvando..." : editingId ? "Salvar alterações" : "Criar rota"}
            </button>
            {editingId && (
              <button
                type="button"
                onClick={cancelEdit}
                className="rounded-md border border-eidolon-border px-3 py-2 text-sm text-eidolon-muted hover:text-eidolon-text"
              >
                Cancelar
              </button>
            )}
          </div>
        </form>
      </div>
    </div>
  );
}

function TextField({
  label,
  value,
  onChange,
  placeholder,
  mono = false,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  mono?: boolean;
}) {
  return (
    <div>
      <label className="mb-1 block text-xs text-eidolon-muted">{label}</label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`w-full rounded-md border border-eidolon-border bg-eidolon-bg px-3 py-2 text-sm text-eidolon-text placeholder:text-eidolon-faint focus:border-eidolon-accent ${
          mono ? "font-mono" : ""
        }`}
      />
    </div>
  );
}
