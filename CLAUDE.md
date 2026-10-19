# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é o projeto

Sistema de gerenciamento financeiro pessoal que roda inteiro localmente, em Windows ou Linux com os mesmos scripts bash: API em C#/.NET 10 hospedada via Kestrel (sem IIS — `UseWindowsService`/`UseSystemd` em `Program.cs`), front-end Angular 22, banco SQLite local (`dados/sql/FinancasLocal.db`). Dados bancários chegam via Open Finance através do **Meu Pluggy**. Veja [README.md](README.md) para a visão funcional completa (painel, transações, orçamentos, metas, investimentos, relatórios) e [docs/ARQUITETURA.md](docs/ARQUITETURA.md) para o diagrama de camadas. O porquê das decisões está nas ADRs em [docs/adr/](docs/adr/README.md).

## Comandos

```bash
# Build
dotnet build FinancasLocal.sln -c Release      # back-end (Financas.Api + Financas.MigrarSqlite)
cd src/frontend && npm run build                # front-end (produção)
cd src/frontend && npm test                     # testes do front-end (Jest): test:regressao | test:usabilidade | test:tipos
cd src/frontend && npm start                    # front-end em dev (http://localhost:4200, proxy /api -> API)

# CI roda também:
python3 scripts/ci/verificar-sql.py             # confere que banco/01-estrutura.sql é idempotente
# Workflows "Teste de instalação no Linux/Windows": instalam do zero num runner limpo quando scripts/ ou banco/ mudam

# Banco (sem migrations)
scripts/2-criar-banco.sh                        # roda os scripts de banco/ em ordem (colunas novas: helper adicionar_coluna_se_ausente)

# Publicação
scripts/3-publicar-atualizar.sh                 # compila front+API, publica, cuida de certificado e do serviço do sistema
scripts/instalar.sh                             # etapas 1-3 de uma vez (bash)
instalar-windows.cmd                            # Windows sem bash: eleva e roda scripts/instalar-windows.ps1 (etapas 1-3 em PowerShell 5.1, ADR 0012)
instalador/gerar-msi.ps1                        # gera o FinancasLocal-<versão>.msi (WiX 5; a CI faz isso no workflow "Versão" e publica na tag vX.Y.Z; ADR 0029)

# Migrar dados de uma instalação antiga (SQL Server/IIS) para o SQLite novo — seguro rodar mesmo sem SQL Server disponível
dotnet run --project src/backend/Financas.MigrarSqlite -- "<connection string SQL Server origem>" "<caminho .db destino>"
```

Testes automatizados só no front-end, com Jest + `jest-preset-angular` em `src/frontend/testes/` ([ADR 0013](docs/adr/0013-testes-do-front-end-com-jest.md)): `regressao/` (bug corrigido ou comportamento garantido por um PR — cada bug corrigido ganha teste aqui) e `usabilidade/` (critérios de UI/UX desta página verificáveis sem navegador). A CI roda `npm run test:tipos` e `npm test`. O back-end não tem testes automatizados; além disso, vale o checklist manual do PR (ver [CONTRIBUINDO.md](CONTRIBUINDO.md)).

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

