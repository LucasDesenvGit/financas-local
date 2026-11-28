### Correções
- **"Sair" encerra o acesso de verdade** (#68). Depois de sair, a sessão não vale mais em nenhum aparelho (computador e celular: é preciso entrar de novo), e quem usa sem parar entra de novo a cada 12 horas.
  - Técnico: "Sair" troca o `CarimboSeguranca`; o cookie guarda o login original (teto de 12 h); certificado com `BasicConstraints` explícito, sem CA.
