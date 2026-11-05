# 0039. A versão sobe a cada pull request, pelo tipo do commit

- **Status:** Aceita
- **Data:** 2026-10-20 (issue #187)
- **Relacionadas:** substitui em parte a [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (quando a versão muda); [0003](0003-esquema-por-scripts-sql-idempotentes.md) (o esquema só acrescenta)

## Contexto

Pela ADR 0029, a versão no `Directory.Build.props` só mudava na hora de publicar. Entre uma versão e outra, todo build do `develop` saía com o número da última publicada: depois da 2.1.0, dezenas de funcionalidades e correções saíram como "2.1.0-dev.52". O número não dizia quanto tinha mudado, nem o que um usuário da versão de desenvolvimento tinha instalado.

O usuário pediu que cada mudança suba a versão da aplicação. A 0029 tinha descartado justamente "versão de desenvolvimento como X.Y.(Z+1)", porque isso "prometeria um número que talvez nem saia".

O projeto já usa Conventional Commits (`feat`, `fix`, `docs`…), e os títulos dos PRs seguem o mesmo padrão.

## Alternativas consideradas

- **Continuar subindo só na hora de publicar** (0029). Prós: um PR nunca conflita com outro na versão. Contras: o número do `develop` não diz nada, e quem publica precisa decidir o número de uma vez, olhando o CHANGELOG.
- **Sempre o último número** (2.2.0 → 2.2.1 → 2.2.2…). Prós: sem decisão a cada PR. Contras: o número não diz o tamanho da mudança.
- **A CI sobe a versão sozinha no merge.** Prós: ninguém esquece e não há conflito. Contras: o workflow precisa gravar no `develop`, que é protegido (commit de bot, permissão de escrita, laço de CI); mais uma peça para dar errado.
- **Pelo tipo do commit, no próprio PR** (versionamento semântico): escolhida.

## Decisão

Todo pull request para o `develop` sobe a versão no `Directory.Build.props`, pelo tipo do título:

| Tipo | Sobe | Exemplo |
|---|---|---|
| `feat` (funcionalidade nova, inclusive `feat(banco)!` que só acrescenta tabela ou coluna) | o número do meio | 2.2.0 → 2.3.0 |
| `fix`, `docs`, `build`, `ci`, `refactor`, `perf`, `test`, `chore`, `style` | o último | 2.2.0 → 2.2.1 |
| Quebra de verdade: o usuário precisa fazer algo à mão, ou não dá para voltar à versão anterior | o primeiro | 2.2.0 → 3.0.0 |

O `!` do `feat(banco)!` continua marcando mudança de estrutura no commit, mas não sobe o primeiro número sozinho: pela ADR 0003, o esquema só acrescenta, e a versão anterior continua abrindo o banco.

A partir desta ADR, o `develop` foi para **2.2.0**, contando o que entrou desde a 2.1.0 (decisão do usuário).

## Consequências

- **Positivas:**
  - o build do `develop` mostra o número que vai sair: `2.3.0-dev.60` diz que tem funcionalidade nova desde a 2.2.0;
  - publicar fica mais simples: renomear a seção do CHANGELOG e criar a tag igual ao `Directory.Build.props`, sem decidir número;
  - o pacote de desenvolvimento e o Release de mesmo número trocam um pelo outro, como a 0029 já testa na CI.
- **Negativas:**
  - **Conflito na versão:** dois PRs abertos ao mesmo tempo mexem na mesma linha do `Directory.Build.props`. Quem entrar depois faz `git merge origin/develop`, pega a versão maior e sobe de novo pelo tipo do seu PR.
  - **Números que não saem:** o `develop` passa por 2.3.0, 2.3.1, 2.4.0…, e só alguns viram Release (o que a 0029 queria evitar). Aceito. O CHANGELOG juntava tudo em "Não publicado" até a versão sair; desde a #294 (2026-10-21) tem uma seção por versão (veja a atualização abaixo).
  - **Quem está na versão de desenvolvimento** `X.Y.Z-dev.N` não recebe aviso do gerenciador quando sai o Release `X.Y.Z`: o gerenciador compara só `X.Y.Z`. Essa pessoa já tem o conteúdo da versão, então o aviso não faz falta, mas o rótulo "-dev.N" só some instalando o Release.
  - **Dependabot** não sobe a versão: o check pula os PRs dele. A atualização de dependência sai junto com o próximo PR que subir.
  - Uma decisão a mais em cada PR, e a quebra de verdade fica a critério de quem faz o PR (a CI não tem como saber).
- **Na prática:**
  - o check **"versão subiu"** (`scripts/ci/verificar-versao.py`, no `ci.yml`) falha o PR se a versão não subiu ou subiu menos do que o tipo do título pede (`feat` → pelo menos o do meio; o resto → pelo menos o último);
  - PR para a `main` (vindo do `develop` ou de `hotfix/*`) não é conferido: a versão já vem certa;
  - regra no `CLAUDE.md` e no `CONTRIBUINDO.md`.

## Atualização de 2026-10-21: uma seção do CHANGELOG por versão (issue #294)

- **Problema:** o "Não publicado" juntava tudo desde a 2.1.0, com um `### Correções` novo por PR. Na publicação da 2.17.0, a seção dela foi criada só na branch `release/2.17.0`, que não voltou para o `develop`: o "Não publicado" do `develop` continuou com o que já tinha saído. As notas da versão de desenvolvimento e a janela "O que mudou" do gerenciador ficavam longas e repetidas.
- **O que mudou:** como todo PR já sobe a versão, o CHANGELOG passa a ter **uma seção por versão**:
  - `## [X.Y.Z] — AAAA-MM-DD · desenvolvimento` enquanto não for publicada, com um título de cada tipo dentro;
  - a entrada do PR vai na seção da versão dele;
  - publicar é tirar o "· desenvolvimento" do título.
- **Reorganização do que já existia:** os itens do antigo "Não publicado" foram para a versão em que entraram, achada pelo histórico do git (o merge no `develop` que acrescentou cada texto e a versão do `Directory.Build.props` nele). A 2.17.0 voltou com o texto publicado.
- **Notas:**
  - **do Release X.Y.Z:** a seção X e as de desenvolvimento abaixo dela, até a última publicada (`scripts/ci/notas-da-versao.sh`). Entre duas publicadas cabem centenas de versões de desenvolvimento, então elas entram até o limite de 125 mil caracteres do GitHub, cortadas no fim de uma versão e com um link para o CHANGELOG da tag (#641), como as de desenvolvimento (#600);
  - **da versão de desenvolvimento:** todas as seções "· desenvolvimento" e, por versão, os PRs que entraram nela (`scripts/ci/notas-desenvolvimento.sh`).
- **No gerenciador:** "O que mudou" mostra primeiro só as seções mais novas que a versão instalada (`NotasDaVersao`).
- **Na prática:**
  - `scripts/ci/verificar-changelog.py`, no check "versão subiu", falha com: tipo repetido na mesma versão; versão repetida ou fora de ordem; versão acima da do `Directory.Build.props`;
  - depois de publicar, a `main` volta para o `develop` por um PR de `main` para `develop`, em que o check "versão subiu" não roda: a versão é a mesma (#641).

## Atualização de 2026-11-02: o conflito da versão resolvido por um comando (issue #587)

- **Problema:** a consequência negativa "Conflito na versão" virou rotina. Com vários PRs por dia, cada merge deixava todos os outros em conflito no `Directory.Build.props` e no `CHANGELOG.md` (os dois mudam no mesmo lugar), e alguém precisava trazer o `develop`, subir a versão de novo e mover a entrada à mão. No mesmo dia, o #578 conflitou duas vezes.
- **O que mudou:** a decisão continua (cada PR sobe a versão pelo tipo); o conflito passa a ser resolvido por `scripts/geral/atualizar-pr.sh <número do PR>`:
  - traz o `origin/develop` numa worktree temporária, sem mexer na pasta de quem roda;
  - conflito só nos dois arquivos: sobe a versão do `develop` no nível em que o PR tinha subido em relação à base dele (o primeiro, o do meio ou o último número), nunca menos do que o tipo do título pede, e põe a seção do PR no topo do CHANGELOG com o número novo e a data de hoje (`scripts/geral/atualizar-versao-do-pr.py`, com teste em `test_atualizar_versao_do_pr.py`, rodado no check "versão subiu");
  - confere com o `verificar-versao.py` e o `verificar-changelog.py` e envia com a conta de quem rodou;
  - conflito em outro arquivo: desfaz e lista os arquivos.
- **Alternativas consideradas:**
  - **A CI sobe a versão no merge**, com um arquivo de CHANGELOG por PR: acabaria com o conflito, mas grava no `develop` protegido. Continua descartada, pelo mesmo motivo de 2026-10-20;
  - **um workflow que roda o mesmo script sozinho quando o `develop` anda:** o commit feito com o `GITHUB_TOKEN` não dispara a CI do PR ([documentação do GitHub](https://docs.github.com/en/actions/concepts/security/github_token#when-github_token-triggers-workflow-runs)), então precisaria de um token próprio nos segredos do repositório. Fica para depois, se o comando local não bastar (decisão do usuário);
  - **`merge=union` no `.gitattributes`:** o GitHub não usa esse ajuste na tela de conflito nem no botão de merge, e a versão continuaria errada.
- **Consequência negativa:** o conflito continua acontecendo; só deixa de ser trabalho à mão. Alguém precisa rodar o comando, e cada atualização roda a CI do PR de novo.
- **Dependabot:** o check "versão subiu" passa a pular os PRs pelo **autor** do PR, e não por quem disparou a execução. Com o `github.actor`, um `@dependabot rebase` pedido por uma pessoa disparava em nome dela, e o check cobrava a versão de um PR do Dependabot (#584).
