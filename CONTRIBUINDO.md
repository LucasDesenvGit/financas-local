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

**O branch padrão do GitHub é o `develop`** (#142).
- **PR novo:** já sugere o `develop` como destino.
- **`Closes #N` na descrição:** vincula o PR à issue (painel *Development* e o workflow "Pull request linked to issue" do Projects) e fecha a issue quando o PR entra no `develop`.
- **Fora do branch padrão, o `Closes #N` não vale.** No PR de `develop` para `main`, ele não faz nada, mas a issue já foi fechada antes.

Fluxo do dia a dia, com a branch criada pela issue:

```powershell
git switch develop
git pull
gh issue develop <número> --base develop --name feature/minha-melhoria --checkout   # ou fix/<assunto>
# ... trabalho e commits; suba a versão no Directory.Build.props (ver abaixo) ...
git push
gh pr create --base develop --fill    # na descrição: Closes #<número>
```

Não há issue? Crie uma antes, já no nome de quem pediu (`gh issue create --assignee @me`): todo PR fecha a issue dele. Ao começar a trabalhar numa issue, ponha a etiqueta "em progresso" (`gh issue edit <número> --add-label "em progresso"`) e tire ao fechar.

**Todo PR para o `develop` sobe a versão** no `Directory.Build.props`, pelo tipo do título ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md)). O check "versão subiu" da CI confere.

| Tipo do PR | Sobe | Exemplo |
|---|---|---|
| `feat` (inclusive `feat(banco)!` que só acrescenta tabela ou coluna) | o número do meio | 2.2.0 → 2.3.0 |
| `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style` | o último | 2.2.0 → 2.2.1 |
| Quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior) | o primeiro | 2.2.0 → 3.0.0 |

Outro PR subiu a versão antes do seu entrar? `git merge origin/develop`, pegue a versão maior e suba de novo pelo tipo do seu PR. O Dependabot não precisa subir.

Para publicar uma versão ([ADR 0029](docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)):
1. No `develop`, no `CHANGELOG.md`, troque `## [Não publicado]` pela versão que está no `Directory.Build.props` (ex.: `## [2.2.0] - AAAA-MM-DD`) e abra uma seção `## [Não publicado]` vazia acima. O número já vem certo dos PRs.
2. PR de `develop` para `main` e merge.
3. Tag na `main`: `git tag v2.2.0; git push --tags`. A tag tem de ser igual ao `Directory.Build.props`, senão o workflow para.
4. O workflow **Versão (instalador .msi)**:
   - gera o `.msi`;
   - testa instalar, atualizar, desinstalar e migrar;
   - publica `FinancasLocal-2.2.0.msi` e `SHA256SUMS.txt` no Release **"Finanças Local 2.2.0"**, com "Como instalar ou atualizar" e a seção do CHANGELOG nas notas. Se alguém criou o Release à mão antes, o título e as notas são acertados.
5. O gerenciador de quem já tem o site instalado passa a oferecer "Atualizar para 2.2.0".

Quem instala pelos scripts continua atualizando com `scripts/geral/3-publicar-atualizar.sh` (ou `instalar-windows.cmd`).

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

## Testes rápidos (padrões de CI)

Em outubro de 2026 o Jest chegou a levar ~6 min na CI (#204): um pacote pesado importado nos testes custava ~15 s em **cada** arquivo de teste. Para não voltar:

**Onde cada teste mora** (um comportamento testado num lugar só):

| O que testar | Onde | Por quê |
|---|---|---|
| Lógica, cálculo, componente isolado, texto, atributo de acessibilidade | **Jest** (`testes/regressao`, `testes/usabilidade`) | rápido, sem navegador ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)) |
| Desenho, CSS calculado (`var()`, grid, tamanho de alvo), rota de verdade, CSP, console, celular | **Navegador** (`testes-navegador/`) | o jsdom não calcula estilo nem aplica a CSP ([ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)) |

Não repita no Jest o que o navegador já confere, e vice-versa. Teste do Jest que lê o `styles.css` por `cssRules` para adivinhar o estilo é candidato a ir para o navegador.

**Limites de tempo (a CI confere):**
- **Jest:** cada arquivo de teste até **10 s** (passou disso, a CI avisa) e no máximo **20 s** (passou, falha); a suíte inteira até **150 s** (`scripts/ci/verificar-tempo-testes.py`). Referência hoje: ~34 s a suíte, 6 s o arquivo mais lento, 0,7 s a mediana (2 workers, como a CI).
- **Navegador:** cada teste até 30 s (padrão do Playwright); a suíte hoje leva ~30 s.

