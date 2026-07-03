# Blueprint Técnico — Analytics Core / Gateway Central
### Sistema de Analytics + Gateway + Reverse Proxy + Painel Administrativo para `*.merchonline.com.br`

---

## 1. Arquitetura Completa em Camadas

```
┌─────────────────────────────────────────────────────────────────┐
│                          INTERNET                                │
└───────────────────────────────┬───────────────────────────────────┘
                                 │ HTTPS (443) / HTTP (80 → redirect)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│  CAMADA 1 — EDGE (Nginx Gateway)                                  │
│  - Terminação TLS (wildcard *.merchonline.com.br)                 │
│  - Rate limit básico (limit_req_zone)                             │
│  - Sanitização de headers (remove X-Forwarded-* de origem externa)│
│  - Proxy único: tudo → Analytics Core                             │
└───────────────────────────────┬───────────────────────────────────┘
                                 │ rede interna (docker network isolada)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│  CAMADA 2 — ANALYTICS CORE (Go)                                   │
│  - Recepção de 100% do tráfego                                    │
│  - Geração de Correlation ID (UUID v7 — ordenável por tempo)      │
│  - Logging estruturado (JSON) → ClickHouse                        │
│  - Inspeção de request (IP, headers, body, latência)              │
│  - Chamada síncrona ao Security Layer (WAF)                       │
│  - Chamada síncrona ao Decision Engine (routing)                  │
│  - Proxy reverso interno para o serviço de destino                │
└───────┬─────────────────────────────────────────┬─────────────────┘
        │                                         │
        ▼                                         ▼
┌───────────────────┐                   ┌───────────────────────────┐
│ CAMADA 3 —          │                   │ CAMADA 4 —                  │
│ SECURITY LAYER (WAF)│                   │ DECISION ENGINE             │
│ - IP blacklist/     │                   │ - Resolve subdomínio → svc  │
│   whitelist (Redis) │                   │ - Resolve path → svc        │
│ - Rate limit por     │                   │ - Regras dinâmicas (hot     │
│   IP/rota (Redis)    │                   │   reload via Postgres)      │
│ - Spike detection    │                   │ - Prioridade de regras      │
│ - Regras WAF          │                   └───────────────┬──────────────┘
│   (payload patterns)  │                                    │
└───────────────────────┘                                    ▼
                                                ┌───────────────────────────┐
                                                │ CAMADA 5 —                  │
                                                │ INTERNAL SERVICES            │
                                                │ (rede privada, sem porta     │
                                                │  pública, mTLS/token interno)│
                                                └───────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  CAMADA 6 — PAINEL ADMINISTRATIVO (Next.js)                       │
│  - Consome WebSocket do Analytics Core (eventos em tempo real)    │
│  - Consome REST API do Analytics Core (config, ações)             │
│  - Autenticação própria (admin), não exposto como serviço interno │
│    "genérico" — é tratado como cliente privilegiado do Core       │
└─────────────────────────────────────────────────────────────────┘
```

**Princípio central:** o Nginx é a única superfície pública. Tudo que passa por ele converge obrigatoriamente no Analytics Core antes de qualquer decisão de roteamento. Nenhum serviço interno responde a tráfego que não tenha vindo do Core.

---

## 2. Fluxo Detalhado de Request

1. **Client → Nginx**: TLS handshake com SNI resolvendo o subdomínio wildcard. Nginx aplica rate limit grosseiro (proteção contra flood óbvio) e remove/reescreve headers `X-Forwarded-*`, `X-Real-IP` recebidos do cliente (nunca confiar em headers de entrada).
2. **Nginx → Analytics Core**: proxy_pass único, injetando o IP real do cliente de forma controlada (via `proxy_set_header` confiável, não repassando o que veio do client).
3. **Analytics Core recebe o request**:
   - Gera `correlation_id` (UUID v7).
   - Inicia timer de latência.
   - Extrai metadados: IP, método, path, headers, user-agent, tamanho do body.
   - Persiste log inicial (fire-and-forget assíncrono para ClickHouse via canal/buffer).
4. **Analytics Core → Security Layer**: chamada síncrona (in-process ou via Redis, latência-alvo <1ms):
   - IP está em blacklist? → bloqueia (403), loga o bloqueio, retorna imediatamente.
   - Rate limit da rota/IP excedido? → bloqueia (429).
   - Padrão de payload bate com regra WAF (SQLi, XSS, path traversal etc.)? → bloqueia (403).
   - Spike detection: taxa de requests do IP/rota anômala em relação à baseline? → aplica mitigação (challenge, block temporário, ou apenas alerta, dependendo da severidade configurada).
