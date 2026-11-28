### Funcionalidades
- **"Recebido na conta" conta o adiantamento** (#768). Quando o holerite (lido do PDF) desconta o adiantamento, o esperado do mês é o líquido mais o adiantamento, e a tela mostra "adiantamento R$ X + saldo R$ Y". Quem recebe em duas vezes e liga os dois créditos não vê mais valor "a mais". As sugestões dizem "parece o adiantamento" ou "parece o saldo".
  - Técnico: `adiantamento` e `esperado` na resposta de `/api/trabalho/holerites/{id}/recebimentos`.
