### Funcionalidades
- **Cotações de ações e FIIs só pelo Yahoo Finance** (#763). A brapi saiu: a janela "Fonte das cotações", em Investimentos, não pede mais token, e o token antigo é apagado do banco.
  - Técnico: sai o endpoint `GET/PUT /api/carteira/configuracao`, a saída de rede para `brapi.dev` e o aviso "O token da brapi precisa ser informado de novo."; ADR 0072.