5. **Analytics Core → Decision Engine**: resolve `subdomain + path` → serviço interno de destino, usando cache em memória (hot reload a partir do Postgres, invalidado por evento).
6. **Analytics Core → Internal Service**: proxy reverso com token interno assinado (JWT curto-lived ou mTLS), incluindo o `correlation_id` como header para rastreabilidade ponta a ponta.
7. **Internal Service → Analytics Core**: resposta é capturada, latência final calculada.
8. **Analytics Core**: finaliza o log estruturado (status, latência total, latência do serviço interno, decisão tomada) e envia para ClickHouse. Publica evento no canal WebSocket para o painel (se dentro dos critérios de "evento relevante" — todo request pode ser pesado demais para streaming completo, então aqui vale amostragem ou agregação em janelas curtas, ex: 1s).
9. **Analytics Core → Nginx → Client**: resposta final é repassada.

**Fail Secure**: se o Security Layer ou o Decision Engine não responderem dentro do timeout definido (ex: 50ms), o Analytics Core bloqueia o request com 503 e loga a falha como evento crítico. Nunca há bypass "por segurança indisponível".

---

## 3. Design do Analytics Core (Go)

### Estrutura de diretórios

```
/analytics-core
  /cmd
    /server          # main.go, bootstrap, wiring de dependências
  /gateway
    handler.go        # entrypoint HTTP, orquestra o fluxo do request
    proxy.go           # reverse proxy interno (httputil.ReverseProxy customizado)
  /router
    decision.go         # Decision Engine: resolução de rotas
    rules_cache.go       # cache em memória + hot reload
  /security
    waf.go                # regras de payload
    ratelimit.go            # rate limiting (token bucket via Redis)
    blacklist.go              # IP block/allow list
    anomaly.go                  # spike detection
  /logger
    structured.go              # logger JSON
    clickhouse_writer.go        # batch writer assíncrono
  /realtime
    hub.go                       # WebSocket hub (broadcast de eventos)
    events.go                     # definição de tipos de evento
  /config
    config.go                     # carregamento de config (env + Postgres)
  /models
    request_log.go                 # struct RequestLog
    routing_rule.go                  # struct RoutingRule
    security_rule.go                  # struct SecurityRule
```

### Decisões técnicas chave

- **HTTP server**: `net/http` puro ou `chi`/`fiber` — preferir `net/http` + `httputil.ReverseProxy` para controle fino sobre o pipeline (facilita interceptar request/response para logging sem overhead de framework).
- **Concorrência**: cada request é uma goroutine (padrão Go). Logging para ClickHouse é assíncrono via canal bufferizado + worker pool, para não adicionar latência ao caminho crítico do request.
- **Cache de regras**: `sync.Map` ou estrutura imutável trocada atomicamente (`atomic.Pointer`) a cada hot reload, evitando locks no caminho quente.
- **Rate limit / blacklist**: Redis como fonte de verdade compartilhada entre instâncias (permite escalar o Core horizontalmente sem perder estado).
- **Comunicação interna**: tokens JWT internos de curta duração (ex: 30s), assinados com chave rotacionada, OU mTLS entre Core e serviços internos — mTLS é preferível em produção por não depender de relógio sincronizado e por autenticar mutuamente.
- **Timeouts agressivos**: todo componente (Security Layer, Decision Engine, serviço interno) tem timeout curto e explícito; estouro de timeout = fail closed.

---

## 4. Design do Painel (Next.js)

### Estrutura de diretórios

```
/dashboard
  /app
    /(auth)/login
    /(dashboard)/overview        # real-time analytics
    /(dashboard)/inspector        # request inspector
    /(dashboard)/security          # security dashboard
    /(dashboard)/rules              # routing + WAF rules
  /components
    /charts                          # gráficos (requests/s, latência, erros)
    /tables                           # tabelas de logs, IPs bloqueados
    /realtime                          # componentes ligados ao WebSocket
  /lib
    api-client.ts                      # wrapper REST
    ws-client.ts                        # wrapper WebSocket
  /hooks
    useRealtimeEvents.ts                # hook de assinatura de eventos
    useSecurityRules.ts
  /services
    routing.ts
    security.ts
```

