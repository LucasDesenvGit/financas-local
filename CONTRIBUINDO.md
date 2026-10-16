# Como contribuir

Projeto pessoal, mas com fluxo de trabalho organizado para facilitar a manutenção.

## Branches

| Branch | Para quê | Regras |
|---|---|---|
| `main` | Versão estável — o que está publicado em produção | Protegida: só recebe merge por pull request vindo de `develop` ou `hotfix/*` |
| `develop` | Integração do que está pronto para a próxima versão | Protegida: recebe merge por pull request das branches de trabalho |
| `feature/<assunto>` | Nova funcionalidade (ex.: `feature/pontos-cartao`) | Sai de `develop`, volta para `develop` |
| `fix/<assunto>` | Correção sem urgência | Sai de `develop`, volta para `develop` |
| `hotfix/<assunto>` | Correção urgente do que está publicado | Sai de `main`, volta para `main` **e** `develop` |
| `release/<versao>` | (opcional) Ajustes finais antes de publicar | Sai de `develop`, volta para `main` e `develop` |

Fluxo do dia a dia:

```powershell
git switch develop
git pull
git switch -c feature/minha-melhoria
# ... trabalho e commits ...
git push -u origin feature/minha-melhoria
gh pr create --base develop --fill
```

Para publicar uma versão: PR de `develop` para `main`, merge, tag (`git tag v1.1.0; git push --tags`) e `scripts/3-publicar-atualizar.sh`.

## Mensagens de commit

Padrão [Conventional Commits](https://www.conventionalcommits.org/pt-br/), em português:

```
<tipo>(<escopo opcional>): <resumo no imperativo, até ~70 caracteres>

<corpo opcional explicando o porquê>
```

Tipos: `feat` (funcionalidade), `fix` (correção), `refactor`, `perf`, `docs`, `style`, `test`, `build`, `ci`, `chore`.
Escopos usados: `api`, `web`, `banco`, `scripts`, `open-finance`, `investimentos`, `relatorios`.

Exemplos:
- `feat(investimentos): comparar cada compra de FII com a cotação atual`
- `fix(open-finance): não duplicar transações ao renovar o consentimento`
- `feat(banco)!: nova coluna Transacao.Emergencia` (o `!` indica que é preciso rodar `2-criar-banco.sh`)

## Checklist do pull request

- [ ] Compila: `dotnet build FinancasLocal.sln -c Release` e `npm run build` em `src/frontend`.
- [ ] Testes do front-end passam: `npm test` em `src/frontend`. Bug corrigido ganha teste em `testes/regressao/`; padrão novo de UI/UX, em `testes/usabilidade/` ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)).
- [ ] Mudança de banco? Tabela/índice novo com `CREATE TABLE/INDEX IF NOT EXISTS` em `banco/01-estrutura.sql`, ou coluna nova com `adicionar_coluna_se_ausente` (de `scripts/comum.sh`) chamado em `scripts/2-criar-banco.sh` (veja [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md)) — e entidades/`FinancasDbContext` ajustados. Sem migrations.
- [ ] Decisão de arquitetura (dependência ou integração nova, banco, segurança, infraestrutura, padrão de código)? ADR criada ou atualizada em [docs/adr/](docs/adr/README.md), no mesmo PR.
- [ ] Consultas ao banco só com LINQ/lambda sobre entidades mapeadas (sem SQL montado em código).
- [ ] Nada de dado pessoal, senha, chave ou `appsettings.Production.json` no commit.
- [ ] README/CHANGELOG atualizados quando a mudança afeta o uso.

## Convenções de código

- Nomes em português, seguindo o que já existe (ex.: `XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Back-end: um serviço por assunto em `Servicos/`, controllers finos, DTOs em `Dtos/Dtos.cs`.
- Front-end: componentes standalone, signals e control flow (`@if`, `@for`); chamadas HTTP só pelo `ApiService`.
- Formatação definida no `.editorconfig`.
