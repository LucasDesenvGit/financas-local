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
  - **Números que não saem:** o `develop` passa por 2.3.0, 2.3.1, 2.4.0…, e só alguns viram Release (o que a 0029 queria evitar). Aceito: o CHANGELOG continua juntando tudo em "Não publicado" até a versão sair.
  - **Quem está na versão de desenvolvimento** `X.Y.Z-dev.N` não recebe aviso do gerenciador quando sai o Release `X.Y.Z`: o gerenciador compara só `X.Y.Z`. Essa pessoa já tem o conteúdo da versão, então o aviso não faz falta, mas o rótulo "-dev.N" só some instalando o Release.
  - **Dependabot** não sobe a versão: o check pula os PRs dele. A atualização de dependência sai junto com o próximo PR que subir.
  - Uma decisão a mais em cada PR, e a quebra de verdade fica a critério de quem faz o PR (a CI não tem como saber).
- **Na prática:**
  - o check **"versão subiu"** (`scripts/ci/verificar-versao.py`, no `ci.yml`) falha o PR se a versão não subiu ou subiu menos do que o tipo do título pede (`feat` → pelo menos o do meio; o resto → pelo menos o último);
  - PR para a `main` (vindo do `develop` ou de `hotfix/*`) não é conferido: a versão já vem certa;
  - regra no `CLAUDE.md` e no `CONTRIBUINDO.md`.
