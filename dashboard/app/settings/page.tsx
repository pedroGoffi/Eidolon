"use client";

import { FormEvent, useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader";
import { api, ApiError, getApiConfig, setApiConfig } from "@/lib/api";

export default function SettingsPage() {
  const [baseUrl, setBaseUrl] = useState("");
  const [token, setToken] = useState("");
  const [status, setStatus] = useState<"idle" | "ok" | "error">("idle");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const cfg = getApiConfig();
    setBaseUrl(cfg.baseUrl);
    setToken(cfg.token);
  }, []);

  async function handleSave(e: FormEvent) {
    e.preventDefault();
    setApiConfig(baseUrl.trim(), token.trim());
    setStatus("idle");
    setMessage("Testando conexão...");
    try {
      await api.health();
      setStatus("ok");
      setMessage("Conectado ao Eidolon Analytics Core com sucesso.");
    } catch (e) {
      setStatus("error");
      setMessage(
        e instanceof ApiError
          ? `Falha ao conectar: ${e.message}`
          : "Não foi possível alcançar o Core nesse endereço."
      );
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="05 — CONEXÃO"
        title="Conexão com o Core"
        description="Endereço do Analytics Core e token usado nas ações administrativas (banir IP, editar rotas)."
      />

      <div className="max-w-lg p-8">
        <form
          onSubmit={handleSave}
          className="space-y-4 rounded-lg border border-eidolon-border bg-eidolon-surface p-5"
        >
          <div>
            <label className="mb-1 block text-xs text-eidolon-muted">
              URL do Analytics Core
            </label>
            <input
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="http://127.0.0.1:8081"
              className="w-full rounded-md border border-eidolon-border bg-eidolon-bg px-3 py-2 font-mono text-sm text-eidolon-text focus:border-eidolon-accent"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs text-eidolon-muted">
              Token de admin (ADMIN_TOKEN)
            </label>
            <input
              value={token}
              onChange={(e) => setToken(e.target.value)}
              type="password"
              placeholder="defina o mesmo valor usado no Core"
              className="w-full rounded-md border border-eidolon-border bg-eidolon-bg px-3 py-2 font-mono text-sm text-eidolon-text focus:border-eidolon-accent"
            />
            <p className="mt-1 text-xs text-eidolon-faint">
              Guardado apenas no localStorage do seu navegador — nunca enviado a
              nenhum lugar além do Core.
            </p>
          </div>

          {message && (
            <p
              className={`text-xs ${
                status === "ok"
                  ? "text-eidolon-ok"
                  : status === "error"
                    ? "text-eidolon-danger"
                    : "text-eidolon-muted"
              }`}
            >
              {message}
            </p>
          )}

          <button
            type="submit"
            className="w-full rounded-md bg-eidolon-accent py-2 text-sm font-medium text-eidolon-bg transition-opacity hover:opacity-90"
          >
            Salvar e testar conexão
          </button>
        </form>
      </div>
    </div>
  );
}
