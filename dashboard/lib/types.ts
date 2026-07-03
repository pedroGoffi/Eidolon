export type RequestLog = {
  correlation_id: string;
  timestamp: string;
  ip: string;
  user_agent: string;
  method: string;
  path: string;
  subdomain: string;
  body_size: number;
  status_code: number;
  latency_ms: number;
  service_target: string;
  security_decision: string; // allowed | blocked_waf | blocked_rate_limit | blocked_blacklist
  waf_rule_matched?: string;
};

export type BanEntry = {
  ip: string;
  expires_at?: string; // ausente/vazio = banimento permanente
};

export type RoutingRule = {
  id: string;
  subdomain: string;
  path_pattern: string;
  destination_service: string;
  destination_addr: string; // host:porta do serviço interno
  priority: number;
  enabled: boolean;
};

export type RealtimeEvent = {
  type: string;
  data: RequestLog;
};
