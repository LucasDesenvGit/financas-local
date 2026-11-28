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
# ... trabalho e commits; um arquivo em changelog.d/ para feat e fix (ver "CHANGELOG", abaixo) ...
git push
gh pr create --base develop --fill    # na descrição: Closes #<número>
```

Não há issue? Crie uma antes: todo PR fecha a issue dele. Quem fica atribuído (os épicos ficam sem dono), a etiqueta "em progresso" e quando uma dependência bloqueia estão no [CLAUDE.md, "Git / commits"](CLAUDE.md#git--commits).

**Dependência entre issues** vai no painel **Relationships**, para ninguém pegar uma issue ainda bloqueada. O `<id>` é o id numérico da issue, não o número dela:

```bash
gh api repos/LucasDesenvGit/financas-local/issues/<N> -q .id                                  # o id de uma issue
gh api -X POST repos/LucasDesenvGit/financas-local/issues/<bloqueada>/dependencies/blocked_by -F issue_id=<id da que bloqueia>
gh api -X POST repos/LucasDesenvGit/financas-local/issues/<épico>/sub_issues -F sub_issue_id=<id da história>   # ligar ao épico
```

### Backlog no Project

As issues e o andamento ficam no Project **"ClaudioFinancas"** ([users/LucasDesenvGit/projects/2](https://github.com/users/LucasDesenvGit/projects/2)). Issue nova (e sub-issue) entra sozinha, pelo workflow "Auto-add to project", com o Status `Backlog`.
- **Campos:** **Status** (Backlog, In progress, In review, Done, Epic), **Priority** (P0–P2) e **Size** (XS–XL). Na prática, a prioridade vai na etiqueta `prioridade: alta|média|baixa`.
- **Pelo `gh`** (a conta precisa do escopo `project`): `gh project item-list 2 --owner LucasDesenvGit`, `gh project field-list 2 --owner LucasDesenvGit` e `gh project item-edit`.

### Versão e CHANGELOG

**Nenhum PR sobe a versão** ([ADR 0070](docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)). O número é calculado no build: parte da `<VersaoBase>` do `Directory.Build.props` e sobe a cada merge na linha principal do `develop`, pelo **tipo do título do PR**. Por isso o título no tipo certo é o que importa (um `fix` que era `feat` sai com o número errado, e depois do merge só a base conserta):

| Tipo do PR | Sobe | Exemplo |
|---|---|---|
| `feat` (inclusive `feat(banco)!` que só acrescenta tabela ou coluna) | o número do meio | 2.2.0 → 2.3.0 |
| `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style`, ou título fora do padrão | o último | 2.2.0 → 2.2.1 |
| Quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior) | a `<VersaoBase>` vira `X+1.0.0` no PR, **depois de perguntar ao usuário** | 2.2.0 → 3.0.0 |

Os merges que trazem a `main` ou uma `release/*` de volta não sobem. Para saber a versão de um commit: `go run ./cmd/versao` em `src/backend` (`-ref <commit>` para outro). Sem o histórico do git (uma cópia `.zip`), a versão sai como a base com o sufixo `-sem-git`.

**CHANGELOG:** um arquivo por PR em `changelog.d/`, obrigatório em `feat` e `fix`; o formato e as regras estão no [`changelog.d/LEIA-ME.md`](changelog.d/LEIA-ME.md).

### Publicar uma versão

Para publicar uma versão ([ADR 0029](docs/adr/0029-instalador-msi-com-wix-gerado-pela-ci.md)):
- **Antes da versão estável:** regere os prints do Manual (`cd src/frontend && npm run prints:manual`, ou o workflow manual, quando existir) e inclua o que mudou no PR da versão, para ver o antes e o depois ([ADR 0071](docs/adr/0071-manual-do-site-com-prints-gerados-das-telas.md)). Print novo: uma linha na tabela de `testes/navegador/prints-manual.spec.ts` e `print: { id, alt }` na seção.
1. Numa branch `release/X.Y.Z`, criada do `develop` (o `X.Y.Z` é o que o `go run ./cmd/versao` dá no commit do `develop` de onde ela sai), rode `scripts/dev/lancar-versao.sh X.Y.Z`. Ele confere a branch, a árvore limpa e o número, junta os arquivos de `changelog.d/` numa seção `## [X.Y.Z] — data` do `CHANGELOG.md`, por tipo (uma seção por versão publicada), apaga os arquivos e faz o commit `chore: versão X.Y.Z`. Sem nenhum arquivo em `changelog.d/`, ele recusa: escreva uma entrada antes. As notas do Release juntam a seção da versão e as seções antigas "· desenvolvimento" abaixo dela, até a última publicada, e cortam no limite de 125 mil caracteres do GitHub, com um link para o resto (#641).
2. PR de `release/X.Y.Z` para `main` e merge. Antes, confira que `main` e `develop` continuam protegidas contra apagar: em 2026-10-18, o merge de um PR para a `main` apagou o `develop`.
   Para conferir, `gh api repos/LucasDesenvGit/financas-local/branches/develop/protection` (precisa ser dono do repositório). Para reaplicar, em `main` e em `develop` (eram do `configurar-github.ps1`, que saiu na #655):
   ```bash
   for b in main develop; do
     gh api -X PUT repos/LucasDesenvGit/financas-local/branches/$b/protection --input - <<'JSON'
   {"required_status_checks": {"strict": true, "contexts": ["build"]},
    "enforce_admins": false,
    "required_pull_request_reviews": {"required_approving_review_count": 0, "dismiss_stale_reviews": true},
    "restrictions": null, "allow_force_pushes": false, "allow_deletions": false,
    "required_conversation_resolution": true}
   JSON
   done
   ```
   O `required_conversation_resolution` faz o merge esperar que toda conversa da revisão esteja resolvida.
3. Tag na `main`: `git tag v2.2.0; git push --tags`. A tag tem de ser igual ao número calculado no commit da release (o segundo pai do merge na `main`), senão o workflow para.
4. O workflow **Versão (instalador .msi)**:
   - gera o `.msi`;
   - testa instalar, atualizar, desinstalar e migrar;
   - publica `FinancasLocal-2.2.0.msi` e `SHA256SUMS.txt` no Release **"Finanças Local 2.2.0"**, com "Como instalar ou atualizar" e, do CHANGELOG, a seção da versão (com as PRs sem arquivo em `changelog.d/` pelo título, em "Outras mudanças", #757) e as de desenvolvimento abaixo dela até a última publicada. Se alguém criou o Release à mão antes, o título e as notas são acertados.
5. O gerenciador de quem já tem o site instalado passa a oferecer "Atualizar para 2.2.0".
6. **Traga a `main` de volta para o `develop`:** PR de `main` para `develop` e merge. Esse merge não sobe a versão, porque o conteúdo já foi contado (ADR 0070). Assim, o `CHANGELOG` do `develop` fica igual ao publicado e a `changelog.d/` esvazia: na 2.17.0, isso não foi feito, e o `develop` ficou sem a seção dela.

Quem instala pelos scripts continua atualizando com `git pull` e `scripts/geral/instalar.sh` (no Windows, ele repassa para o `instalar-windows.ps1`, o mesmo do `instalar-windows.cmd`). Só o `3-publicar-atualizar.sh` não basta quando o `go.mod` pede um Go mais novo: quem o baixa é a etapa 1 ([Atualizar](docs/INSTALACAO-PELO-CODIGO.md#atualizar)).

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
- `feat(banco)!: nova coluna Transacao.Emergencia` (o `!` marca que a estrutura do banco mudou; não sobe o primeiro número da versão nem pede nada de quem atualiza, porque a API aplica o esquema sozinha ao iniciar, [ADR 0070](docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md))

## O que a CI roda

Além do build e dos testes do back-end e do front-end, a CI confere o que dá para rodar aqui também:

```bash
python3 scripts/ci/banco/verificar-sql.py                           # banco/01-estrutura.sql é idempotente (e é aplicado a um SQLite descartável)
python3 scripts/dev/gerar-diagrama-banco.py --conferir         # os diagramas de docs/BANCO-DE-DADOS.md estão em dia
python3 scripts/ci/versao/verificar-changelog.py                     # changelog.d/ no formato; no PR, o arquivo em feat e fix (ADR 0070)
```

| Workflow ou job | Quando | O que faz |
|---|---|---|
| **CI › o que mudou** | todo PR e push | decide quais jobs rodam: no PR, pelo que ele mudou (ADR 0047); no push do `develop`/`main`, todos |
| **CI › API e banco** | todo PR e push | o site (Go) ligando de verdade, como o serviço (`scripts/ci/ligar-api.sh`), `verificar-sql.py`, os diagramas do banco e o cache dos arquivos do build |
| **CI › front-end: Jest** e **navegador** (2 partes) | PR que mexe no front-end; todo push | Jest e Playwright inteiros (#774; [ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)) |
| **CI › Go** (#231) | PR que mexe no que o Go lê (o Go, o `banco/`, a CSP); todo push | `gofmt`, `go vet`, regras de ouro (#457: `float64`, `time.Now` e `time.Sleep` de teste fora de `scripts/ci/go/regras-go-excecoes.txt`), `go test -race` com cobertura por pacote que só sobe (#370) e trava de tempo (#371), `go generate` em dia, build do Windows, govulncheck ([README do Go](src/backend/README.md)) |
| **CI › Go (Windows)** (#459) | como o job Go | `go vet` e `go test` no `windows-latest`, para os `_windows.go` (DPAPI, serviço) |
| **CI › gerenciador em Go** e **(Linux)** (#418, [ADR 0052](docs/adr/0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)) | PR que mexe em `src/gerenciador-go/`; todo push | `go vet`, `go test`, a compilação e o govulncheck, no Windows (com `gofmt` e o ícone do `.exe`) e no Linux (com GTK e WebKitGTK) |
| **CI › contrato** (#72) | PR que mexe no back-end ou no esquema; todo push | o contrato contra o Go sozinho, sem rede e com a Pluggy falsa (`testes/contrato/rodar.sh`); os esperados são a regressão do Go (ADR 0051) |
| **CI › manual do site e changelog.d** (#177, [ADR 0070](docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)) | todo PR | avisa quando o PR muda telas ou endpoints sem mexer no Manual (não bloqueia); confere o formato de `changelog.d/` e, em PR `feat` e `fix`, que o arquivo existe; roda os testes das notas dos Releases |
| **CI › build** | sempre | junta os resultados (abaixo, em "Testes rápidos") |
| **Teste de mutação (Go)** (#373, #456) | PR que mexe no Go ou no gerenciador em Go, toda segunda e à mão | no PR, os pacotes de `scripts/ci/go/mutacao-go.txt` (e os do gerenciador, em `mutacao-gerenciador-go.txt`) que ele mudou, com piso; no agendamento, as listas inteiras. Um job por grupo (a área do pacote), em paralelo (#673) |
| **Fuzz do Go** (#372) | toda segunda e à mão | o `-fuzz` dos alvos `FuzzXxx`; não roda no PR |
| **CodeQL** (#451, [ADR 0054](docs/adr/0054-analise-de-seguranca-do-codigo-com-o-codeql.md)) | todo PR e push, toda quinta e à mão | segurança do código (Go, TS, Python, workflows); achados em Security › Code scanning, sem bloquear. Na quinta, porque as consultas ganham regras novas mesmo sem o código mudar |
| **Teste de instalação no Linux** | PR e push que mexem em `scripts/geral/`, `scripts/linux/`, `banco/`, `src/gerenciador-go/` ou no `go.mod` do Go | instala do zero num runner limpo e restaura uma cópia, seguindo o [guia de operação](docs/OPERACAO.md) |
| **Teste de instalação no Windows** | PR e push que mexem em `scripts/windows/`, nos `.cmd` da raiz, no `instalar.sh`, no `3-publicar-atualizar.sh`, no `comum.sh`, no `2-criar-banco.sh`, em `banco/` ou no `go.mod` do Go | o mesmo, pelo `instalar-windows.ps1` e pelo repasse do Git Bash |
| **Prints do Manual** ([ADR 0071](docs/adr/0071-manual-do-site-com-prints-gerados-das-telas.md)) | só à mão, na branch da versão estável | regera os prints do Manual no `windows-latest` (`npm run prints:manual`) e os entrega no artefato `prints-manual`, para copiar por cima de `src/frontend/` e commitar |
| **Versão (instalador .msi)** ([ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)) | push do `develop`, tag `vX.Y.Z` e à mão | gera e testa o `.msi` (`instalador/gerar-msi.ps1`, WiX 5, ADR 0029), restaurando uma cópia pelo guia de operação (#527); na tag, publica o Release |

## Testes rápidos (padrões de CI)

Em outubro de 2026 o Jest chegou a levar ~6 min na CI (#204): um pacote pesado importado nos testes custava ~15 s em **cada** arquivo de teste. Para não voltar:

**Onde cada teste mora** (um comportamento testado num lugar só):

| O que testar | Onde | Por quê |
|---|---|---|
| Lógica, cálculo, componente isolado, texto, atributo de acessibilidade | **Jest** (`testes/jest/regressao`, `testes/jest/usabilidade`) | rápido, sem navegador ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)) |
| Desenho, CSS calculado (`var()`, grid, tamanho de alvo), rota de verdade, CSP, console, celular | **Navegador** (`testes/navegador/`) | o jsdom não calcula estilo nem aplica a CSP ([ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)) |

Não repita no Jest o que o navegador já confere, e vice-versa.

**Antes de subir (push) mudança no front-end:** `npx playwright install chromium` uma vez; depois `cd src/frontend && NAVEGADOR=chromium npm run test:afetados`, com o Chromium do Playwright, o mesmo da CI (#264: passou no Edge daqui e falhou na CI), só no que mudou (#286). Não rode a suíte inteira a cada envio: ela roda na CI, no PR. O Edge do computador continua para o dia a dia.
- **O que ele roda** (`scripts/dev/testes-afetados.py`, ADR 0047), contando também o que ainda não foi commitado:
  - o Jest inteiro (~8 s);
  - o build;
  - os testes no navegador das telas afetadas (as que importam o arquivo mudado). Mudou arquivo global (estilos, `core/`, configuração, a lista `GLOBAIS` do script): o navegador inteiro.
- **O front-end não mudou:** não roda nada.
- **Tempo (22/11/2026):** numa mudança de uma tela, ~16 s, contra ~55 s com o navegador inteiro. A CI roda tudo no PR (#774).
- **Teste novo no navegador abre a tela pela rota escrita** (`page.goto('/transacoes')`), senão a seleção não o acha (a CI o roda de qualquer jeito).
- **O Python 3 é achado sozinho:** `python` no Windows, `python3` no Linux.

- **Estilo calculado se testa no navegador**, em `testes/navegador/estilos.spec.ts`: layout (alturas, colunas, largura), borda, grid/subgrid, `var()`, quem vence na cascata, animação, "reduzir movimento", tamanho de alvo. **Não escreva teste do Jest que procura a regra no `styles.css`** (`cssRules`, ordem das regras no arquivo): ele quebra quando a regra é reescrita com o mesmo efeito e passa quando outra regra a anula. Propriedade simples que o jsdom calcula (`width`, `min-height`, `visibility`) pode ficar no Jest.
- **Meça o efeito, não a regra:** "os três cards têm a mesma altura" (`boundingBox`), não "existe `align-items: stretch`".
- **Teste novo de estilo prova que falha:** estrague a regra no CSS do build (`dist/…/styles-*.css`), rode o teste, veja falhar e restaure. Se não falhar, os dados da API falsa provavelmente não exercitam o caso (ex.: dois cards com o mesmo conteúdo ficam iguais mesmo sem a regra): use conteúdos de tamanhos diferentes.

**Pegadinhas do navegador:**
- **Teste novo no navegador:** rode com `--repeat-each=5`, nos dois projetos (computador e celular). Teste que passa uma vez e falha na quinta é instável e vai falhar na CI.
- **Falhou só na CI:** baixe o artefato (`gh run download <run> -n testes-navegador`) e olhe o `test-failed-1.png` e o `error-context.md` antes de mexer. Não suba "tentativas" para ver se passa.
- No celular, o toque pode rolar a página: componente que fecha sozinho ao rolar precisa aguentar a rolagem do próprio toque (o calendário fechava antes de aparecer).
- **Medida em px tem folga de 0,5:** com as fontes do Linux da CI, uma caixa de 24 px no CSS mede 23,99997 (#271). Alvo mínimo se confere com `toBeGreaterThanOrEqual(23.5)`, nunca `(24)`; igualdade de tamanho, com `toBeLessThan(2)` na diferença.
- **Não leia o que vai mudar antes de esperar a mudança:** depois de um clique que troca o conteúdo, espere o sinal da troca (`await expect(titulo).not.toHaveText(antes)`) antes de ler; na CI, mais lenta, a leitura pegava o estado anterior (#271, calendário).

**Limites de tempo (a CI confere):**
- **Jest:** cada arquivo de teste até **10 s** (passou disso, a CI avisa) e no máximo **20 s** (passou, falha); a suíte inteira até **150 s** (`scripts/ci/front/verificar-tempo-testes.py`). Referência hoje: ~34 s a suíte, 6 s o arquivo mais lento, 0,7 s a mediana (2 workers, como a CI).
- **Navegador:** cada teste até 30 s (padrão do Playwright); a suíte hoje leva ~30 s.
- **Go** (#371, `scripts/ci/go/verificar-tempo-testes-go.py`, sobre o `go test -race -json` do job "Go"):
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
- **antes de pôr uma dependência nova no front-end**, rode um teste que a use e veja o tempo (`npx jest testes/jest/<pasta>/<arquivo>.spec.ts`);
- **se pesar**, troque-a nos testes do Jest por um substituto leve em `testes/jest/apoio/`, ligado no `moduleNameMapper` do `jest.config.js`, como o `lucide-falso.js` (ícones). O build e os testes no navegador continuam com a de verdade.

**Como medir quando algo ficou lento:**
1. `npx jest --ci --json --outputFile=jest-resultado.json` e `python3 ../../scripts/ci/front/verificar-tempo-testes.py jest-resultado.json`: os arquivos mais lentos;
2. **um teste vazio** mostra o custo fixo do ambiente (hoje ~3 s, contando a partida do Jest);
3. **importar peça por peça** num teste temporário (`await import(...)` com o tempo de cada um) acha a importação que pesa. Foi assim que o #204 achou os 15 s do `@lucide/angular`.

**Workflows da CI** (quais jobs rodam no PR: a tabela de "O que a CI roda"):
- **o .msi não roda no PR, só no merge para o develop, na tag e à mão** ([ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)): PR que mexe em `instalador/`, `src/gerenciador-go/`, `src/backend/cmd/financas/` ou `src/backend/internal/plataforma/banco/esquema_banco.go` roda o workflow "Versão (instalador .msi)" à mão na branch antes do merge, e diz no PR que rodou;
- jobs em paralelo (API, Jest, navegador em 2 partes): o tempo do PR é o do mais lento, não a soma. **O job final `build` junta os resultados e é o check exigido pela proteção do `develop`/`main`:** não troque o nome dele; todo job novo no `ci.yml` entra no `needs` dele;
- todo workflow de PR tem `concurrency` com `cancel-in-progress`;
- filtros `paths:` por pasta (`scripts/geral/**`), não por nome de arquivo, para um arquivo novo também disparar;
- cache do que é baixado sempre igual (npm, NuGet, navegador do Playwright), menos quando baixar do zero é justamente o que se testa (os testes de instalação);
- **mexeu em teste ou em CI? Ponha no PR o tempo antes e depois**, por job e pelos passos mais caros.

## Testes do back-end (Go)

**No Go, toda regra nasce com teste**, além do caso de contrato ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)). Um comportamento é testado num lugar só, na camada mais barata que o alcança:

| O que testar | Onde | Como |
|---|---|---|
| Regra de negócio, cálculo, conversão, validação | `*_test.go` no pacote do assunto (`internal/<area>/<assunto>`) | tabela de casos com `t.Run` |
| Consulta SQL, transação, cascata | o mesmo pacote | SQLite **de verdade** num arquivo do `t.TempDir()`, com o esquema real; sem mock de banco e sem `:memory:` |
| Handler: status, validação, erro | o mesmo pacote | `httptest.NewRequestWithContext` + `httptest.NewRecorder`, com o usuário por `sessao.ComUsuario` |
| O mesmo JSON e o mesmo banco de antes | `testes/contrato` (#72) | casos gravados do .NET até a #86; hoje, a regressão do Go (`rodar.sh --atualizar`, com o `git diff` conferido; abaixo) |
| Pluggy, Yahoo, BCB, Binance | o pacote do cliente | o cliente recebe a URL base e o `http.Client` pelo construtor, e o teste usa o `testeapoio.ServicoDeFora` (#460): um `httptest.NewTLSServer` com as rotas cadastradas, que responde com arquivos de `testdata/` ou com os erros prontos (tempo esgotado, 401, 429, 500, JSON quebrado, página vazia), confere os cabeçalhos e guarda os pedidos, com os cabeçalhos de segredo trocados por `<segredo>`. Pedido sem rota cadastrada é erro do teste |
| O que está num `_windows.go` (DPAPI, serviço do Windows, registro do gerenciador) | um `_windows_test.go` do mesmo pacote | o `go` só o compila no Windows ([build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)); roda nos jobs "Go (Windows)" (#459) e "gerenciador em Go", sem o `-race` (no Windows ele pede cgo). O que grava no registro usa uma chave só do teste, apagada no `t.Cleanup`. Fica de fora o que pede administrador (UAC, instalar serviço), com o porquê no teste |

O banco temporário, a fábrica de dados inventados, o relógio parado em Brasília e a chamada do handler com o usuário logado já existem: use o `internal/plataforma/testeapoio` ([README](src/backend/internal/plataforma/testeapoio/README.md)), só em arquivos `_test.go`. Tabela nova que o assunto precisa vira um método da `Fabrica`. Os pacotes que o `testeapoio` importa (`banco`, `relogio`, `tipos`, `sessao`) não podem importá-lo nos próprios testes (seria ciclo) e usam um ajudante local, como o `bancoDeTeste`.

**Não repita** num lugar o que outro já confere: o contrato prova a equivalência com pelo menos um caso de sucesso e um por status de erro; o unitário cobre as variações da regra; o HTTP, cada ramo de validação.

**Contrato, a regressão do Go** (#86, [ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md#atualização-de-2026-11-03-o-contrato-vira-a-regressão-do-go-issue-86-parte-3)):
- `testes/contrato/rodar.sh` sobe o Go sozinho, com a linha do serviço instalado (`-aplicar-esquema -rotinas-de-partida`), o relógio de teste, a Pluggy falsa e sem rede (`unshare -rn` no Linux; senão, o proxy numa porta fechada). Roda no Linux e no Git Bash do Windows; o job da CI é o "contrato". Detalhes no [README do contrato](src/backend/testes/contrato/README.md);
- **um esperado que já existe muda só de propósito**, com o motivo de cada um no PR (bug corrigido ou mudança pedida). Diferença que aparece sem querer é regressão: o código volta, e o esperado fica;
- **caso novo** é gravado com `rodar.sh --atualizar`, e o `git diff` de `esperado/` tem de mostrar só ele. Endpoint novo: caso de contrato, inclusive os de erro (400, 401, 404, 409), mais o teste HTTP dos ramos de validação.

**Uma regra nova do Go está pronta quando:**
- cada endpoint tem caso de contrato, inclusive os de erro;
- cada consulta que grava, apaga em cascata, filtra texto ou ordena tem teste com o SQLite real;
- os casos-limite que se aplicam estão na tabela (o PR diz quais não se aplicam):
  - valor negativo, zero, 8 casas, meio centavo (`0,125`: o `Math.Round` do C# arredonda para o par, [Microsoft Learn](https://learn.microsoft.com/dotnet/api/system.math.round); em Go, o `RoundBank`);
  - `NULL` no banco e no JSON, e lista vazia (`[]`, nunca `null`);
  - meia-noite em Brasília × UTC (23h30 em Brasília já é o dia seguinte em UTC), horário de verão antigo (até 2019);
  - virada de mês e de ano, dias 29, 30 e 31, inclusive fevereiro de ano bissexto e o "mesmo dia no mês seguinte" a partir do dia 31;
  - texto com acento e maiúsculas ("Ação" × "acao"), porque o `LIKE` do SQLite só ignora a caixa em ASCII ([sqlite.org](https://www.sqlite.org/lang_expr.html#the_like_glob_regexp_match_and_extract_operators));
- `gofmt`, `go vet` e `go test -race` limpos, dentro dos pisos de cobertura e de tempo.

A tabela regra → teste traz o nome do teste e do caso (*inferido: o formato*).

**Teste novo prova que falha:** estrague o **código** (não o teste) de propósito (troque `RoundBank` por `Round`, tire um filtro, inverta uma comparação, devolva `nil` no lugar de `[]`), veja o teste falhar e desfaça. Liste as mutações no PR, com o teste que pegou cada uma, como no #360 (12 mutações, todas pegas): pelo menos uma por regra de cálculo (*inferido: a quantidade*). Teste que nenhuma mutação derruba é refeito; em geral, os dados dele não exercitam o caso. **Bug corrigido** ganha teste de regressão, com o número da issue no comentário, que falha antes da correção. **Bug do C# achado no porte** não se corrige em silêncio: o teste fixa o comportamento de hoje e aponta a issue aberta, e muda junto quando ela for decidida.

**Teste de mutação (#373, #456):** o workflow "Teste de mutação (Go)" roda o [gremlins](https://github.com/go-gremlins/gremlins) v0.6.0 (`go run …@v0.6.0`, fora do `go.mod`, numa cópia do módulo) nos pacotes de `scripts/ci/go/mutacao-go.txt`: no PR, só nos que o PR mudou (código, teste ou `testdata/` deles, comparando com a base como o `testes-afetados.py`); toda segunda e à mão, em todos. Falha quando um pacote fica abaixo do piso dele.
- **Como mede:** todos os operadores ligados, 4 workers e folga de 30× no tempo de cada teste (no padrão de 5×, a compilação em paralelo estourava). Eficácia = mortas / (mortas + vivas), com piso por pacote que só sobe. Se o PR só muda a própria mutação (script, listas, workflow), mede o `internal/plataforma/tipos`, para provar que ela ainda roda. *Julgamento:* medir também quem importa o pacote mudado pegaria mais, mas multiplicaria o tempo (~1 min por pacote).
- **Nenhum pacote fica de fora sem ninguém perceber:** todo pacote de `internal/` com código fora de teste está em `mutacao-go.txt` ou em `scripts/ci/go/mutacao-go-fora.txt`, com o motivo (código gerado, só montagem de rotas, apoio a teste). O script confere antes de medir. **Pacote novo, que não está em nenhuma das duas, não derruba a CI** (#716): é medido sem piso, e um aviso dá a linha a pôr em `mutacao-go.txt` (com a eficácia medida, arredondada para baixo) ou manda pô-lo em `mutacao-go-fora.txt` com o motivo. Continuam sendo erro o pacote nas duas listas, a linha de pacote que não existe e o de fora sem o motivo. O teste do script é o `scripts/ci/go/test_mutacao_go.py`.
- **Um job por grupo (#673):** o grupo é a área do pacote (`internal/<area>/<assunto>`; sem área, "raiz"), e cada um roda em paralelo, com 30 min de limite (a lista inteira num job só passou dos 60 min). Um job escolhe os grupos (`mutacao-go.py --grupos`) e a matriz mede cada um (`--grupo`); no PR, só os grupos dos pacotes mudados. Pacote novo entra no grupo da área sozinho. Um grupo passou dos 30 min? Dê ao pacote mais lento dele um grupo próprio, com o nome na terceira coluna de `mutacao-go.txt` (como o `carteira`). *Julgamento:* grupo pela área, e não por tempo medido, que ficaria velho; o custo é compilar o gremlins e o módulo de novo em cada grupo (~1 min).
- **Assunto novo com regra de negócio ou dinheiro:** entra nessa lista no PR dele, com o piso da primeira medição: `python3 scripts/ci/go/mutacao-go.py internal/<area>/<assunto>`, uns 2 min.
- **Mutação viva:** vira teste. Se ela não muda o resultado (é equivalente, como `100 * time.Nanosecond` por `100 / time.Nanosecond`), vai para `scripts/ci/go/mutacao-go-equivalentes.txt`, pelo texto da linha, com o porquê. Só entra o que foi conferido à mão.
- **Ponto cego:** a cobertura do Go conta comandos, e a condição de um `case` não é comando: a mutação ali sai como "NOT COVERED" e o gremlins nunca a testa. No nosso estilo (`switch { case …: }`), isso cobre boa parte das regras, e a mutação à mão no PR é o que vale.

**Proibido em teste:**
- `time.Now()`, no código testado ou para montar dado: use o `relogio.Fixo` (o código de regra recebe um `relogio.Relogio`). Exceção: medir o próprio tempo real, quando ele é o assunto (o ajudante de WAL, `internal/plataforma/banco/wal_test.go`), escrita em `regras-go-excecoes.txt`;
- `time.Sleep` para esperar algo: espere um sinal (canal, `sync.WaitGroup`, a linha `{"porta":N}` lida da saída, como em `cmd/financas/main_test.go`) ou use o [`testing/synctest`](https://pkg.go.dev/testing/synctest) em código com temporizador (relógio falso que adianta quando todos estão parados; [blog do Go](https://go.dev/blog/testing-time)). Um teto (`time.After` num `select`, ou um contexto com prazo) pode;
- rede de verdade: o `testeapoio.ServicoDeFora` com as respostas de `testdata/`. O próprio teste ouvindo em `127.0.0.1` pode;
- dado real: o repositório é compartilhado. Resposta gravada de API externa entra com os valores trocados à mão (nunca token, conta, documento ou saldo de verdade). Exceção: os testes manuais que leem uma **cópia** do banco (`FINANCAS_COPIA_BANCO`): só leem, não mostram valores, pulam sozinhos sem a variável e nada deles vai para o repositório;
- biblioteca de asserção ou de mock (`testify`, `gomock`, `go-sqlmock`): só a biblioteca padrão, com `if obtido != esperado { t.Errorf(...) }` e um *fake* pequeno escrito à mão onde há interface.

**A CI confere as regras de ouro (#457):** `python3 ../../scripts/ci/go/verificar-regras-go.py`, rodado em `src/backend` (o mesmo do job "Go", antes do `go test`), falha em `float64` e `float32` (e `Float64()`, `ParseFloat`, `FormatFloat`) em qualquer `.go`, em `time.Now` (com ou sem parênteses) fora do `internal/plataforma/relogio` e em `time.Sleep` de `_test.go`; dentro de um `synctest.Test`, onde o relógio é de mentira, `time.Now` e `time.Sleep` passam. A mensagem diz o que usar no lugar (o `relogio`, o `tipos.Decimal` ou o sinal).
- **Só o código conta:** comentário e texto entre aspas não entram. A conferência é por texto: um `import t "time"` escapa, e `time.Since`/`time.Until` não são conferidos.
- **Exceção:** uma linha em `scripts/ci/go/regras-go-excecoes.txt` (regra, arquivo, texto da linha do código e o motivo), como o `mutacao-go-equivalentes.txt`: vale pelo texto, não pelo número da linha. Exceção que não casa mais com nenhuma linha também falha. Só entra o que foi conferido à mão.
- **O que pode ser exceção:** `float64` só para o que o C# também calcula em double e que não é dinheiro (na primeira conferência, 1/11/2026, nenhum era: o `[Range(0.01d, ...)]` das validações, o fator da Selic, a pontuação das sugestões de categoria); `time.Now` só para hora de sistema (processo, duração, validade de certificado, limite por IP); `time.Sleep` só quando o tempo é o que se testa (o ajudante de WAL segura a trava do banco por 5 ms para quem grava junto encontrar o banco ocupado).
- **O teste do script** (`scripts/ci/go/test_verificar_regras_go.py`, só `unittest`) roda antes dele no mesmo job.

**`t.Parallel()`:**
- **use** nos testes de banco e de HTTP que têm o próprio banco no próprio `t.TempDir()`: é onde está o tempo. Chame no começo do teste e de cada subteste da tabela. O teste paralelo espera os não paralelos do pacote terminarem ([`T.Parallel`](https://pkg.go.dev/testing#T.Parallel)), e desde o Go 1.22 cada volta do `for` tem a própria variável ([blog do Go](https://go.dev/blog/loopvar-preview));
- **não use** com `t.Setenv` ou `t.Chdir` (afetam o processo inteiro, e o Go recusa), variável de pacote, banco dividido, tempo real ou processo filho (o teste de WAL), nem no contrato (casos em ordem, no mesmo banco, um usando o que o anterior capturou);
- **unitário puro em tabela:** pode, mas não precisa; ele leva microssegundos (*julgamento*);
- o `go test ./...` já roda pacotes diferentes ao mesmo tempo, em processos separados (`-p`): por isso **nenhum teste usa um caminho de banco fixo**, sempre o `t.TempDir()`. E o `-race` só acha a corrida que acontece de fato: teste paralelo sobre código com goroutine ajuda a corrida a aparecer (*julgamento*).

**Como escrever:**
- mensagem com a função, a entrada, o obtido e o esperado, nessa ordem (`Hoje = 2026-10-16, esperado 2026-10-15`; [Got before want](https://go.dev/wiki/TestComments#got-before-want));
- nome do caso em português, uma frase que diz a regra ("meia-noite em Brasília", "virada do ano");
- `t.Error` para continuar, `t.Fatal` só quando o resto não faz sentido, como uma preparação que falhou;
- `t.Helper()` nos ajudantes e `t.Context()` (cancelado antes dos `t.Cleanup`) no lugar de `context.Background()`;
- teste que precisa de algo de fora (uma cópia do banco) pula com `t.Skip` e uma mensagem que diz o que definir, e **nunca é o único teste de uma regra**.

**Fuzz** ([Go Fuzzing](https://go.dev/doc/security/fuzz/), #372), nos tipos que todo assunto usa: `internal/plataforma/tipos` e `internal/plataforma/relogio`.
- **O que confere:** o Go gera as entradas, e o alvo confere o que vale para qualquer uma: ida e volta pelo banco e pelo JSON, a escala do decimal, o formato do EF Core e o dia de Brasília.
- **Sementes:** os valores do gabarito do EF Core.
- **Achado:** a entrada que achou bug fica em `testdata/fuzz/<alvo>/` e roda em todo `go test`, como regressão. Comite o arquivo junto com a correção. A primeira rodada achou 2 bugs: o começo do dia no horário de verão (que também lança exceção no C#, #393) e o GUID com hífen fora do lugar.
- **Onde roda:** o `-fuzz` não roda no PR, só no workflow "Fuzz do Go", toda segunda e à mão. *Julgamento:* o achado do fuzz quase nunca é culpa do PR aberto, e travá-lo por isso atrasaria um trabalho sem relação.
- **Rodar:**

  ```bash
  go test ./internal/plataforma/tipos -run '^$' -fuzz '^FuzzDecimalNoBanco$' -fuzztime 5m
  ```

  Um alvo por vez: o `-fuzz` não aceita mais de um.
- **Tipo novo** em `internal/plataforma/tipos` ganha um alvo `FuzzXxx`, com as sementes do gabarito.

**Gabarito** (*golden file*) fica em `testdata/`, que o `go` ignora. **Ele vem da referência, nunca do código que ele testa**, senão o teste só confirma o próprio Go. Os que vieram do .NET ou do EF Core (pelas sondas, que saíram na #86) não são mais regravados: são a regressão. Os outros são regravados só por opção explícita (`-atualizar`, como o `-update` da biblioteca padrão), revisados linha a linha no PR que os grava.

**Teste de arquitetura (#476, só local):** quem pode importar quem fica em `internal/arquitetura/arquitetura_test.go`, no estilo do ArchUnit, com só a biblioteca padrão (o grafo vem do `go list -json ./...`; `arch-go`, `go-arch-lint` e `depguard` seriam dependência nova). A build tag `arquitetura` o tira do `go test ./...` e da CI (decisão de 29/10/2026): rode `go test -tags arquitetura ./internal/arquitetura/` antes de subir um PR que mexe em import ou pacote novo. As regras, cada uma com o motivo e as exceções:
1. só o `internal/plataforma/banco` (e o contrato) importam o driver do SQLite;
2. o `testeapoio` só entra em `_test.go`;
3. pacote de regra (a lista `regraSemHTTP`) não chega a `net/http`, ao `respostas` nem ao `servidor`, nem por outro pacote;
4. todo pacote está num grupo (base, transversal, montador ou assunto), e um assunto só importa outro se o par estiver na tabela `dependencias` (a semente do mapa de contextos, #474).

Exceção que deixou de acontecer, pacote que sumiu e pacote novo sem grupo também fazem o teste falhar, com o que fazer. O teste se testa: um grafo de mentira com um import proibido faz cada regra falhar.

**Antes de subir:** os comandos do [README do Go](src/backend/README.md#antes-de-subir-push), em `src/backend`.

**Cobertura** (#370): o piso de cada pacote fica em `src/backend/cobertura-minima.txt` e **só sobe**.
- **O que a CI confere** (`scripts/ci/go/verificar-cobertura-go.py`):
  - pacote abaixo do piso falha, mostrando as funções que nenhum teste roda;
  - pacote novo precisa de 80%;
  - descer um piso falha, comparando com o arquivo do `develop`;
  - subiu a cobertura 1 ponto ou mais: a CI avisa para subir o piso no mesmo PR.
- **Como mede:** `go test -race -coverpkg=./internal/...`, em que cada pacote conta as linhas que qualquer teste roda. Os pisos começaram no valor de 24/10/2026, medido na CI; os 80% de pacote novo são *julgamento* (o que a maioria dos pacotes escritos com teste desde o começo já tinha).
- **"fora"**, com o motivo na linha, é só para código gerado (o gerador do `go generate`).
- **Subir o piso é de quem subiu a cobertura**, no mesmo PR: ela cai sozinha quando entra código sem teste num pacote antigo, mas não sobe sozinha.
- **Ver localmente:** em `src/backend`,

  ```bash
  go test -coverpkg=./internal/... -coverprofile=c.out ./...
  go tool cover -html=c.out
  ```

  O `go tool cover -func=c.out` dá a lista por função.
- **Atenção:** cobertura mede linha executada, não regra conferida. Ela não substitui a tabela regra → teste nem as mutações.

**Benchmark** só para código quente, antes de otimizar (skill `go-no-projeto`, "Desempenho").

## Claude Code no projeto

Quem usa o Claude Code no projeto recebe, pelo repositório:
- **as skills de `.claude/skills/`**: `angular-no-projeto`, `testar-no-navegador` e `revisar-no-celular` (front-end), `go-no-projeto` (Go), `reportar-erro` e `trabalho-longo`;
- **os agentes de `.claude/agents/`**: `dev-angular` (#559) e `dev-go` (#560), que carregam as skills do assunto e têm um hook só deles (`.claude/hooks/bloquear-merge-em-agente.sh`) que barra `gh pr merge` e `git push` forçado;
- **o hook de `.claude/settings.json`** (#280). A cada pedido, ele roda `.claude/hooks/lembrar-trabalho-longo.sh` (bash, sem dependência nova; no Windows, pelo Git Bash). Quando o pedido tem link de issue ou PR, cita um épico ou a sessão fica longa, ele lembra o Claude de oferecer a skill `trabalho-longo`, para o trabalho continuar sozinho quando o limite de uso voltar.
  - O hook nunca bloqueia o pedido.
  - Ele avisa uma vez por motivo e por sessão.
  - Para desligar, defina `FINANCAS_SEM_LEMBRETE_TRABALHO_LONGO=1` no ambiente.

## Convenções de código

As que valem em toda tarefa (nomes em português, consultas parametrizadas, dinheiro, Manual, e o `Eager` e o `ApiService` no front-end) estão no `CLAUDE.md`, em ["Convenções de código"](CLAUDE.md#convenções-de-código) e ["Arquitetura"](CLAUDE.md#arquitetura-detalhes-em-docsarquiteturamd); o Go em detalhe, na skill `go-no-projeto`, e o Angular, na `angular-no-projeto`. O checklist do PR, só com o que a CI não confere, está no [modelo de PR](.github/pull_request_template.md).
