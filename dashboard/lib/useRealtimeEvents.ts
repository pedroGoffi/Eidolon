"use client";

import { useEffect, useRef, useState } from "react";
import { getApiConfig } from "./api";
import type { RealtimeEvent, RequestLog } from "./types";

/**
 * Assina o stream SSE do Core (/__eidolon/realtime) e mantém uma janela
 * deslizante dos últimos eventos em memória — é a partir daqui que a
 * Overview calcula requests/s, taxa de erro e latência média ao vivo.
 */
export function useRealtimeEvents(windowSize = 200) {
  const [events, setEvents] = useState<RequestLog[]>([]);
  const [connected, setConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const { baseUrl } = getApiConfig();
    const es = new EventSource(`${baseUrl}/__eidolon/realtime`);
    esRef.current = es;

    es.onopen = () => setConnected(true);
    es.onerror = () => setConnected(false);
    es.onmessage = (msg) => {
      try {
        const ev: RealtimeEvent = JSON.parse(msg.data);
        if (ev.type === "request") {
          setEvents((prev) => {
            const next = [ev.data, ...prev];
            return next.slice(0, windowSize);
          });
        }
      } catch {
        /* evento malformado, ignora */
      }
    };

    return () => {
      es.close();
      esRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { events, connected };
}