### Funcionalidades e como se conectam ao Core

- **Real-time analytics**: assina o canal WebSocket do Core, que envia eventos agregados em janelas curtas (ex: contadores de 1s: requests/s, taxa de erro, latência p50/p95). Evita floodar o painel com um evento por request.
- **Request inspector**: busca via REST (`GET /api/logs?correlation_id=...` ou filtros por IP/rota/status), com paginação — os dados vêm do ClickHouse, consultado pelo próprio Core (o painel nunca acessa ClickHouse diretamente, sempre via API do Core).
- **Security dashboard**: lista IPs bloqueados, regras WAF ativas, alertas de spike — dados vindos do Postgres (configuração) + Redis (estado em tempo real, ex: contadores de rate limit).
- **Ações em tempo real**: toda ação (banir IP, bloquear rota, alterar rate limit, redirecionar tráfego, desligar serviço) é um `POST/PATCH` REST autenticado que o Core aplica imediatamente (grava em Postgres/Redis e invalida cache local via hot reload).

### Autenticação do painel

O painel deve ter sua própria autenticação (ex: sessão + RBAC de administradores), **separada** da lógica de tokens internos entre Core e serviços. O painel é tratado como "cliente privilegiado" do Core, autenticado via API key/JWT de admin, nunca via os mesmos tokens internos usados entre microserviços.

---

## 5. Modelos de Banco

### PostgreSQL — dados estruturados de configuração (baixa escrita, leitura frequente com cache)

