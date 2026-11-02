Closes #<!-- número da issue -->

## O que muda

<!-- Resumo em 1-3 linhas. -->

## Tipo

- [ ] Funcionalidade (`feat`)
- [ ] Correção (`fix`)
- [ ] Refatoração / manutenção (`refactor`, `perf`, `chore`, `style`)
- [ ] Documentação (`docs`)
- [ ] CI (`ci`), sem mudança de produto junto (ADR 0039)
- [ ] Testes (`test`) ou build (`build`)

## Checklist

- [ ] Versão subiu no `Directory.Build.props`, pelo tipo do título (ADR 0039; o check "versão subiu" confere)
- [ ] `CHANGELOG.md`: a entrada na seção da versão deste PR, dentro do título do tipo que já existe nela (`python3 scripts/ci/verificar-changelog.py`)
- [ ] `dotnet build FinancasLocal.sln -c Release` sem erros
- [ ] Front-end: `npm run build`, `npm run test:tipos` e `npm test` em `src/frontend`, e `NAVEGADOR=chromium npm run test:afetados` antes do push (bug corrigido ganha teste de regressão)
- [ ] Regras novas do back-end Go com teste unitário (ADR 0051): tabela regra → teste no PR, consulta nova no SQLite real, endpoint novo com caso de contrato, mutações listadas; `gofmt`, `go vet` e `go test -race` limpos
- [ ] Mexeu em import, pacote ou pasta nova (Go ou front-end)? Teste de arquitetura, que só roda local: `go test -tags arquitetura ./internal/arquitetura/` e `npm run test:arquitetura`
- [ ] Mudou o banco? Tabela ou índice novo idempotente em `banco/01-estrutura.sql`; coluna nova com `adicionar_coluna_se_ausente` no `scripts/geral/2-criar-banco.sh`; entidade e `FinancasDbContext` em sincronia; `feat(banco)!` no título (sem migrations)
- [ ] Decisão de arquitetura (dependência, integração, banco, segurança, infraestrutura, padrão de código)? ADR criada ou atualizada no mesmo PR, ou o motivo de não precisar
- [ ] Mexeu em `instalador/`, `src/gerenciador/`, `Program.cs` ou `EsquemaBanco.cs`? Rodei o workflow "Versão (instalador .msi)" à mão nesta branch (ADR 0050)
- [ ] Mexeu em teste ou em CI? O tempo antes e depois, por job, está aqui
- [ ] Consultas só com LINQ/lambda
- [ ] Nenhum dado pessoal, senha, chave ou `appsettings.Production.json`
- [ ] **Manual do site** (`src/frontend/src/app/paginas/manual-conteudo.ts`) atualizado se mudou algo que o usuário vê; se não precisa, escreva aqui "Manual: não se aplica" (a CI avisa quando falta)

## Depois do merge

- [ ] Quem instala pelos scripts atualiza com `git pull` e `scripts/geral/instalar.sh` (a API aplica o esquema do banco sozinha ao iniciar)
