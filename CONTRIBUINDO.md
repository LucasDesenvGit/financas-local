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

Não há issue? Crie uma antes, já no nome de quem pediu (`gh issue create --assignee @me`): todo PR fecha a issue dele. **Épicos e as histórias deles ficam sem ninguém atribuído:** só a história que alguém pega para fazer vai para o nome dessa pessoa; o épico continua sem dono. Ao começar a trabalhar numa issue, ponha a etiqueta "em progresso" (`gh issue edit <número> --add-label "em progresso"`) e tire ao fechar.

**Todo PR para o `develop` sobe a versão** no `Directory.Build.props`, pelo tipo do título ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md)). O check "versão subiu" da CI confere.

| Tipo do PR | Sobe | Exemplo |
|---|---|---|
| `feat` (inclusive `feat(banco)!` que só acrescenta tabela ou coluna) | o número do meio | 2.2.0 → 2.3.0 |
| `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style` | o último | 2.2.0 → 2.2.1 |
| Quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior) | o primeiro | 2.2.0 → 3.0.0 |

Outro PR subiu a versão antes do seu entrar? `git merge origin/develop`, pegue a versão maior e suba de novo pelo tipo do seu PR. O Dependabot não precisa subir.

Para publicar uma versão ([ADR 0029](docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)):
1. No `develop`, no `CHANGELOG.md`, tire o ` · desenvolvimento` do título da versão do topo (ex.: `## [2.27.0] — 2026-10-21 · desenvolvimento` vira `## [2.27.0] — 2026-10-21`, com a data de hoje) (#294). O número já vem certo dos PRs, e as versões de desenvolvimento abaixo dela continuam com o rótulo: as notas do Release juntam todas, até a última publicada. **Depois do merge na `main`, traga a `main` de volta para o `develop`**, para o `CHANGELOG` do `develop` ficar igual ao publicado (na 2.17.0 isso não foi feito, e o `develop` ficou sem a seção dela).
2. PR de `develop` para `main` e merge.
3. Tag na `main`: `git tag v2.2.0; git push --tags`. A tag tem de ser igual ao `Directory.Build.props`, senão o workflow para.
4. O workflow **Versão (instalador .msi)**:
   - gera o `.msi`;
   - testa instalar, atualizar, desinstalar e migrar;
   - publica `FinancasLocal-2.2.0.msi` e `SHA256SUMS.txt` no Release **"Finanças Local 2.2.0"**, com "Como instalar ou atualizar" e, do CHANGELOG, a seção da versão e as de desenvolvimento abaixo dela até a última publicada. Se alguém criou o Release à mão antes, o título e as notas são acertados.
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

Não repita no Jest o que o navegador já confere, e vice-versa.

**Localmente, rode só o que mudou** (#286): `cd src/frontend && NAVEGADOR=chromium npm run test:afetados`.
- **O que ele roda:** a mesma escolha que a CI faz no PR (`scripts/ci/testes-afetados.py`, ADR 0047), contando também o que ainda não foi commitado. Ou seja:
  - o Jest dos arquivos ligados à mudança;
  - o build;
  - os testes no navegador das telas afetadas.
- **Mudou arquivo global** (estilos, `core/`, configuração): roda tudo.
- **O front-end não mudou:** não roda nada.
- **Tempo (medido em 21/10/2026):** numa mudança de uma tela, ~45 s, contra ~2,7 min da suíte inteira. A suíte inteira continua rodando na CI, no merge.
- **O Python 3 é achado sozinho:** `python` no Windows, `python3` no Linux.

- **Estilo calculado se testa no navegador**, em `testes-navegador/estilos.spec.ts`: layout (alturas, colunas, largura), borda, grid/subgrid, `var()`, quem vence na cascata, animação, "reduzir movimento", tamanho de alvo. **Não escreva teste do Jest que procura a regra no `styles.css`** (`cssRules`, ordem das regras no arquivo): ele quebra quando a regra é reescrita com o mesmo efeito e passa quando outra regra a anula. Propriedade simples que o jsdom calcula (`width`, `min-height`, `visibility`) pode ficar no Jest.
- **Meça o efeito, não a regra:** "os três cards têm a mesma altura" (`boundingBox`), não "existe `align-items: stretch`".
- **Teste novo de estilo prova que falha:** estrague a regra no CSS do build (`dist/…/styles-*.css`), rode o teste, veja falhar e restaure. Se não falhar, os dados da API falsa provavelmente não exercitam o caso (ex.: dois cards com o mesmo conteúdo ficam iguais mesmo sem a regra): use conteúdos de tamanhos diferentes.

**Antes de subir (push) mudança no front-end, rode os testes no navegador com o mesmo navegador da CI** (#264: passou no Edge daqui e falhou na CI):
- `npx playwright install chromium` uma vez; depois `NAVEGADOR=chromium npx playwright test` (o Chromium do Playwright, o mesmo da CI). O Edge do computador continua para o dia a dia.
- **Teste novo no navegador:** rode com `--repeat-each=5`, nos dois projetos (computador e celular). Teste que passa uma vez e falha na quinta é instável e vai falhar na CI.
- **Falhou só na CI:** baixe o artefato (`gh run download <run> -n testes-navegador`) e olhe o `test-failed-1.png` e o `error-context.md` antes de mexer. Não suba "tentativas" para ver se passa.
- No celular, o toque pode rolar a página: componente que fecha sozinho ao rolar precisa aguentar a rolagem do próprio toque (o calendário fechava antes de aparecer).
- **Medida em px tem folga de 0,5:** com as fontes do Linux da CI, uma caixa de 24 px no CSS mede 23,99997 (#271). Alvo mínimo se confere com `toBeGreaterThanOrEqual(23.5)`, nunca `(24)`; igualdade de tamanho, com `toBeLessThan(2)` na diferença.
- **Não leia o que vai mudar antes de esperar a mudança:** depois de um clique que troca o conteúdo, espere o sinal da troca (`await expect(titulo).not.toHaveText(antes)`) antes de ler; na CI, mais lenta, a leitura pegava o estado anterior (#271, calendário).

**Limites de tempo (a CI confere):**
- **Jest:** cada arquivo de teste até **10 s** (passou disso, a CI avisa) e no máximo **20 s** (passou, falha); a suíte inteira até **150 s** (`scripts/ci/verificar-tempo-testes.py`). Referência hoje: ~34 s a suíte, 6 s o arquivo mais lento, 0,7 s a mediana (2 workers, como a CI).
- **Navegador:** cada teste até 30 s (padrão do Playwright); a suíte hoje leva ~30 s.
- **Go** (#371, `scripts/ci/verificar-tempo-testes-go.py`, sobre o `go test -race -json` do job "Go"):
  - **teste:** aviso a partir de **5 s** e falha a partir de **20 s**;
  - **pacote:** até **60 s**;
  - **suíte:** até **120 s**, do primeiro ao último teste, com os pacotes em paralelo;
  - **referência** (24/10/2026, CI, com `-race`): ~22 s a suíte; `banco` 20 s e `autenticacao` 17 s.
  - **Causas comuns de teste lento:**
    - `time.Sleep` para esperar: use um sinal ou o `testing/synctest`;
    - banco recriado em cada subteste: um banco por teste, com os subtestes em cima dele quando não gravam;
    - rede de verdade: proibida (ADR 0051).
  - **No resumo do job:** os 10 testes mais lentos.

**Importação pesada é a causa nº 1 de teste lento.** Um arquivo de teste importa a tela, que importa os componentes dela, que importam bibliotecas. Biblioteca grande (ícones, gráficos, editores…) carregada no Jest custa em todo arquivo que encosta nela:
- **antes de pôr uma dependência nova no front-end**, rode um teste que a use e veja o tempo (`npx jest testes/<arquivo>.spec.ts`);
- **se pesar**, troque-a nos testes do Jest por um substituto leve em `testes/apoio/`, ligado no `moduleNameMapper` do `jest.config.js`, como o `lucide-falso.js` (ícones). O build e os testes no navegador continuam com a de verdade.

**Como medir quando algo ficou lento:**
1. `npx jest --ci --json --outputFile=jest-resultado.json` e `python3 ../../scripts/ci/verificar-tempo-testes.py jest-resultado.json`: os arquivos mais lentos;
2. **um teste vazio** mostra o custo fixo do ambiente (hoje ~3 s, contando a partida do Jest);
3. **importar peça por peça** num teste temporário (`await import(...)` com o tempo de cada um) acha a importação que pesa. Foi assim que o #204 achou os 15 s do `@lucide/angular`.

**Workflows da CI:**
- **no PR, só os testes afetados** ([ADR 0047](docs/adr/0047-ci-paralela-so-testes-afetados-no-pr.md)): `scripts/ci/testes-afetados.py` liga cada arquivo mudado às telas que o usam (grafo de imports) e roda só os testes delas; arquivo global (`styles.css`, `core/`, `layout/`, rotas, configuração, apoio dos testes, a própria CI) roda tudo. No push do `develop`/`main`, sempre tudo. O porquê aparece no resumo da execução. Para ver aqui o que rodaria: `python3 scripts/ci/testes-afetados.py --base origin/develop`;
- **teste no navegador novo abre a tela pela rota escrita** (`page.goto('/transacoes')`), senão a seleção não o acha (ele ainda roda no merge);
- **arquivo novo que vale para todas as telas** fora das pastas já marcadas como globais: acrescente-o em `GLOBAIS` no `testes-afetados.py`;
- **o .msi não roda no PR, só no merge para o develop, na tag e à mão** ([ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)): PR que mexe no instalador, no gerenciador ou na inicialização da API roda o workflow "Versão (instalador .msi)" à mão na branch antes do merge;
- jobs em paralelo (API, Jest, navegador em 2 partes): o tempo do PR é o do mais lento, não a soma. **O job final `build` junta os quatro e é o check exigido pela proteção do `develop`/`main`:** não troque o nome dele; job novo no `ci.yml` entra no `needs` dele;
- todo workflow de PR tem `concurrency` com `cancel-in-progress`;
- filtros `paths:` por pasta (`scripts/geral/**`), não por nome de arquivo, para um arquivo novo também disparar;
- cache do que é baixado sempre igual (npm, NuGet, navegador do Playwright), menos quando baixar do zero é justamente o que se testa (os testes de instalação);
- **mexeu em teste ou em CI? Ponha no PR o tempo antes e depois**, por job e pelos passos mais caros.

## Testes do back-end (Go)

O C# chegou a 128 rotas sem nenhum teste automatizado e continua assim: para ele, vale o checklist do PR. **No Go, toda regra nasce com teste**, além do caso de contrato ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)). Um comportamento é testado num lugar só, na camada mais barata que o alcança:

| O que testar | Onde | Como |
|---|---|---|
| Regra de negócio, cálculo, conversão, validação | `*_test.go` no pacote do assunto (`internal/<assunto>`) | tabela de casos com `t.Run` |
| Consulta SQL, transação, cascata | o mesmo pacote | SQLite **de verdade** num arquivo do `t.TempDir()`, com o esquema real; sem mock de banco e sem `:memory:` |
| Handler: status, validação, erro | o mesmo pacote | `httptest.NewRequestWithContext` + `httptest.NewRecorder`, com o usuário por `sessao.ComUsuario` |
| O mesmo JSON e o mesmo banco do .NET | `testes/contrato` (#72) | casos gravados do .NET, antes do código Go |
| Pluggy, Yahoo/brapi, BCB, Mercado Bitcoin | o pacote do cliente | `httptest.NewServer` com respostas gravadas em `testdata/` |

O banco temporário, a fábrica de dados inventados, o relógio parado em Brasília e a chamada do handler com o usuário logado já existem: use o `internal/testeapoio` ([README](src/backend-go/internal/testeapoio/README.md)), só em arquivos `_test.go`. Tabela nova que o assunto precisa vira um método da `Fabrica`.

**Uma etapa do #70 está pronta quando:**
- cada regra da lista "Regras de negócio" do levantamento (skill `migrar-para-go`, seção 2) tem pelo menos uma linha na tabela de um teste unitário, e o PR traz a tabela **regra → teste**;
- cada ponto de risco de equivalência do levantamento tem um teste;
- cada endpoint tem caso de contrato, inclusive os de erro;
- cada consulta que grava, apaga em cascata, filtra texto ou ordena tem teste com o SQLite real;
- os casos-limite que se aplicam estão na tabela (o PR diz quais não se aplicam):
  - valor negativo, zero, 8 casas, meio centavo;
  - `NULL` e lista vazia (`[]`, nunca `null`);
  - meia-noite em Brasília × UTC, horário de verão antigo;
  - virada de mês e de ano, dias 29, 30 e 31;
  - texto com acento e maiúsculas.

**Teste novo prova que falha:** estrague o código de propósito (troque `RoundBank` por `Round`, tire um filtro, devolva `nil` no lugar de `[]`), veja o teste falhar e desfaça. Liste as mutações no PR, com o teste que pegou cada uma, como no #360. **Bug corrigido** ganha teste de regressão, com o número da issue no comentário, que falha antes da correção. **Bug do C# achado no porte** não se corrige em silêncio: o teste fixa o comportamento de hoje e aponta a issue aberta.

**Teste de mutação semanal (#373):** o workflow "Teste de mutação (Go)" roda o gremlins toda segunda nos pacotes de `scripts/ci/mutacao-go.txt`, e falha quando um pacote fica abaixo do piso dele.
- **Assunto novo com regra de negócio ou dinheiro:** entra nessa lista no PR dele, com o piso da primeira medição: `python3 scripts/ci/mutacao-go.py internal/<assunto>`, uns 2 min.
- **Mutação viva:** vira teste. Se ela não muda o resultado (é equivalente), vai para `scripts/ci/mutacao-go-equivalentes.txt` com o porquê.
- **Ponto cego:** o gremlins não testa mutação dentro de `case`. Ali, a mutação à mão no PR continua valendo.

**Proibido em teste:**
- `time.Now()`: use o `relogio.Fixo`;
- `time.Sleep` para esperar algo: espere um sinal (canal, `sync.WaitGroup`) ou use o `testing/synctest` em código com temporizador. Um teto (`time.After` num `select`) pode;
- rede de verdade: respostas gravadas. O próprio teste ouvindo em `127.0.0.1` pode;
- dado real: o repositório é compartilhado. Resposta gravada de API externa entra com os valores trocados à mão;
- biblioteca de asserção ou de mock (`testify`, `gomock`, `go-sqlmock`): só a biblioteca padrão.

**`t.Parallel()`:**
- **use** nos testes de banco e de HTTP que têm o próprio banco no próprio `t.TempDir()`;
- **não use** com `t.Setenv` ou `t.Chdir` (o Go recusa), variável de pacote, banco dividido, tempo real ou processo filho, nem no contrato (casos em ordem, no mesmo banco).

**Como escrever:**
- mensagem com a função, o obtido e o esperado (`Hoje = 2026-10-16, esperado 2026-10-15`);
- nome do caso em português, dizendo a regra;
- `t.Error` para continuar, `t.Fatal` só na preparação;
- `t.Helper()` nos ajudantes e `t.Context()` no lugar de `context.Background()`.

**Fuzz** ([Go Fuzzing](https://go.dev/doc/security/fuzz/), #372), nos tipos que todo assunto usa: `internal/tipos` e `internal/relogio`.
- **O que confere:** o Go gera as entradas, e o alvo confere o que vale para qualquer uma: ida e volta pelo banco e pelo JSON, a escala do decimal, o formato do EF Core e o dia de Brasília.
- **Sementes:** os valores do gabarito do EF Core.
- **Achado:** a entrada que achou bug fica em `testdata/fuzz/<alvo>/` e roda em todo `go test`, como regressão. Comite o arquivo junto com a correção.
- **Rodar:**

  ```bash
  go test ./internal/tipos -run '^$' -fuzz '^FuzzDecimalNoBanco$' -fuzztime 5m
  ```

  Um alvo por vez: o `-fuzz` não aceita mais de um.
- **Tipo novo** em `internal/tipos` ganha um alvo `FuzzXxx`, com as sementes do gabarito.

**Gabarito** (*golden file*) fica em `testdata/`, vem da referência (o .NET ou o EF Core) e é regravado só por opção explícita (`-atualizar`), olhando a diferença no PR.

**Antes de subir:** em `src/backend-go`, `gofmt -l .`, `go vet ./...` e `go test -race ./...` (o resto está no [README do Go](src/backend-go/README.md#antes-de-subir-push)). A CI confere também a trava de tempo (acima, em "Testes rápidos") e a cobertura com piso por pacote (abaixo). A mutação automática (#373) entra pelo épico #374.

**Cobertura** (#370): o piso de cada pacote fica em `src/backend-go/cobertura-minima.txt` e **só sobe**.
- **O que a CI confere** (`scripts/ci/verificar-cobertura-go.py`):
  - pacote abaixo do piso falha, mostrando as funções que nenhum teste roda;
  - pacote novo precisa de 80%;
  - descer um piso falha, comparando com o arquivo do `develop`;
  - subiu a cobertura 1 ponto ou mais: a CI avisa para subir o piso no mesmo PR.
- **"fora"**, com o motivo na linha, é só para código gerado.
- **Ver localmente:** em `src/backend-go`,

  ```bash
  go test -coverpkg=./internal/... -coverprofile=c.out ./...
  go tool cover -html=c.out
  ```

  O `go tool cover -func=c.out` dá a lista por função.
- **Atenção:** cobertura mede linha executada, não regra conferida. Ela não substitui a tabela regra → teste nem as mutações.

## Checklist do pull request

- [ ] Versão subiu no `Directory.Build.props`, pelo tipo do PR ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md)).
- [ ] Compila: `dotnet build FinancasLocal.sln -c Release` e `npm run build` em `src/frontend`.
- [ ] Testes no navegador passam: `npm run test:navegador` em `src/frontend` (a CI também roda; [ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)). Tela nova no menu entra em `TELAS` (`testes-navegador/base.ts`); endpoint novo, em `testes-navegador/api-falsa.ts`. Mudou o visual? Olhe os prints (`FOTOS=rota npx playwright test fotos`).
- [ ] Testes do front-end passam: `npm run test:tipos` e `npm test` em `src/frontend` (a CI roda os dois; o `npm test` sozinho não confere tipos). Bug corrigido ganha teste em `testes/regressao/`; padrão novo de UI/UX, em `testes/usabilidade/` ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)).
- [ ] Back-end em Go: regra nova com teste unitário, consulta nova com teste no SQLite real, endpoint novo com caso de contrato, e as mutações listadas no PR ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md), "Testes do back-end (Go)" acima).
- [ ] Mudança de banco? Tabela/índice novo com `CREATE TABLE/INDEX IF NOT EXISTS` em `banco/01-estrutura.sql`, ou coluna nova com `adicionar_coluna_se_ausente` (de `scripts/geral/comum.sh`) chamado em `scripts/geral/2-criar-banco.sh` (veja [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md)) — e entidades/`FinancasDbContext` ajustados. Sem migrations.
- [ ] Decisão de arquitetura (dependência ou integração nova, banco, segurança, infraestrutura, padrão de código)? ADR criada ou atualizada em [docs/adr/](docs/adr/README.md), no mesmo PR.
- [ ] Consultas ao banco só com LINQ/lambda sobre entidades mapeadas (sem SQL montado em código).
- [ ] Nada de dado pessoal, senha, chave ou `appsettings.Production.json` no commit.
- [ ] README/CHANGELOG atualizados quando a mudança afeta o uso. No `CHANGELOG.md` (#294), a entrada vai na seção da **versão do seu PR** (a do `Directory.Build.props`): `## [X.Y.Z] — AAAA-MM-DD · desenvolvimento`, criada no topo se ainda não existe, e **dentro** do título do tipo que já existe nela (`### Funcionalidades`, `### Correções`, `### Publicação e documentação`…). Outro PR subiu a versão antes? Depois do `git merge origin/develop`, a sua entrada muda para a seção da versão nova. A CI confere (`scripts/ci/verificar-changelog.py`: tipo repetido, versão repetida ou fora de ordem, versão acima da do `Directory.Build.props`; `--corrigir` junta tipos repetidos), porque as notas das versões e a janela "O que mudou" do gerenciador saem dele.
- [ ] **Manual do site** (`src/frontend/src/app/paginas/manual-conteudo.ts`) atualizado quando muda algo que o usuário vê (tela, fluxo, cálculo, texto). Se não precisa, escreva "Manual: não se aplica" na descrição do PR. A CI avisa (job "manual do site") quando o PR mexe em telas ou endpoints sem mexer no Manual.
- [ ] Mudou o jeito de usar uma tela (botão, campo, filtro, passo)? Atualize a seção do **Manual** em `src/frontend/src/app/paginas/manual-conteudo.ts`; tela nova no menu entra também em `AJUDA_POR_ROTA` ([ADR 0026](docs/adr/0026-manual-do-site-como-dados-no-front-end.md)).

## Claude Code no projeto

Quem usa o Claude Code no projeto recebe, pelo repositório:
- **as skills de `.claude/skills/`**: `testar-no-navegador` e `trabalho-longo`;
- **o hook de `.claude/settings.json`** (#280). A cada pedido, ele roda `.claude/hooks/lembrar-trabalho-longo.sh` (bash, sem dependência nova; no Windows, pelo Git Bash). Quando o pedido tem link de issue ou PR, cita um épico ou a sessão fica longa, ele lembra o Claude de oferecer a skill `trabalho-longo`, para o trabalho continuar sozinho quando o limite de uso voltar.
  - O hook nunca bloqueia o pedido.
  - Ele avisa uma vez por motivo e por sessão.
  - Para desligar, defina `FINANCAS_SEM_LEMBRETE_TRABALHO_LONGO=1` no ambiente.

## Convenções de código

- Nomes em português, seguindo o que já existe (ex.: `XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Back-end: um serviço por assunto em `Servicos/`, controllers finos, DTOs em `Dtos/Dtos.cs`.
- Front-end: componentes standalone, signals e control flow (`@if`, `@for`); chamadas HTTP só pelo `ApiService`.
- Formatação definida no `.editorconfig`.