**Importação pesada é a causa nº 1 de teste lento.** Um arquivo de teste importa a tela, que importa os componentes dela, que importam bibliotecas. Biblioteca grande (ícones, gráficos, editores…) carregada no Jest custa em todo arquivo que encosta nela:
- **antes de pôr uma dependência nova no front-end**, rode um teste que a use e veja o tempo (`npx jest testes/<arquivo>.spec.ts`);
- **se pesar**, troque-a nos testes do Jest por um substituto leve em `testes/apoio/`, ligado no `moduleNameMapper` do `jest.config.js`, como o `lucide-falso.js` (ícones). O build e os testes no navegador continuam com a de verdade.

**Como medir quando algo ficou lento:**
1. `npx jest --ci --json --outputFile=jest-resultado.json` e `python3 ../../scripts/ci/verificar-tempo-testes.py jest-resultado.json`: os arquivos mais lentos;
2. **um teste vazio** mostra o custo fixo do ambiente (hoje ~3 s, contando a partida do Jest);
3. **importar peça por peça** num teste temporário (`await import(...)` com o tempo de cada um) acha a importação que pesa. Foi assim que o #204 achou os 15 s do `@lucide/angular`.

**Workflows da CI:**
- todo workflow de PR tem `concurrency` com `cancel-in-progress`;
- filtros `paths:` por pasta (`scripts/geral/**`), não por nome de arquivo, para um arquivo novo também disparar;
- cache do que é baixado sempre igual (npm, NuGet, navegador do Playwright), menos quando baixar do zero é justamente o que se testa (os testes de instalação);
- **mexeu em teste ou em CI? Ponha no PR o tempo antes e depois**, por job e pelos passos mais caros.

## Checklist do pull request

- [ ] Versão subiu no `Directory.Build.props`, pelo tipo do PR ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md)).
- [ ] Compila: `dotnet build FinancasLocal.sln -c Release` e `npm run build` em `src/frontend`.
- [ ] Testes no navegador passam: `npm run test:navegador` em `src/frontend` (a CI também roda; [ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)). Tela nova no menu entra em `TELAS` (`testes-navegador/base.ts`); endpoint novo, em `testes-navegador/api-falsa.ts`. Mudou o visual? Olhe os prints (`FOTOS=rota npx playwright test fotos`).
- [ ] Testes do front-end passam: `npm run test:tipos` e `npm test` em `src/frontend` (a CI roda os dois; o `npm test` sozinho não confere tipos). Bug corrigido ganha teste em `testes/regressao/`; padrão novo de UI/UX, em `testes/usabilidade/` ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)).
- [ ] Mudança de banco? Tabela/índice novo com `CREATE TABLE/INDEX IF NOT EXISTS` em `banco/01-estrutura.sql`, ou coluna nova com `adicionar_coluna_se_ausente` (de `scripts/geral/comum.sh`) chamado em `scripts/geral/2-criar-banco.sh` (veja [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md)) — e entidades/`FinancasDbContext` ajustados. Sem migrations.
- [ ] Decisão de arquitetura (dependência ou integração nova, banco, segurança, infraestrutura, padrão de código)? ADR criada ou atualizada em [docs/adr/](docs/adr/README.md), no mesmo PR.
- [ ] Consultas ao banco só com LINQ/lambda sobre entidades mapeadas (sem SQL montado em código).
- [ ] Nada de dado pessoal, senha, chave ou `appsettings.Production.json` no commit.
- [ ] README/CHANGELOG atualizados quando a mudança afeta o uso.
- [ ] **Manual do site** (`src/frontend/src/app/paginas/manual-conteudo.ts`) atualizado quando muda algo que o usuário vê (tela, fluxo, cálculo, texto). Se não precisa, escreva "Manual: não se aplica" na descrição do PR. A CI avisa (job "manual do site") quando o PR mexe em telas ou endpoints sem mexer no Manual.
- [ ] Mudou o jeito de usar uma tela (botão, campo, filtro, passo)? Atualize a seção do **Manual** em `src/frontend/src/app/paginas/manual-conteudo.ts`; tela nova no menu entra também em `AJUDA_POR_ROTA` ([ADR 0026](docs/adr/0026-manual-do-site-como-dados-no-front-end.md)).

## Convenções de código

- Nomes em português, seguindo o que já existe (ex.: `XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Back-end: um serviço por assunto em `Servicos/`, controllers finos, DTOs em `Dtos/Dtos.cs`.
- Front-end: componentes standalone, signals e control flow (`@if`, `@for`); chamadas HTTP só pelo `ApiService`.
- Formatação definida no `.editorconfig`.
