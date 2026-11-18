### Funcionalidades
- **Emprego que paga em duas vezes** (#765). Em Trabalho › Salário, "Recebo em duas vezes" marca o salário com adiantamento no dia 15 e saldo no fim do mês. Ao ler o holerite em PDF, o site avisa se o PDF não desconta o adiantamento (costuma ser o recibo do dia 15).
  - Técnico: coluna `Emprego.AdiantamentoQuinzenal` (`adicionar_coluna_se_ausente`) e o campo `adiantamentoQuinzenal` no JSON do emprego.
