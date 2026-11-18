### Funcionalidades
- **Cotações de ações e FIIs só pelo Yahoo Finance** (#763). A brapi, que era a reserva quando o Yahoo não respondia, saiu: a janela "Fonte das cotações", em Investimentos, não pede mais token, e o token antigo é apagado do banco. Some também o aviso "O token da brapi precisa ser informado de novo.".
  - Técnico: sai o endpoint `GET/PUT /api/carteira/configuracao` e a saída de rede para `brapi.dev`; ADR 0072.
