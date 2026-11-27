Closes #<!-- número da issue -->

## O que muda

<!-- Resumo em 1-3 linhas. -->

## Tipo

- [ ] Funcionalidade (`feat`)
- [ ] Correção (`fix`)
- [ ] Refatoração / manutenção (`refactor`, `perf`, `chore`, `style`)
- [ ] Documentação (`docs`)
- [ ] CI (`ci`), sem mudança de produto junto (ADR 0070)
- [ ] Testes (`test`) ou build (`build`)

## Checklist

Só o que a CI não confere (o resto ela barra ou avisa: [CONTRIBUINDO.md](../CONTRIBUINDO.md#o-que-a-ci-roda)).

- [ ] Título no tipo certo: ele decide a versão (ADR 0070); quebra de verdade, só depois de perguntar ao usuário
- [ ] Back-end Go (ADR 0051): tabela regra → teste e mutações à mão listadas aqui; consulta nova com teste no SQLite real; endpoint novo com caso de contrato
- [ ] Mexeu em import, pacote ou pasta nova (Go ou front-end)? Rodei o teste de arquitetura, que só roda local
- [ ] Mudou o banco? Segui o [docs/BANCO-DE-DADOS.md](../docs/BANCO-DE-DADOS.md#como-alterar-a-estrutura), com `feat(banco)!` no título
- [ ] Simples e enxuto ([docs/SIMPLICIDADE.md](../docs/SIMPLICIDADE.md)): o que é novo tem um segundo uso real ou uma falha medida; texto dentro dos limites
- [ ] Decisão de arquitetura? ADR criada ou atualizada no mesmo PR, ou o motivo de não precisar
- [ ] Mexeu em `instalador/`, `src/gerenciador-go/`, `src/backend/cmd/financas/` ou `esquema_banco.go`? Rodei o workflow "Versão (instalador .msi)" à mão nesta branch (ADR 0050)
- [ ] Mexeu em teste ou em CI? O tempo antes e depois, por job, está aqui
- [ ] Consultas ao banco só parametrizadas (com `?`)
- [ ] Nenhum dado pessoal, senha, chave ou `appsettings.Production.json`
- [ ] **Manual do site** e README atualizados se mudou algo que o usuário vê; se não precisa, escreva aqui "Manual: não se aplica"
