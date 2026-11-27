### Correções
- **FGTS, recebimento e rescisão: pontos da revisão** (#751). As férias pagas em dobro não têm mais FGTS sobre a dobra, e conferências de antes de 03/2024 usam o vencimento do dia 7. A tela do FGTS explica o saldo acima do digitado. Na rescisão, você confirma se está no saque-aniversário, e a conferência do app no mês da saída vale.
  - Técnico: `Conferencia.pendente`, `Recebimento.valorMudou`, `saqueAniversario` na simulação, comparação e registro; Registrar confere dentro da transação (409); ADRs 0065 e 0066.
