# 0019. Status da sincronização por consulta periódica a um endpoint leve

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois da decisão, no PR #52 — UX-8)
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (sincronização com a Pluggy)

## Contexto

Não havia como saber, sem abrir Configurações, se os dados estavam atualizados, se uma sincronização automática estava rodando ou se alguma conexão tinha falhado (Nielsen 1, visibilidade do estado do sistema). A barra superior precisava mostrar "Atualizado há 2 h", "Sincronizando…" e "1 conexão com erro", inclusive quando a sincronização é a automática do servidor.

## Alternativas consideradas

- **Push do servidor (SignalR/WebSocket ou Server-Sent Events)** — atualização imediata; mas é infraestrutura nova (conexão permanente, reconexão, autenticação do canal) para um dado que muda poucas vezes por dia.
- **Recarregar o Painel inteiro periodicamente** — sem endpoint novo; pesado e só serve para uma tela.
- **Consulta periódica (polling) a um endpoint leve** — simples, usa a autenticação por cookie que já existe.

## Decisão

O front-end consulta `GET /api/open-finance/status` (`StatusSincronizacaoDto`: última sincronização, se há uma em execução, conexões ativas, com erro e consentimentos vencendo — só contagens lidas do banco, sem chamar a Pluggy). O `StatusSincronizacaoService` consulta a cada **2 min**, a cada **10 s** enquanto há sincronização rodando, e de novo quando a aba volta a ficar visível; o "há X min" avança a cada 30 s sem consultar. Mudanças de estado são anunciadas a leitores de tela (`aria-live="polite"`).

## Consequências

- **Positivas:** estado sempre visível na barra; endpoint barato; nenhuma infraestrutura nova.
- **Negativas:**
  - atraso de até 2 min para perceber uma sincronização automática que começou (10 s depois que já se sabe que está rodando);
  - uma requisição a cada 2 min por aba aberta, mesmo sem nada mudar (desprezível para uso local, mas existe);
  - se um dia houver mais estados em tempo real, polling em vários lugares pode justificar trocar por push.
- **Na prática:** o endpoint de status não pode ficar caro (nada de chamar a Pluggy ou varrer transações nele); quem precisar do status usa o `StatusSincronizacaoService` (contador de usuários com `iniciar()`/`parar()`), não um novo polling.
