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

- [ ] Título no tipo certo: é ele que decide a versão, calculada no merge (ADR 0070). Não mexa no `Directory.Build.props`; quebra de verdade só depois de perguntar ao usuário (a `<VersaoBase>` vira `X+1.0.0`)
- [ ] Um arquivo em `changelog.d/<nome-da-branch>.md` com a entrada, obrigatório em `feat` e `fix` (modelo em `changelog.d/LEIA-ME.md`; `python3 scripts/ci/versao/verificar-changelog.py`). Não mexa no `CHANGELOG.md`
- [ ] Front-end: `npm run build`, `npm run test:tipos` e `npm test` em `src/frontend`, e `NAVEGADOR=chromium npm run test:afetados` antes do push (bug corrigido ganha teste de regressão)
- [ ] Regras novas do back-end Go com teste unitário (ADR 0051): tabela regra → teste no PR, consulta nova no SQLite real, endpoint novo com caso de contrato, mutações listadas; `gofmt`, `go vet` e `go test -race` limpos
- [ ] Mexeu em import, pacote ou pasta nova (Go ou front-end)? Teste de arquitetura, que só roda local: `go test -tags arquitetura ./internal/arquitetura/` e `npm run test:arquitetura`
- [ ] Mudou o banco? Tabela ou índice novo idempotente em `banco/01-estrutura.sql`; coluna nova com `adicionar_coluna_se_ausente` no `scripts/geral/2-criar-banco.sh`; `feat(banco)!` no título (sem migrations)
- [ ] Simples e enxuto ([docs/SIMPLICIDADE.md](../docs/SIMPLICIDADE.md)): abstração, pacote, dependência ou check novo tem um segundo uso real ou uma falha medida; texto novo dentro dos limites (ADR ~1,5 mil palavras, README 10 a 20 linhas, entrada do CHANGELOG ~60 palavras)
- [ ] Decisão de arquitetura (dependência, integração, banco, segurança, infraestrutura, padrão de código)? ADR criada ou atualizada no mesmo PR, ou o motivo de não precisar
- [ ] Mexeu em `instalador/`, `src/gerenciador-go/`, `src/backend/cmd/financas/` ou `src/backend/internal/plataforma/banco/esquema_banco.go`? Rodei o workflow "Versão (instalador .msi)" à mão nesta branch (ADR 0050)
- [ ] Mexeu em teste ou em CI? O tempo antes e depois, por job, está aqui
- [ ] Consultas ao banco só parametrizadas (`?` ou o `sqlc`), nunca SQL montado com texto do usuário
- [ ] Nenhum dado pessoal, senha, chave ou `appsettings.Production.json`
- [ ] **Manual do site** (`src/frontend/src/app/paginas/manual/manual-conteudo.ts`) atualizado se mudou algo que o usuário vê; se não precisa, escreva aqui "Manual: não se aplica" (a CI avisa quando falta)

## Depois do merge

- [ ] Quem instala pelos scripts atualiza com `git pull` e `scripts/geral/instalar.sh` (a API aplica o esquema do banco sozinha ao iniciar)
