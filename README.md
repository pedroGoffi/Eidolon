# Eidolon

Sistema de analytics + gateway + segurança + painel administrativo.

## Estrutura

```
eidolon/
  analytics-core/   # Núcleo em Go: recebe tráfego, aplica WAF/rate-limit,
                     # roteia para serviços internos, expõe API admin.
  dashboard/         # Painel Next.js: visão geral, request inspector,
                     # blacklist de IPs, configuração de rotas/portas.
```

## 1. Subir o Analytics Core

```bash
cd analytics-core
go build -o bin/eidolon-core ./cmd/server
ADMIN_TOKEN=troque-por-um-segredo-forte ./bin/eidolon-core
```

Sobe por padrão em `http://127.0.0.1:8081`. Variáveis de ambiente disponíveis
em `internal/config/config.go` (rate limit, timeouts, etc).

**`ADMIN_TOKEN`** protege as ações de escrita da API (banir IP, criar/editar/
remover rota). Sem ele definido, a API fica sem autenticação — só use assim
em desenvolvimento local.

## 2. Subir o painel

```bash
cd dashboard
npm install
npm run dev
```

Abre em `http://localhost:3001`. Na primeira vez, vá em **Conexão** (menu
lateral) e configure:

- **URL do Analytics Core**: `http://127.0.0.1:8081` (ou o endereço real da
  sua VPS, se estiver rodando remoto)
- **Token de admin**: o mesmo valor que você colocou em `ADMIN_TOKEN`

O token fica salvo só no `localStorage` do seu navegador.

## 3. Testar o fluxo

Com os dois serviços no ar:

1. Vá em **Rotas & portas** e cadastre uma rota, ex: subdomínio `app`,
   path `/api/*`, destino `127.0.0.1:9001` (porta do seu serviço interno real).
2. Mande uma requisição de teste:
   ```bash
   curl http://127.0.0.1:8081/api/teste -H "Host: app.merchonline.com.br"
   ```
3. Veja o evento aparecer em tempo real na **Visão geral** e o log completo
   no **Request inspector**.
4. Em **Segurança**, bloqueie um IP e confirme que requests dele passam a
   receber 403 imediatamente.

## Produção (VPS com PM2, sem Docker)

Ver seção 7 do blueprint técnico (`blueprint-analytics-core.md`) para o
`ecosystem.config.js`, configuração do Nginx como `upstream`, e as regras de
isolamento de rede via bind em `127.0.0.1` + firewall (já que não há
network namespace do Docker aqui).

Resumo rápido:

```bash
pm2 start ./analytics-core/bin/eidolon-core --name eidolon-core \
  --env ADMIN_TOKEN=seu-token-aqui

cd dashboard && npm run build
pm2 start npm --name eidolon-dashboard -- start
pm2 startup && pm2 save
```

O Nginx deve ser a única coisa exposta publicamente; tanto o Core (`8081`)
quanto o painel (`3001`) devem ouvir só em `127.0.0.1`, com o Nginx
fazendo proxy para um subdomínio de administração (ex:
`painel.merchonline.com.br`) — protegido por autenticação adicional
(basic auth no Nginx, VPN, ou IP allowlist), já que o painel em si tem
acesso a ações destrutivas (banir IP, redirecionar tráfego).

## Limitações conhecidas desta versão (propositais, dado o ambiente sem
acesso a módulos externos do Go em que foi gerada)

- Blacklist, rate limiter e anomaly detection são **em memória** — não
  compartilham estado entre múltiplas instâncias do Core. Para escalar
  horizontalmente de verdade, trocar por Redis (a interface já isola essa
  troca, ver `internal/security/`).
- Logs vão para stdout via `AsyncSink` — trocar `logger.StdoutWriter` por
  um writer real de ClickHouse quando for para produção com volume alto.
- Realtime usa SSE em vez de WebSocket (mesmo papel, unidirecional).
- Regras de roteamento/segurança vivem em memória no processo — reiniciar
  o Core volta pro estado inicial hardcoded em `loadInitialRoutes`. Trocar
  por Postgres real quando for produção (estrutura de tabelas já está no
  blueprint).