**Gerenciador** (`src/gerenciador/`): situação do site, ligar/desligar, cópia do banco. `Financas.Gerenciador.Nucleo` tem o que vale nos dois sistemas (instalação, situação, serviço, cópia do banco); `Financas.Gerenciador` é a janela do Windows (WinForms, UAC, versão nova pelo GitHub Releases; ADR 0028), instalada pelo `.msi` ou pelo `scripts/instalar-gerenciador.ps1`; `Financas.Gerenciador.Linux` é a do Linux (Avalonia, `systemctl --user`, sem rede; ADR 0033), pelo `scripts/instalar-gerenciador.sh`. Texto ou ação nova vai nas duas janelas.

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
- Coluna nova numa tabela existente: SQLite não tem `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, então essa idempotência fica no bash: chamar `adicionar_coluna_se_ausente` (de `scripts/comum.sh`, checa via `PRAGMA table_info`) em `scripts/2-criar-banco.sh`, não no SQL — uma chamada por linha, argumentos entre aspas duplas, porque o `instalar-windows.ps1` e a API (`EsquemaBanco`) leem essas linhas.
- Mudou `1-preparar-ambiente.sh` ou `3-publicar-atualizar.sh`? Confira se `scripts/instalar-windows.ps1` precisa da mesma mudança (e vice-versa). Serviço, certificado ou pastas de dados também existem no `instalador/configurar.ps1` (.msi, ADR 0029).
- Versão do sistema: só no `Directory.Build.props` (API e gerenciador); a tag `vX.Y.Z` tem de ser igual.
- Ao mudar a estrutura: atualizar `Entidades/` + `FinancasDbContext.cs`, rodar `2-criar-banco.sh`, e usar `feat(banco)!:` no commit. Passos detalhados em [docs/BANCO-DE-DADOS.md](docs/BANCO-DE-DADOS.md).

## Decisões de arquitetura (ADRs) — obrigatório

As decisões ficam em `docs/adr/` (índice, critérios e ciclo de vida em [docs/adr/README.md](docs/adr/README.md); modelo em `docs/adr/0000-modelo.md`). **Antes de implementar** qualquer mudança que adicione ou troque dependência/integração externa (inclusive novo domínio de saída de rede), mude o banco além de uma tabela/coluna rotineira, mexa em segurança, infraestrutura/publicação/CI, ou crie/quebre um padrão de código do projeto:

1. Leia o índice de `docs/adr/README.md` e as ADRs do assunto. Se a mudança contraria uma ADR **Aceita**, avise o usuário antes de seguir — não contorne a decisão em silêncio.
2. **Já existe ADR do assunto:** atualize-a se a mudança só detalha ou ajusta a mesma decisão (consequências, passos práticos). Se a decisão em si mudou, crie uma ADR nova e marque a antiga como `Substituída por NNNN` — nunca apague ADR.
3. **Não existe:** crie `docs/adr/NNNN-titulo-em-kebab-case.md` a partir do modelo, com o próximo número livre, alternativas consideradas e consequências negativas; marque como *inferido* o que não foi confirmado pelo usuário.
4. Atualize o índice em `docs/adr/README.md` e, se mudar o estado atual descrito, `docs/ARQUITETURA.md`.
5. A ADR vai no mesmo commit/PR da mudança. Ao final, diga ao usuário qual ADR foi criada ou atualizada (ou por que a mudança não precisou de uma).

Não precisa de ADR: bugfix, refatoração local, ajuste visual, tela ou endpoint que só segue padrões já registrados.

## Convenções de código

- Nomes em português em todo o código (`XxxService`, `XxxController`, `XxxDto`, `SalvarXxxDto`).
- Consultas ao banco **só via LINQ/lambda** sobre entidades mapeadas (nunca SQL montado a partir de texto do usuário); atualizações em massa com `ExecuteUpdate`/`ExecuteDelete`.
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
| WCAG 2.2 — 2.5.8 *Target Size (Minimum)* (AA) | Alvo de clique/toque ≥ 24×24 CSS px (fatias pequenas de gráfico não atendem) | [w3.org/WAI/WCAG22/Understanding/target-size-minimum](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum) |
| WCAG 2.2 — 1.4.13 *Content on Hover or Focus* (AA) | Tooltips: dispensável, alcançável e persistente; em toque não existe hover | [w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus](https://www.w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus) |
| WCAG 2.2 — 1.4.11 *Non-text Contrast* (AA) | Borda e estados de controles (caixa de marcar, interruptor, foco) com contraste ≥ 3:1 com o que está ao lado | [w3.org/WAI/WCAG22/Understanding/non-text-contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast) |
| Kendrick — *Toggle-Switch Guidelines* (NN/g, 2018) | Interruptor só para efeito **imediato**; o que depende de "Salvar" fica caixa de marcar; rótulo curto, palavra-chave primeiro (teste: ler com "ligado/desligado" no fim) | [nngroup.com/articles/toggle-switch-guidelines](https://www.nngroup.com/articles/toggle-switch-guidelines/) |
| WAI-ARIA APG — *Switch Pattern* | Interruptor: `role="switch"` (no checkbox nativo usa o `checked`), Espaço alterna, rótulo **não muda** com o estado | [w3.org/WAI/ARIA/apg/patterns/switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/) |
| WCAG 2.2 — 3.3.1 *Error Identification* (A) | Erro de entrada detectado: diz qual campo e o que está errado, em texto | [w3.org/WAI/WCAG22/Understanding/error-identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification) |
| WCAG 2.2 — 3.3.2 *Labels or Instructions* (A) | Todo campo com rótulo visível (o "R$" dentro do campo não substitui o rótulo) | [w3.org/WAI/WCAG22/Understanding/labels-or-instructions](https://www.w3.org/WAI/WCAG22/Understanding/labels-or-instructions) |

Padrões já adotados a partir delas (seguir, salvo motivo registrado):
- **Detalhe por navegação, não modal**, quando já existe tela com o detalhe (ex.: total por categoria → `/transacoes` filtrada), levando o contexto completo (categoria + período + tipo) em query params.
- **Filtro ativo sempre visível** (chips com ✕ e "Limpar filtros") em tela que pode abrir já filtrada.
- **Estado de tela navegável na URL**: botão voltar do navegador tem que funcionar; atualização de filtro usa `replaceUrl` para não poluir o histórico.
- **Gráfico se explica**: título + frase-resumo com a conclusão + linha "como ler" + legenda quando há mais de uma série; resumo respeita "ocultar valores".
- **Termo técnico com definição no contexto** (ⓘ que abre por clique/teclado, vindo de um glossário único) — nunca só em `title=`, que não funciona em toque nem teclado.
- **Tela densa mostra primeiro a resposta à pergunta principal dela**; análises de apoio vão para `<app-mais-analises>` (botão com `aria-expanded` que diz o que tem dentro, conteúdo em `<ng-template>` criado só ao abrir, escolha lembrada por tela). Nada é removido: tudo a no máximo um clique.
- **Interruptor** (`label.interruptor` + `role="switch"` no checkbox nativo) para o que muda a tela na hora; caixa de marcar para o que depende de "Salvar" ([ADR 0032](docs/adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- **Seleção em tabela:** a célula inteira é o alvo (`label.celula-marcar`), a linha marcada fica destacada (`tr.selecionada`), e o nome da caixa diz qual linha é ([ADR 0032](docs/adr/0032-controles-padrao-interruptor-e-campo-de-valor.md)).
- O porquê e o custo aceito de cada padrão acima estão nas ADRs [0017](docs/adr/0017-preferencias-de-exibicao-no-navegador.md) a [0021](docs/adr/0021-componentes-padrao-para-graficos-e-telas-densas.md).

Ao responder: separar explicitamente o que vem dessas fontes do que é **julgamento próprio** (trade-off sem fonte direta), e dizer quando algo não foi validado com uso real. Fonte nova entra nesta tabela com link verificado, não citada de memória.

## Git / commits

Conventional Commits em português (ver [CONTRIBUINDO.md](CONTRIBUINDO.md) para fluxo de branches completo). Escopos usados: `api`, `web`, `banco`, `scripts`, `open-finance`, `investimentos`, `relatorios`. Branch `main` é protegida (só recebe merge de `develop`/`hotfix/*`); trabalho novo parte de `develop` em `feature/<assunto>` ou `fix/<assunto>`.

## Segurança (específico deste projeto — não genérico)

Tudo roda local, sem exposição de rede (bind só em `127.0.0.1:5443`, direto pelo Kestrel — sem IIS/proxy). Pontos que afetam como escrever código aqui:
- Acesso ao banco é só permissão de arquivo do SO sobre `dados/` — SQLite não tem login/usuário de banco; quem lê/grava é o processo da API, rodando como a conta do serviço do sistema (systemd no Linux, Serviço do Windows no Windows). Sem senha de banco em lugar nenhum.
- Segredos (Client Secret da Pluggy, token da brapi) via ASP.NET Data Protection, chaves em `dados/chaves`: DPAPI no Windows (`OperatingSystem.IsWindows()` em `Program.cs`), permissão de arquivo no Linux.
- CSP restritiva: nada de scripts/recursos de terceiros carregados do front-end.
- Saídas de rede permitidas e já implementadas: `api.pluggy.ai`, `api.bcb.gov.br`, `query1.finance.yahoo.com`/`brapi.dev` (API) e `api.github.com`/`github.com` (só o gerenciador do Windows, para versão nova; ADR 0028) — qualquer integração externa nova deve seguir o mesmo padrão (HTTPS, dado mínimo enviado, ver tabela em [README.md](README.md#segurança)).
