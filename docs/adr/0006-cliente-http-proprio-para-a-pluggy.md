# 0006. Cliente HTTP próprio para a Pluggy, sem SDK

- **Status:** Aceita
- **Data:** 2026-10-15

## Contexto

Os dados bancários chegam pelo Open Finance via Meu Pluggy. A integração usa uma parte pequena da API da Pluggy: autenticação, item (conexão), contas, faturas de cartão, transações (`/v2/transactions`, com paginação por cursor), investimentos e empréstimos. A sincronização roda em segundo plano (`SincronizacaoHostedService`).

*Inferido:* não havia SDK oficial e mantido da Pluggy para .NET que justificasse a dependência, e o projeto prefere ter o mínimo de dependências de terceiros num código que manuseia credenciais bancárias.

## Alternativas consideradas

- **SDK de terceiros/comunitário** — menos código; mas é uma dependência a mais tocando credenciais, com atualização incerta e sem controle do que é enviado.
- **Cliente HTTP próprio** (`HttpClient` tipado + modelos próprios).

## Decisão

Usamos um cliente HTTP próprio em `Servicos/Pluggy/` (`PluggyClient` registrado com `AddHttpClient`), com modelos só dos campos usados.

## Consequências

- **Positivas:** controle total sobre o que é enviado e recebido; sem dependência extra; o mesmo padrão serve para as outras integrações (Banco Central, Yahoo Finance; a brapi, que também seguia esse padrão, saiu na #763, [ADR 0072](0072-cotacoes-de-acoes-e-fiis-so-pelo-yahoo-sem-a-brapi.md)).
- **Negativas:** mudanças na API da Pluggy (campos, versões de endpoint, paginação) precisam ser acompanhadas à mão; não há tipagem gerada.
- **Na prática:** integração externa nova segue o mesmo padrão — cliente `http.Client` com URL base HTTPS, tempo limite e `User-Agent` (como em `internal/patrimonio/cripto/cliente.go`), envio do dado mínimo, e o domínio acrescentado na tabela "O que sai para a internet" do [`docs/SEGURANCA.md`](../SEGURANCA.md#o-que-sai-para-a-internet).
- **No Go (#83):** o mesmo cliente, em `internal/openfinance/pluggy`, também sem SDK. A mais do que o .NET: não segue redirecionamento para fora do host configurado, porque o `net/http` repassa o `X-API-KEY`.
