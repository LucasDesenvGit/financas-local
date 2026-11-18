### Funcionalidades
- **Emprego que paga em duas vezes** (#765). Em Trabalho › Salário, "Recebo em duas vezes" marca que o salário vem com o adiantamento no dia 15 e o saldo no fim do mês. Ao ler o holerite em PDF, o site avisa se o emprego paga assim e o PDF não desconta o adiantamento (costuma ser o recibo do dia 15, que não é o holerite do mês).
  - Técnico: coluna `Emprego.AdiantamentoQuinzenal` (`adicionar_coluna_se_ausente`) e o campo `adiantamentoQuinzenal` no JSON do emprego.