```sql
-- Regras de roteamento
CREATE TABLE routing_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  subdomain TEXT NOT NULL,
  path_pattern TEXT NOT NULL,           -- ex: '/api/*'
  destination_service TEXT NOT NULL,     -- nome interno do serviço
  priority INT NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_routing_subdomain ON routing_rules (subdomain, priority DESC);

-- Regras de segurança
CREATE TABLE security_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_type TEXT NOT NULL,               -- 'blacklist' | 'whitelist' | 'rate_limit' | 'waf_pattern'
  target TEXT NOT NULL,                   -- IP, CIDR, rota, ou regex de payload
  config JSONB NOT NULL DEFAULT '{}',      -- parâmetros específicos (limite, janela, ação)
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ                    -- para blocks temporários
);
CREATE INDEX idx_security_type ON security_rules (rule_type, enabled);

-- Administradores do painel
CREATE TABLE admins (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'viewer',   -- 'owner' | 'admin' | 'viewer'
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### ClickHouse — logs de alto volume (append-only, otimizado para análise)

```sql
CREATE TABLE request_logs (
  correlation_id UUID,
  timestamp DateTime64(3),
  ip String,
  user_agent String,
  method LowCardinality(String),
  path String,
  subdomain LowCardinality(String),
  headers String,                -- JSON serializado
  body_size UInt32,
  status_code UInt16,
  latency_ms UInt32,
  service_target LowCardinality(String),
  security_decision LowCardinality(String),  -- 'allowed' | 'blocked_waf' | 'blocked_rate_limit' | 'blocked_blacklist'
  waf_rule_matched Nullable(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (subdomain, timestamp)
TTL timestamp + INTERVAL 90 DAY;   -- retenção configurável
```

> Body completo (quando aplicável) deve ser truncado/opcional por política de privacidade e volume — logar corpo completo de todo request escala mal e pode violar dados sensíveis. Recomenda-se logar body apenas sob flag de debug por rota, ou armazenar hash/tamanho por padrão.

### Redis — estado efêmero e de alta frequência

- `blacklist:{ip}` → TTL configurável.
- `ratelimit:{ip}:{route}` → contador com janela deslizante.
- `routing_cache:{subdomain}:{path}` → cache de resolução (invalidado em hot reload).

---

## 6. Estratégia de Segurança Completa

### Isolamento de rede
- Todos os serviços internos em uma Docker network dedicada, sem `ports:` publicados no host.
- Apenas o container do Nginx expõe 80/443.
- Comunicação Core ↔ serviços internos via mTLS (certificados internos emitidos por uma CA privada, rotacionados periodicamente) ou, como alternativa mais simples, JWT de curta duração assinado com chave simétrica rotacionada.

### Zero Trust
- Nenhum header vindo do cliente (`X-Forwarded-For`, `X-Real-IP`, etc.) é confiado sem reescrita explícita no Nginx.
- Cada request entre camadas internas carrega o `correlation_id` e um token de autenticação de serviço — mesmo tráfego "interno" é autenticado.

### Fail Secure
- Timeouts curtos e explícitos entre Core → Security Layer → Decision Engine → Internal Service.
- Qualquer falha de dependência crítica (Redis fora do ar, Postgres inacessível para hot reload) resulta em bloqueio do tráfego, não em passagem livre.
- Circuit breaker no Core: se um serviço interno está falhando consistentemente, o Core para de rotear para ele e retorna 503 controlado, evitando cascata.

### WAF lógico
- Regras baseadas em padrões conhecidos (SQLi, XSS, path traversal, LFI/RFI) aplicadas ao path, query string e, quando habilitado, ao body.
- Regras customizáveis via painel, armazenadas em `security_rules` (Postgres), com hot reload.
- Severidade escalonável: `log_only` → `challenge` → `block`.

### Anti-abuso
- Rate limiting em duas camadas: grosseiro no Nginx (proteção contra flood bruto) e fino no Core (por IP, por rota, por usuário autenticado se aplicável).
- Spike detection: comparação de taxa de requests contra baseline móvel (ex: média das últimas N janelas); desvio acima de threshold dispara alerta e, dependendo da config, block automático temporário.
- Blacklist/whitelist dinâmica, editável em tempo real pelo painel, propagada via Redis (efeito imediato em todas as instâncias do Core).

---

## 7. Estratégia de Deploy na VPS (PM2, sem Docker)

Sem Docker, o isolamento de rede não vem "de graça" (não há network namespace separando os serviços). O isolamento precisa ser recriado em nível de sistema operacional. Os princípios da seção 6 continuam obrigatórios — só muda o mecanismo.

### Princípio de isolamento sem containers

- **Bind em localhost, nunca em `0.0.0.0`**: todo serviço interno (serviços internos, Postgres, ClickHouse, Redis) deve escutar apenas em `127.0.0.1` (ou socket Unix). Isso já impede acesso externo direto, independente de firewall.
- **Firewall como segunda camada** (defesa em profundidade): `ufw`/`iptables` liberando apenas 80/443 (Nginx) e a porta de SSH administrativo. Todas as demais portas bloqueadas por padrão, mesmo que algo escute incorretamente em `0.0.0.0` por engano futuro.
- **Apenas o Nginx** deve ter porta acessível publicamente — o Analytics Core é alcançado pelo Nginx via `127.0.0.1:porta`, nunca pela interface pública.
- **Usuários de sistema separados** por serviço (ex: `svc-analytics`, `svc-dashboard`, `svc-postgres`) com permissões mínimas, reduzindo o raio de impacto caso um processo seja comprometido.
- **mTLS/token interno continua valendo** mesmo em localhost — proteção adicional caso, no futuro, algum serviço precise escutar em interface diferente de loopback (ex: se a VPS crescer para múltiplas máquinas).

### Processos gerenciados pelo PM2

```js
// ecosystem.config.js
module.exports = {
  apps: [
    {
      name: "analytics-core",
      script: "./analytics-core/bin/server",   // binário Go compilado
      instances: 1,                              // ver nota abaixo sobre escalar Go com PM2
      exec_mode: "fork",
      env: { PORT: 8081, BIND_ADDR: "127.0.0.1", NODE_ENV: "production" },
      max_memory_restart: "500M",
      autorestart: true,
      watch: false
    },
    {
      name: "dashboard",
      script: "npm",
      args: "start",                              // next start
      cwd: "./dashboard",
      instances: 1,
      env: { PORT: 3001, HOSTNAME: "127.0.0.1" },
      autorestart: true
    }
  ]
};
```

```
pm2 start ecosystem.config.js
pm2 startup && pm2 save   # garante restart automático após reboot da VPS
```

> **Nota sobre escalar o Core com PM2**: o modo `cluster` do PM2 é um recurso específico do runtime Node.js (usa o módulo `cluster` interno, com round-robin nativo na mesma porta). Para o binário Go do Analytics Core, isso não se aplica. Para escalar horizontalmente:
> - Rode N instâncias do binário Go como apps PM2 separados (`analytics-core-1`, `analytics-core-2`, ...), cada uma em fork mode e em uma porta distinta (8081, 8082, 8083...), e deixe o **Nginx fazer o load balancing** entre elas via `upstream` — padrão recomendado, simples e robusto.
> - Alternativa mais avançada: `SO_REUSEPORT` configurado no próprio binário Go, permitindo múltiplas instâncias na mesma porta com balanceamento pelo kernel — mais eficiente, mas exige suporte explícito no código.

### Nginx com múltiplas instâncias do Core

```nginx
upstream analytics_core {
    least_conn;
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
    server 127.0.0.1:8083;
}

server {
    listen 443 ssl;
    server_name *.merchonline.com.br;
    # ... configuração TLS ...

    location / {
        proxy_pass http://analytics_core;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;   # sobrescreve qualquer valor vindo do cliente
    }
}
```

### Componentes de infraestrutura sem Docker

- **Postgres, ClickHouse, Redis**: instalados nativamente na VPS (pacotes do sistema ou binários oficiais), configurados para bind apenas em `127.0.0.1`, e gerenciados via `systemd` — não via PM2. PM2 é focado em processos de aplicação (Node/Go); bancos de dados ficam mais estáveis sob `systemd`, com restart policies e integração de logs próprias do SO.
- **PM2** gerencia especificamente os componentes de aplicação que você desenvolve e atualiza com frequência: instâncias do Analytics Core (Go) e o Dashboard (Next.js).
- Certificados TLS: Let's Encrypt com wildcard `*.merchonline.com.br` via DNS challenge (`certbot` com plugin do provedor DNS), renovação automática via cron/systemd timer.
- Observabilidade: `pm2 monit` / `pm2 logs` para operação do dia a dia, complementado por métricas Prometheus expostas localmente pelo Core e pelos logs estruturados indo para o ClickHouse como já descrito.

---

## 8. Pontos de Falha e Mitigação

| Ponto de falha | Impacto | Mitigação |
|---|---|---|
| Analytics Core cai | Todo tráfego para | Múltiplas réplicas + fail closed é aceitável (melhor indisponível que inseguro); health check no Nginx para tirar réplicas mortas do pool |
| Redis indisponível | Rate limit/blacklist não avaliáveis | Fail closed (bloquear) ou modo degradado explícito (config), nunca "permitir tudo" silenciosamente |
| Postgres indisponível | Hot reload de regras para | Core continua operando com o último cache válido em memória; alerta crítico disparado |
| ClickHouse indisponível | Logs não persistidos | Buffer local com backpressure; se buffer encher, logar erro e (decisão de produto) optar por dropar logs vs. bloquear tráfego — recomenda-se **não** bloquear tráfego só por falha de log, mas alertar imediatamente |
| Certificado TLS expira | Todo domínio fica inacessível | Renovação automatizada (certbot + cron/systemd timer) com alerta antecedente |
| Serviço interno lento/instável | Latência alta ou timeouts em cascata | Circuit breaker no Core, timeout por serviço, fallback de resposta controlada |
| Ataque de amplificação/flood | Sobrecarga da VPS inteira | Rate limit em camada dupla (Nginx + Core), possibilidade de CDN/proxy externo (Cloudflare) na frente do Nginx como camada adicional |

---

## 9. Sugestões de Evolução Futura

- **CDN/edge externo** (ex: Cloudflare) na frente do Nginx para absorver DDoS volumétrico antes de chegar à VPS.
- **Machine learning para anomaly detection**: substituir threshold estático por modelo de baseline adaptativo por rota/horário.
- **Distributed tracing** (OpenTelemetry) end-to-end, complementando o `correlation_id` com spans detalhados entre Core e serviços internos.
- **Multi-região**: se o tráfego crescer, replicar o Analytics Core em outra região com roteamento por latência (GeoDNS), mantendo Postgres/ClickHouse centralizados ou replicados.
- **Políticas de retenção e privacidade de dados**: anonimização/hash de IPs após período configurável, para conformidade com LGPD.
- **API pública de regras** (assinada, versão "self-service") para que times internos criem regras de rota sem depender do painel manual.
- **Canary routing**: suporte no Decision Engine para rotear uma % do tráfego para uma nova versão de serviço interno, permitindo deploys graduais.

---

*Este blueprint cobre a especificação completa solicitada. Próximos passos possíveis: gerar o código-base inicial (Go + Next.js) ou um `docker-compose.yml` funcional para deploy na VPS.*
