# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Idioma

**Responda sempre em português do Brasil**, para qualquer pessoa da equipe: respostas, perguntas, resumos de fim de tarefa, descrições de PR e de issue, mensagens de commit e comentários no código. Vale também quando o conteúdo técnico (log, erro, documentação) vier em inglês e depois de a conversa ser resumida. Nomes de comandos, arquivos e APIs ficam como são.

## O que é o projeto

Sistema de gerenciamento financeiro pessoal que roda inteiro localmente, em Windows ou Linux com os mesmos scripts bash: API em C#/.NET 10 hospedada via Kestrel (sem IIS — `UseWindowsService`/`UseSystemd` em `Program.cs`), front-end Angular 22, banco SQLite local (`dados/sql/FinancasLocal.db`). Dados bancários chegam via Open Finance através do **Meu Pluggy**. Veja [README.md](README.md) para a visão funcional completa (painel, transações, orçamentos, metas, investimentos, relatórios) e [docs/ARQUITETURA.md](docs/ARQUITETURA.md) para o diagrama de camadas. O porquê das decisões está nas ADRs em [docs/adr/](docs/adr/README.md).

## Comandos

```bash
# Build
dotnet build FinancasLocal.sln -c Release      # back-end (Financas.Api + Financas.MigrarSqlite)
cd src/frontend && npm run build                # front-end (produção)
cd src/frontend && npm test                     # testes do front-end (Jest): test:regressao | test:usabilidade | test:tipos
cd src/frontend && npm run test:navegador       # testes no navegador (Playwright): build de produção + CSP da API + API falsa (ADR 0041)
cd src/frontend && npm run test:afetados        # antes do push: Jest + navegador só no que mudou desde o develop (#286, mesma escolha da CI)
cd src/frontend && npm start                    # front-end em dev (http://localhost:4200, proxy /api -> API)

# CI roda também:
python3 scripts/ci/verificar-sql.py             # confere que banco/01-estrutura.sql é idempotente
python3 scripts/ci/gerar-diagrama-banco.py --conferir   # confere que os diagramas de docs/BANCO-DE-DADOS.md estão em dia
python3 scripts/ci/testes-afetados.py --base origin/develop   # no PR, a CI roda só os testes afetados (ADR 0047): mostra quais
# Job "Go" (#231): gofmt, go vet, go test -race (com cobertura por pacote que só sobe, #370, e trava de tempo, #371), go generate em dia, build Windows, govulncheck (src/backend-go/README.md)
# Workflow "Fuzz do Go" (#372): o -fuzz dos alvos FuzzXxx, toda segunda e à mão; não roda no PR
# Workflow "CodeQL" (#451, ADR 0054): segurança do código (C#, Go, TS, Python, workflows) em todo PR; achados em Security › Code scanning, sem bloquear
# Workflows "Teste de instalação no Linux/Windows": instalam do zero num runner limpo quando scripts/ ou banco/ mudam

# Banco (sem migrations)
scripts/geral/2-criar-banco.sh                  # roda os scripts de banco/ em ordem (colunas novas: helper adicionar_coluna_se_ausente)

# Publicação
scripts/geral/3-publicar-atualizar.sh           # compila front+API, publica, cuida de certificado e do serviço do sistema
scripts/geral/instalar.sh                       # etapas 1-3 de uma vez (bash)
instalar-windows.cmd                            # Windows sem bash: eleva e roda scripts/windows/instalar-windows.ps1 (etapas 1-3 em PowerShell 5.1, ADR 0012)
instalador/gerar-msi.ps1                        # gera o FinancasLocal-<versão>.msi (WiX 5; a CI faz isso no workflow "Versão" e publica na tag vX.Y.Z; ADR 0029)

# Migrar dados de uma instalação antiga (SQL Server/IIS) para o SQLite novo — seguro rodar mesmo sem SQL Server disponível
dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho .db destino>"
```

**O instalador `.msi` não é gerado no PR, só no merge para o develop** (#344, [ADR 0050](docs/adr/0050-msi-gerado-no-merge-e-nao-no-pr.md)): o workflow "Versão (instalador .msi)" roda no push do `develop`, na tag `vX.Y.Z` e à mão. PR que mexe em `instalador/`, `src/gerenciador/`, `Program.cs` ou `EsquemaBanco.cs`: rode o workflow à mão na branch (Actions → Run workflow) antes do merge e diga no PR que rodou. Mudança só de CI (workflows, `scripts/ci/`) continua num PR `ci:` ou `ci(escopo):` (#307, ADR 0039); não misture produto nele.

**Testes rápidos (#204):** cada comportamento testado num lugar só: lógica no Jest, desenho/CSS/rota/CSP no navegador. A CI falha com arquivo do Jest acima de 20 s ou suíte acima de 150 s (`scripts/ci/verificar-tempo-testes.py`). Biblioteca pesada no front-end vira substituto leve nos testes do Jest (`testes/apoio/`, `moduleNameMapper`, como o `lucide-falso.js`); antes de pôr uma dependência nova, meça um teste que a use. Mexeu em teste ou em CI: tempo antes e depois no PR. Detalhes em [CONTRIBUINDO.md](CONTRIBUINDO.md#testes-rápidos-padrões-de-ci).

**Testes no navegador** (Playwright, [ADR 0041](docs/adr/0041-testes-no-navegador-com-playwright.md)), em `src/frontend/testes-navegador/`: o build de produção com a CSP da API e a API falsa (`api-falsa.ts`, sem dado real); cada tela do menu abre sem erro de console, exceção ou bloqueio de CSP, em computador e celular. **Mudou tela, CSS, rota ou chamada da API no front-end? Use a skill `testar-no-navegador`** (`.claude/skills/`): roda a regressão, tira prints das telas mudadas para olhar e diz o que foi conferido. Tela nova no menu entra em `TELAS` (`base.ts`), e endpoint novo, na API falsa.

**Antes de subir (push) mudança no front-end, rode os testes no navegador com o mesmo navegador da CI** (#264: passou no Edge daqui e falhou na CI):
- `npx playwright install chromium` uma vez; depois **`NAVEGADOR=chromium npm run test:afetados`** (#286), com o Chromium do Playwright, o mesmo da CI.
  - Ele roda o Jest, o build e o navegador **só no que o PR afeta**, inclusive o que não foi commitado, com a mesma escolha da CI (ADR 0047). Arquivo global roda tudo.
  - Numa tela só: ~45 s, contra ~2,7 min da suíte inteira, que continua rodando na CI, no merge.
  - Não rode a suíte inteira a cada envio. O Edge do computador continua para o dia a dia.
- **Teste novo no navegador:** rode com `--repeat-each=5`, nos dois projetos (computador e celular). Teste que passa uma vez e falha na quinta é instável e vai falhar na CI.
- **Falhou só na CI:** baixe o artefato (`gh run download <run> -n testes-navegador`) e olhe o `test-failed-1.png` e o `error-context.md` antes de mexer. Não suba "tentativas" para ver se passa.
- No celular, o toque pode rolar a página: componente que fecha sozinho ao rolar precisa aguentar a rolagem do próprio toque (o calendário fechava antes de aparecer).
- **Medida em px tem folga de 0,5:** com as fontes do Linux da CI, uma caixa de 24 px no CSS mede 23,99997 (#271). Alvo mínimo se confere com `toBeGreaterThanOrEqual(23.5)`, nunca `(24)`; igualdade de tamanho, com `toBeLessThan(2)` na diferença.
- **Não leia o que vai mudar antes de esperar a mudança:** depois de um clique que troca o conteúdo, espere o sinal da troca (`await expect(titulo).not.toHaveText(antes)`) antes de ler; na CI, mais lenta, a leitura pegava o estado anterior (#271, calendário).

Testes do front-end com Jest + `jest-preset-angular` em `src/frontend/testes/` ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)): `regressao/` (bug corrigido ou comportamento garantido por um PR — cada bug corrigido ganha teste aqui) e `usabilidade/` (critérios de UI/UX desta página verificáveis sem navegador). A CI roda `npm run test:tipos` e `npm test`.

**Back-end em Go: toda regra nasce com teste** ([ADR 0051](docs/adr/0051-testes-do-back-end-em-go-cada-regra-com-teste.md)): unitário em tabela no pacote do assunto, além do caso de contrato do endpoint; consulta num SQLite real em arquivo temporário (sem mock), handler com `httptest`, só a biblioteca padrão. Sem `time.Now()` (use `relogio.Fixo`), `time.Sleep` para esperar, rede ou dado real. Bug corrigido ganha teste que falha antes; teste novo prova que falha estragando o código. O C# continua sem testes: vale o checklist manual do PR (ver [CONTRIBUINDO.md](CONTRIBUINDO.md)).

Em desenvolvimento no Visual Studio, o perfil *Desenvolvimento (Kestrel)* sobe a API em `https://localhost:7443`; as chaves do Data Protection ficam em `dados\chaves-dev` (Client Secret da Pluggy precisa ser reinformado).

## Arquitetura (resumo — detalhes em docs/ARQUITETURA.md)

Angular é publicado dentro do `wwwroot` da própria API: mesma origem, sem CORS, cookies `__Host-`.

**Back-end** (`src/backend/Financas.Api`):
- `Controllers/` — endpoints finos (`BaseController` padroniza erros); um controller pode cobrir múltiplos recursos relacionados (ex.: `PlanejamentoControllers.cs` reúne orçamentos/metas/contas a pagar).
- `Servicos/` — regra de negócio: `RelatorioService` (painel/orçamentos/projeção/visão anual), `CategorizacaoService` (regras automáticas), `SugestaoCategoriaService`, `ContaService`, `ConfiguracaoService`, `AuditoriaService`, `Relogio` (hora sempre em fuso de Brasília).
- `Servicos/Pluggy/` — cliente HTTP próprio (sem SDK) para Open Finance; `SincronizacaoHostedService` agenda `SincronizacaoService` periodicamente (endpoint `/v2/transactions`, paginação por cursor).
- `Servicos/Investimentos/` — carteira de renda variável: `CarteiraService`, `CotacaoService` (Yahoo Finance/brapi), `SelicService` (api.bcb.gov.br), `FiiService`, `ExtratoCarteiraService` (importa compras/vendas a partir da descrição de transações), `ConferenciaInvestimentosService` ("Conferir investimentos": tipo FII, cotas que faltam, troco × provento; ADR 0027).
- `Entidades/` — classes mapeadas no EF Core (`Financeiro.cs`, `Carteira.cs`, `OpenFinance.cs`, `Seguranca.cs`, `Enums.cs`).
- `Data/FinancasDbContext.cs` — único lugar com nomes/tamanhos/precisão/índices das tabelas; deve ficar em sincronia manual com `banco/01-estrutura.sql`.
- `Dtos/Dtos.cs` — todos os DTOs num arquivo único.
- `Seguranca/` — cabeçalhos de segurança, validação de sessão, constantes de cookies.

**Gerenciador** (`src/gerenciador/`): situação do site, ligar/desligar, cópia do banco. `Financas.Gerenciador.Nucleo` tem o que vale nos dois sistemas (instalação, situação, serviço, cópia do banco); `Financas.Gerenciador` é a janela do Windows (WinForms, UAC, versão nova pelo GitHub Releases; ADR 0028), instalada pelo `.msi` ou pelo `scripts/windows/instalar-gerenciador.ps1`; `Financas.Gerenciador.Linux` é a do Linux (Avalonia, `systemctl --user`, sem rede; ADR 0033), pelo `scripts/linux/instalar-gerenciador.sh`. Texto ou ação nova vai nas duas janelas.

**Front-end** (`src/frontend/src/app`):
- `core/` — `ApiService` (único ponto de chamadas HTTP), modelos TS, autenticação, tema, privacidade (ocultar valores).
- `compartilhado/` — componentes reutilizáveis (gráfico Chart.js, modal, paginação, calculadora flutuante, pipe de moeda).
- `layout/` — casca da tela (menu lateral).
- `paginas/` — uma página por rota.
- Componentes standalone, signals e control flow (`@if`/`@for`), com `changeDetection: ChangeDetectionStrategy.Eager`: desde o Angular 22 o padrão é OnPush, mas o app roda com zone.js e muitas telas guardam estado em campos comuns (não signals) — componente novo sem `Eager` pode não atualizar a tela.

**Banco** (`banco/`) — **sem EF Core Migrations**, de propósito, em SQLite:
- `01-estrutura.sql` — tabelas/índices/restrições com `CREATE TABLE/INDEX IF NOT EXISTS` (idempotência nativa do SQLite); baseline limpa da troca SQL Server → SQLite, com tabelas novas acrescentadas no fim — verificado pelo CI via `scripts/ci/verificar-sql.py` e aplicado de fato a um banco SQLite descartável no CI. Nunca apagar/renomear coluna em uso sem bloco de transição.
- `02-dados-iniciais.sql` — categorias/config/regras padrão.
- A API também aplica esses scripts ao iniciar (`Data/EsquemaBanco.cs`, embutidos no build; cópia em `dados/backups` antes de mudar um banco existente; dados iniciais só no banco novo) — é o que a instalação pelo .msi usa.
- Coluna nova numa tabela existente: SQLite não tem `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, então essa idempotência fica no bash: chamar `adicionar_coluna_se_ausente` (de `scripts/geral/comum.sh`, checa via `PRAGMA table_info`) em `scripts/geral/2-criar-banco.sh`, não no SQL — uma chamada por linha, argumentos entre aspas duplas, porque o `instalar-windows.ps1` e a API (`EsquemaBanco`) leem essas linhas.
- **Pastas dos scripts (#154):** `scripts/geral/` é o bash que roda nos dois sistemas (no Windows, pelo Git Bash); `scripts/windows/` e `scripts/linux/` têm o que só funciona num deles; `scripts/ci/`, o que só a CI usa. Script novo vai na pasta do sistema em que roda. Os `.cmd` ficam na raiz.
- Mudou `1-preparar-ambiente.sh` ou `3-publicar-atualizar.sh`? Confira se `scripts/windows/instalar-windows.ps1` precisa da mesma mudança (e vice-versa). Serviço, certificado ou pastas de dados também existem no `instalador/configurar.ps1` (.msi, ADR 0029).
- **Dado pessoal novo:** tabela ou coluna com dado pessoal, ou dado novo que sai do computador, entra em [docs/DADOS-PESSOAIS.md](docs/DADOS-PESSOAIS.md) no mesmo PR.
- **Mudou a cópia, o WAL, a pasta `dados/` ou as chaves?** Atualize o [guia de operação](docs/OPERACAO.md) no mesmo PR, e os passos de restaurar dos workflows "Teste de instalação", que seguem o guia (#468).
- Versão do sistema: só no `Directory.Build.props` (API e gerenciador); a tag `vX.Y.Z` tem de ser igual. **Todo PR para o `develop` sobe esse número** (ver Git / commits).
- Ao mudar a estrutura: atualizar `Entidades/` + `FinancasDbContext.cs`, rodar `2-criar-banco.sh`, rodar `python3 scripts/ci/gerar-diagrama-banco.py` (tabela nova entra em `ASSUNTOS`) e usar `feat(banco)!:` no commit. Passos detalhados em [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md).

## Decisões de arquitetura (ADRs) — obrigatório

As decisões ficam em `docs/adr/` (índice, critérios e ciclo de vida em [docs/adr/README.md](docs/adr/README.md); modelo em `docs/adr/0000-modelo.md`). **Antes de implementar** qualquer mudança que adicione ou troque dependência/integração externa (inclusive novo domínio de saída de rede), mude o banco além de uma tabela/coluna rotineira, mexa em segurança, infraestrutura/publicação/CI, ou crie/quebre um padrão de código do projeto:

1. Leia o índice de `docs/adr/README.md` e as ADRs do assunto. Se a mudança contraria uma ADR **Aceita**, avise o usuário antes de seguir — não contorne a decisão em silêncio.
2. **Já existe ADR do assunto:** atualize-a se a mudança só detalha ou ajusta a mesma decisão (consequências, passos práticos). Se a decisão em si mudou, crie uma ADR nova e marque a antiga como `Substituída por NNNN` — nunca apague ADR.
3. **Não existe:** crie `docs/adr/NNNN-titulo-em-kebab-case.md` a partir do modelo, com o próximo número livre, alternativas consideradas e consequências negativas; marque como *inferido* o que não foi confirmado pelo usuário.
4. Atualize o índice em `docs/adr/README.md` e, se mudar o estado atual descrito, `docs/ARQUITETURA.md`.
5. A ADR vai no mesmo commit/PR da mudança. Ao final, diga ao usuário qual ADR foi criada ou atualizada (ou por que a mudança não precisou de uma).

Não precisa de ADR: bugfix, refatoração local, ajuste visual, tela ou endpoint que só segue padrões já registrados.

**Requisitos não funcionais** ([docs/REQUISITOS-NAO-FUNCIONAIS.md](docs/REQUISITOS-NAO-FUNCIONAIS.md)): mudança que mexe numa meta (memória, tamanho, partida) mede antes e depois no PR, como já se faz com o tempo dos testes.

**Diagramas sempre junto** ([ADR 0055](docs/adr/0055-documentacao-de-arquitetura-em-diagramas-mermaid.md)): mudou um contêiner, uma saída de rede ou uma fronteira? Atualize os diagramas de Contexto, de Contêineres e de implantação em `docs/ARQUITETURA.md` **no mesmo PR**, como o Manual (#177). Termo novo do domínio entra na lista de [`docs/arquitetura/dominio.md`](docs/arquitetura/dominio.md), e estado novo (enum, `Status`) atualiza o diagrama de estados de lá.

## Convenções de código

- **Manual do site sempre junto (#177):** toda mudança que o usuário vê (tela nova ou alterada, fluxo, cálculo que muda um número, texto de botão) atualiza o Manual (`src/frontend/src/app/paginas/manual-conteudo.ts`) **no mesmo PR**: a seção da tela, os passos e os termos. Se a mudança não aparece para o usuário (refatoração, bug interno), escrever "Manual: não se aplica" na descrição do PR. A CI (job "manual do site") avisa quando falta. O teste `manual.spec.ts` já exige uma seção por tela.
- Nomes em português em todo o código (`XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Consultas ao banco **só via LINQ/lambda** sobre entidades mapeadas (nunca SQL montado a partir de texto do usuário); atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
- **Código Go** (`src/backend-go/`, migração do épico #70, [ADR 0049](docs/adr/0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)):
  - **Bibliotecas:** só as da ADR (`net/http`, `modernc.org/sqlite`, `sqlc`, `shopspring/decimal`, `log/slog`, `x/sys/windows/svc`, `go-pkcs12`, `x/text/unicode/norm` e `x/text/collate`); outra dependência atualiza a ADR.
  - **Consultas:** SQL só pelo código do `sqlc` ou com parâmetros `?`, **nunca** texto do usuário concatenado.
  - **Dinheiro:** nunca `float64`, nem `SUM`/`ORDER BY`/`<`/`>` no SQL em coluna de dinheiro (é `TEXT`); somar e comparar em Go, com decimal.
  - **Banco:** grava no formato do EF Core (decimal, data, GUID em maiúsculas), porque .NET e Go convivem no mesmo banco.
  - **Estilo:** nomes em português, erro como valor (`fmt.Errorf("...: %w", err)`), `context.Context` primeiro, dependências pelo construtor, relógio injetável no fuso de Brasília; `gofmt`, `go vet` e `go test -race` limpos.
  - **Troca aos poucos:** o .NET fica na frente e repassa ao Go os prefixos migrados (#231). O C# de um assunto só é apagado depois de uma versão publicada com ele no Go, e só o que nenhum outro C# usa.
  - **Funcionalidade nova fora do núcleo** (tabelas e rotas próprias) **é feita em Go, atrás do .NET**: prefixo em `Go:Prefixos`, pacote em `src/backend-go/internal/<assunto>`, rotas em `internal/api`. Ajuste em tela ou endpoint que já existe fica onde está (#231).
- Segredos (Client Secret da Pluggy, token da brapi) ficam **criptografados na tabela `Configuracao`** via ASP.NET Data Protection — nunca em arquivo de config nem devolvidos pela API.
- Front-end chama a API só pelo `ApiService` (não usar `HttpClient` direto nos componentes).
- Formatação: `.editorconfig`.

## UI/UX — referências obrigatórias

Ao propor, revisar ou implementar qualquer coisa de UI/UX (telas, navegação, gráficos, filtros, cards de melhoria), basear a recomendação nestas fontes e **citar qual princípio sustenta cada decisão**:

| Fonte | Usar para | Link |
|---|---|---|
| Shneiderman — *"Overview first, zoom and filter, then details-on-demand"* (*The Eyes Have It*, IEEE Symposium on Visual Languages, 1996) | Gráficos, painéis, drill-down do resumo ao detalhe | [doi:10.1109/VL.1996.545307](https://doi.org/10.1109/VL.1996.545307) |
| Nielsen — 10 heurísticas de usabilidade (NN/g) | Avaliação geral de tela; as mais usadas aqui: 1 *Visibility of System Status*, 2 *Match Between the System and the Real World* (sem jargão), 3 *User Control and Freedom*, 4 *Consistency and Standards*, 6 *Recognition Rather than Recall*, 8 *Aesthetic and Minimalist Design*, 10 *Help and Documentation* | [nngroup.com/articles/ten-usability-heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/) |
| Nielsen — *Progressive Disclosure* (NN/g, 2006) | Telas densas: mostrar primeiro o essencial, o especializado atrás de um controle secundário ("Mais análises", abas) | [nngroup.com/articles/progressive-disclosure](https://www.nngroup.com/articles/progressive-disclosure/) |
| WCAG 2.2 — 1.1.1 *Non-text Content* (A) | Todo gráfico (`<canvas>`) tem alternativa em texto: a frase-resumo do bloco serve para isso | [w3.org/WAI/WCAG22/Understanding/non-text-content](https://www.w3.org/WAI/WCAG22/Understanding/non-text-content) |
| WCAG 2.2 — 2.1.1 *Keyboard* (A) | Toda ação clicável funciona por teclado; clique em `<canvas>` (Chart.js) **nunca** é o único caminho, precisa de equivalente em HTML (`<a>`/`<button>`) | [w3.org/WAI/WCAG22/Understanding/keyboard](https://www.w3.org/WAI/WCAG22/Understanding/keyboard) |
| WCAG 2.2 — 1.4.3 *Contrast (Minimum)* (AA) | Texto ≥ 4,5:1 sobre o fundo (3:1 se grande: 18 pt, ou 14 pt em negrito); cores do tema conferidas em `testes/regressao/contraste-tema.spec.ts` | [w3.org/WAI/WCAG22/Understanding/contrast-minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum) |
| WCAG 2.2 — 2.5.8 *Target Size (Minimum)* (AA) | Alvo de clique/toque ≥ 24×24 CSS px (fatias pequenas de gráfico não atendem) | [w3.org/WAI/WCAG22/Understanding/target-size-minimum](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum) |
| WCAG 2.2 — 1.4.13 *Content on Hover or Focus* (AA) | Tooltips: dispensável, alcançável e persistente; em toque não existe hover | [w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus](https://www.w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus) |
| WCAG 2.2 — 1.4.11 *Non-text Contrast* (AA) | Borda e estados de controles (caixa de marcar, interruptor, foco) com contraste ≥ 3:1 com o que está ao lado | [w3.org/WAI/WCAG22/Understanding/non-text-contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast) |
| Kendrick — *Toggle-Switch Guidelines* (NN/g, 2018) | Interruptor só para efeito **imediato**; o que depende de "Salvar" fica caixa de marcar; rótulo curto, palavra-chave primeiro (teste: ler com "ligado/desligado" no fim) | [nngroup.com/articles/toggle-switch-guidelines](https://www.nngroup.com/articles/toggle-switch-guidelines/) |
| WAI-ARIA APG — *Switch Pattern* | Interruptor: `role="switch"` (no checkbox nativo usa o `checked`), Espaço alterna, rótulo **não muda** com o estado | [w3.org/WAI/ARIA/apg/patterns/switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/) |
| WAI-ARIA APG — *Disclosure (Show/Hide) Pattern* | Mostrar e esconder um bloco (grupo do menu, seção que abre): o controle é `<button>`, Enter e Espaço alternam, `aria-expanded` diz o estado e `aria-controls` aponta para o conteúdo | [w3.org/WAI/ARIA/apg/patterns/disclosure](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/) |
| Huei-Hsin Wang — *Accordions on Desktop: When and How to Use* (NN/g, 2023) | Sanfona: deixar várias seções abertas ao mesmo tempo (fechar as outras sozinho atrapalha quem usa mais de uma) | [nngroup.com/articles/accordions-on-desktop](https://www.nngroup.com/articles/accordions-on-desktop/) |
| WCAG 2.2 — 3.3.1 *Error Identification* (A) | Erro de entrada detectado: diz qual campo e o que está errado, em texto | [w3.org/WAI/WCAG22/Understanding/error-identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification) |
| WCAG 2.2 — 3.3.2 *Labels or Instructions* (A) | Todo campo com rótulo visível (o "R$" dentro do campo não substitui o rótulo) | [w3.org/WAI/WCAG22/Understanding/labels-or-instructions](https://www.w3.org/WAI/WCAG22/Understanding/labels-or-instructions) |

Padrões já adotados a partir delas (seguir, salvo motivo registrado):
- **Detalhe por navegação, não modal**, quando já existe tela com o detalhe (ex.: total por categoria → `/transacoes` filtrada), levando o contexto completo (categoria + período + tipo) em query params.
- **Filtro ativo sempre visível** (chips com ✕ e "Limpar filtros") em tela que pode abrir já filtrada.
- **Escolher data ou mês sobe de nível pelo título** (#264, #342): todo quadro de escolher data ou mês tem o título como botão que sobe de nível (dia → mês → ano, em páginas de 12 anos), e Esc volta um nível antes de fechar. Vale para o calendário dos campos de data e para o seletor de mês; componente novo de data segue o mesmo e entra em `testes/usabilidade/escolher-data-sobe-nivel.spec.ts` (Nielsen 4 e 7).
- **Estado de tela navegável na URL**: botão voltar do navegador tem que funcionar; atualização de filtro usa `replaceUrl` para não poluir o histórico.
- **Clique em linha ou bloco é atalho de mouse**: a mesma ação tem `<button>`/`<a>` no template (ex.: o nome da transação abre a edição), conferido em `testes/regressao/clique-com-teclado.spec.ts` (WCAG 2.1.1).
- **Gráfico se explica**: título + frase-resumo com a conclusão + linha "como ler" + legenda quando há mais de uma série; resumo respeita "ocultar valores".
- **Gráfico de linha: a principal preenchida até a base, com pontos; a tracejada sem nada** ([ADR 0021](docs/adr/0021-componentes-padrao-para-graficos-e-telas-densas.md), #292). O `<app-grafico>` faz isso sozinho (`linhasNoPadrao`, em `compartilhado/grafico-linhas.ts`): não use `fill: true`, que preenche até o zero e inverte a área com valor negativo. Para fugir do padrão, o conjunto declara `fill: false` ou `pointRadius`.
- **Barras com linha: a linha por cima, com um ponto por mês** ([ADR 0021](docs/adr/0021-componentes-padrao-para-graficos-e-telas-densas.md), #282). O `<app-grafico>` faz isso sozinho (`linhaPorCimaDasBarras`, em `compartilhado/grafico-misto.ts`): não repita `order`/`pointRadius` na tela. Linha de referência (média, meta) declara `pointRadius: 0`. O valor de cada ponto também vai na tabela do bloco.
- **Termo técnico com definição no contexto** (ⓘ que abre por clique/teclado, vindo de um glossário único) — nunca só em `title=`, que não funciona em toque nem teclado.
- **Tela densa mostra primeiro a resposta à pergunta principal dela**; análises de apoio vão para `<app-mais-analises>` (botão com `aria-expanded` que diz o que tem dentro, conteúdo em `<ng-template>` criado só ao abrir, escolha lembrada por tela). Nada é removido: tudo a no máximo um clique.
- **Interruptor** (`label.interruptor` + `role="switch"` no checkbox nativo) para o que muda a tela na hora; caixa de marcar para o que depende de "Salvar" ([ADR 0032](docs/adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- **Menu lateral em grupos** que abrem e fecham, vários abertos ao mesmo tempo, todos abertos no começo; tela nova entra no grupo do assunto dela em `MENU` ([ADR 0046](docs/adr/0046-menu-lateral-em-grupos-que-abrem-e-fecham.md)).
- **Seleção em tabela:** a célula inteira é o alvo (`label.celula-marcar`), a linha marcada fica destacada (`tr.selecionada`), e o nome da caixa diz qual linha é ([ADR 0032](docs/adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- O porquê e o custo aceito de cada padrão acima estão nas ADRs [0017](docs/adr/0017-preferencias-de-exibicao-no-navegador.md) a [0021](docs/adr/0021-componentes-padrao-para-graficos-e-telas-densas.md).

Ao responder: separar explicitamente o que vem dessas fontes do que é **julgamento próprio** (trade-off sem fonte direta), e dizer quando algo não foi validado com uso real. Fonte nova entra nesta tabela com link verificado, não citada de memória.

## Git / commits

Conventional Commits em português (ver [CONTRIBUINDO.md](CONTRIBUINDO.md) para fluxo de branches completo). Escopos usados: `api`, `web`, `banco`, `scripts`, `open-finance`, `investimentos`, `relatorios`. Branch `main` é protegida (só recebe merge de `develop`/`hotfix/*`); trabalho novo parte de `develop` em `feature/<assunto>` ou `fix/<assunto>`.

**`CHANGELOG.md`: uma seção por versão** (#294). A entrada de um PR vai na seção da versão dele (a do `Directory.Build.props`): `## [X.Y.Z] — AAAA-MM-DD · desenvolvimento`, criada no topo se não existir, **dentro** do `### Funcionalidades`/`### Correções`/`### Publicação e documentação` que já existe nela (nunca outro título de mesmo tipo). Ao trazer o `develop` e subir a versão de novo, **mova a entrada para a seção da versão nova**. Antes do push, rode `python3 scripts/ci/verificar-changelog.py` (a CI roda também; `--corrigir` junta tipos repetidos). As notas das versões e a janela "O que mudou" do gerenciador saem dele: seção errada vira nota errada para quem atualiza.

**Todo PR para o `develop` sobe a versão no `Directory.Build.props`, pelo tipo do título** ([ADR 0039](docs/adr/0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md); o check "versão subiu" da CI confere):
- `feat` (inclusive `feat(banco)!` que só acrescenta tabela ou coluna) → o número do meio (2.2.0 → 2.3.0);
- `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style` → o último (2.2.0 → 2.2.1);
- quebra de verdade (o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior) → o primeiro (3.0.0); pergunte ao usuário antes.
- Outro PR subiu antes? `git merge origin/develop`, pegue a maior e suba de novo. Diga ao usuário, no fim, de qual para qual versão foi.

**Backlog no Project "ClaudioFinancas"** ([users/LucasDesenvGit/projects/2](https://github.com/users/LucasDesenvGit/projects/2)): as issues e o andamento ficam lá. A conta do `gh` tem o escopo `project`, então dá para ler e mexer nele: `gh project item-list 2 --owner LucasDesenvGit`, `gh project field-list 2 --owner LucasDesenvGit` e `gh project item-edit`. Issue nova (e sub-issue) entra sozinha, pelo workflow "Auto-add to project", com o Status `Backlog`. Campos: **Status** (Backlog, In progress, In review, Done, Epic), **Priority** (P0–P2) e **Size** (XS–XL); na prática, a prioridade vai na etiqueta `prioridade: alta|média|baixa`. Consulte o Project quando a conversa for sobre backlog, andamento ou o que falta.

**O branch padrão do repositório é o `develop`** (#142). Por isso, todo PR para o `develop` leva `Closes #N` na descrição: o GitHub vincula o PR à issue e a fecha no merge. Fora do branch padrão, a palavra-chave é ignorada.
- **Branch nova:** de preferência criada pela issue, com `gh issue develop <N> --base develop --name feature/<assunto> --checkout`.
- **Issue nova já nasce no nome de quem pediu:** `gh issue create --assignee @me` (a conta do `gh` é a do usuário) ou o login de quem pediu, se for outra pessoa. **Exceção, os épicos:** veja a regra abaixo.
- **Épico e as histórias dele não são atribuídos a ninguém:** nem ao criar, nem depois, nem ao ligar uma história ao épico (sub-issue). O épico é a demanda inteira, e ninguém "é dono" dela. Só quando uma pessoa **pega uma história**, pedindo aqui, essa história (e só ela) vai para o nome dela, com a etiqueta "em progresso". O épico continua sem ninguém. Se o usuário pedir para não atribuir uma issue a ele, não atribua.
- **Dependência entre issues vai no GitHub, não só no texto:** quando uma issue só pode começar depois de outra estar pronta, marque com *blocked by* (painel **Relationships**), além do "Depende de: #N" no texto. Assim ninguém pega uma issue que ainda está bloqueada. Vale ao criar uma issue, ao montar as histórias de um épico (inclusive entre elas) e ao descobrir uma dependência nova numa issue que já existe.
  - Comando: `gh api -X POST repos/LucasDesenvGit/financas-local/issues/<bloqueada>/dependencies/blocked_by -F issue_id=<id>`. O `<id>` é o id numérico da issue que bloqueia (`gh api repos/LucasDesenvGit/financas-local/issues/<N> -q .id`), não o número dela.
  - Só dependência real: "de preferência antes" não bloqueia. Ligar uma história ao épico (sub-issue, `.../issues/<épico>/sub_issues -F sub_issue_id=<id>`) é outra coisa e não substitui o *blocked by*.
- **Antes de trabalhar numa issue:** atribuir a quem pediu e pôr a etiqueta "em progresso" (`gh issue edit <N> --add-assignee @me --add-label "em progresso"`). Ao fechar, tirar a etiqueta.
- **Não sabe o número da issue:** pergunte ao usuário. Se não houver issue, ofereça criar uma com `gh issue create`.

**Migração do back-end para Go (épico #70):** use a skill `migrar-para-go` (`.claude/skills/`) ou o agente `dev-go` (`.claude/agents/`, para analisar assuntos em paralelo). Eles seguem a ordem das etapas do épico e não começam código Go antes da ADR da #71.

**Pedido grande demais para uma vez só:** ofereça a skill `trabalho-longo` (`.claude/skills/`) em uma frase. Ela põe o plano em arquivo, cria o agendamento que retoma sozinho depois do limite de uso e faz o fechamento. Ofereça quando **qualquer um** destes for verdade, e não só quando o pedido "parecer grande" (#280: numa sessão de muitas issues pedidas uma de cada vez, ela nunca foi oferecida, e o trabalho parou no limite):
- o pedido cita **mais de uma issue ou PR**, ou um **épico**;
- é a **2ª issue ou PR na mesma conversa**, ou a conversa **já foi resumida** (sessão longa);
- a pessoa fala em "deixar rodando", "continuar depois" ou "quando o limite voltar".

Não repita se já houver agendamento (`CronList`) ou se a pessoa recusou nesta conversa. O hook `.claude/hooks/lembrar-trabalho-longo.sh` (em `.claude/settings.json`, vale para todos do projeto) lembra automaticamente quando o pedido tem link de issue/PR ou a sessão fica longa; o lembrete chega como contexto e deve ser seguido.

## Segurança (específico deste projeto — não genérico)

Tudo roda local, sem exposição de rede (bind só em `127.0.0.1:5443`, direto pelo Kestrel — sem IIS/proxy). Pontos que afetam como escrever código aqui:
- Acesso ao banco é só permissão de arquivo do SO sobre `dados/` — SQLite não tem login/usuário de banco; quem lê/grava é o processo da API, rodando como a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows). Sem senha de banco em lugar nenhum.
- Segredos (Client Secret da Pluggy, token da brapi) via ASP.NET Data Protection, chaves em `dados/chaves`: DPAPI no Windows (`OperatingSystem.IsWindows()` em `Program.cs`), permissão de arquivo no Linux.
- CSP restritiva: nada de scripts/recursos de terceiros carregados do front-end.
- **Modelo de ameaças em dia:** uma saída de rede nova, uma fronteira nova (processo, porta, pasta) ou uma mudança no login revisa o [modelo de ameaças](docs/MODELO-DE-AMEACAS.md) no mesmo PR.
- Saídas de rede permitidas e já implementadas: `api.pluggy.ai`, `api.bcb.gov.br`, `olinda.bcb.gov.br` (câmbio PTAX), `query1.finance.yahoo.com`/`brapi.dev`, `api.mercadobitcoin.net` (cripto, ADR 0048) (API) e `api.github.com`/`github.com` (só o gerenciador: o do Windows para versão nova, ADR 0028; e, na instalação de desenvolvimento, o botão "Atualizar para a última do develop" nas duas janelas, ADR 0040) — qualquer integração externa nova deve seguir o mesmo padrão (HTTPS, dado mínimo enviado, ver a tabela em [docs/SEGURANCA.md](docs/SEGURANCA.md#o-que-sai-para-a-internet) e o diagrama "Como funciona" do [README.md](README.md#como-funciona), que mudam junto).
