# 0070. Versão calculada no build pelos merges do develop, e o CHANGELOG com um arquivo por PR

- **Status:** Aceita (aval de Felipe em 16/11/2026, na #721; implementada na mesma PR)
- **Data:** 2026-11-16 (issue #721, item 1 da #689; anda junto com a #722, grupo 5 da #690)
- **Relacionadas:** substitui a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md); ajusta a [0053](0053-receber-versoes-de-desenvolvimento-por-escolha.md) (o número de cada merge passa a ser calculado); [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md) (o `.msi` só no merge e na tag); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o `.msi` na CI); [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (sem Python na instalação)

O aval (sete perguntas e respostas), os passos da troca e como ficou a implementação estão na [versão anterior desta ADR](https://github.com/LucasDesenvGit/financas-local/blob/9579c81a6de07ffbc2d46af5e6191e4f8bf2f2c5/docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md#aval-felipe-16112026-na-721) e na PR da #721.

## Contexto

Pela 0039, todo PR subia o número no `Directory.Build.props`, pelo tipo do título, e escrevia a entrada numa seção própria do `CHANGELOG.md`. As duas mudanças caíam nas mesmas linhas para todo PR: com vários PRs abertos, cada merge deixava os outros em conflito.

- **O custo medido** (levantamento da #689): desde 15/10, 341 PRs com merge e 210 merges do `develop` dentro das branches, quase todos por esse conflito. Em 15/11, o 3.13.0 aparecia em duas PRs abertas e o 3.14.0 em três; em 16/11, cinco PRs seguidas precisaram do `atualizar-pr.sh` em cascata.
- **O que existia para conviver com isso:** o check "versão subiu" e os scripts dele, 867 linhas com os testes, mais três regras no `CLAUDE.md` e um item no modelo de PR. O check não era obrigatório: a proteção do `develop` só exige o `build`.
- **Quem lê o número:** o workflow do `.msi`, o `gerar-msi.ps1`, os scripts de instalação, o teste de contrato e o gerenciador. **Quem lê o CHANGELOG:** o `notas.sh` (notas dos Releases) e o gerenciador, que mostra no "O que mudou" as seções `### X.Y.Z — ` mais novas que a versão instalada. Ou seja, cada entrada precisa ficar sob o número da versão em que entrou.
- **Os merges guardam o tipo:** desde 15/10, todo PR entrou por merge commit, com o título (`feat(web): …`) no corpo. Dá para saber o tipo de cada merge só pelo git, sem a API do GitHub.

## Alternativas consideradas

### Para o número da versão

- **(a) Manter a 0039.** Prós: nada a fazer; o número aparece no próprio PR. Contras: o conflito a cada merge, que é o problema; o `atualizar-pr.sh` só automatiza a resolução, e cada rodada roda a CI do PR de novo.
- **(b) Um job do `develop` que grava a versão depois do merge.** Prós: o número continua num arquivo, legível sem git. Contras: o commit feito com o `GITHUB_TOKEN` não dispara workflows ([documentação do GitHub](https://docs.github.com/en/actions/concepts/security/github_token#when-github_token-triggers-workflow-runs)), então o push seria recusado pelo check obrigatório (*inferido*, não testado) e precisaria de um token que passa por cima da proteção; dois merges seguidos disputam o mesmo push; o `.msi` do merge sairia com o número antigo; a 0039 já tinha descartado essa saída duas vezes.
- **(c1) Última tag mais a contagem de merges** (`git describe`). Prós: o mais simples. Contras: o número deixa de dizer se entrou funcionalidade, e a última tag (`v3.3.14`) daria um número abaixo do `3.15.8` do `develop`: o gerenciador não ofereceria a versão a quem já tem uma de desenvolvimento (a 0053 exige número maior).
- **(c2) Base mais o tipo de cada merge, calculado no build:** escolhida. Prós: o mesmo commit dá o mesmo número em qualquer máquina; nenhum PR mexe no número; o número continua dizendo o tamanho da mudança. Contras: precisa do histórico do git em todo build, e o PR não sabe o número que vai ganhar. Aplicada aos merges desde a `v3.3.14`, a regra dava 3.14.8, e não o 3.15.8 de então (subidas à mão e na cascata, *inferido*): por isso a base foi acertada na troca, acima de tudo o que já tinha saído.

### Para o CHANGELOG

- **(d1) Uma seção "Não lançado" única.** Prós: um arquivo só, como no Keep a Changelog. Contras: dois PRs que acrescentam uma linha no topo do mesmo `### Correções` **ainda conflitam** (conferido com o git 2.56.1); e as entradas perdem o número, então o gerenciador não sabe o que é mais novo que a versão instalada.
- **(d2) A (d1) com `merge=union` no `.gitattributes`.** Prós: no git local, as duas linhas entram sem conflito (conferido). Contras: **no GitHub, não vale**, segundo os relatos (a [discussão #9288](https://github.com/orgs/community/discussions/9288), sem resposta oficial; *não confirmado*). Teria de ser testado num PR de verdade antes de alguém contar com isso.
- **(d3) Um arquivo por PR em `changelog.d/`, juntado no lançamento:** escolhida. Dois arquivos novos com nomes diferentes não conflitam nunca. Prós: sem conflito nem no botão do GitHub; o merge que acrescentou o arquivo dá o número da versão dele. Contras: a pasta precisa de um programa que a junte, e quem lê o `CHANGELOG.md` do `develop` não vê o que ainda não saiu.

A comparação peça a peça (`.msi` e tag, notas do gerenciador, check, scripts, o que lembrar em cada PR, quebra de verdade) está na [tabela da versão anterior](https://github.com/LucasDesenvGit/financas-local/blob/9579c81a6de07ffbc2d46af5e6191e4f8bf2f2c5/docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md#o-que-acontece-com-cada-peça).

## Decisão

**O número da versão é calculado no build: parte da base do `Directory.Build.props` e sobe a cada merge na linha principal do `develop`, pelo tipo do título do PR, com a tabela da 0039. Nenhum PR mexe no número, salvo a quebra de verdade. A entrada do CHANGELOG vai num arquivo próprio em `changelog.d/`, juntado no `CHANGELOG.md` quando a versão é publicada.**

A regra do cálculo:

1. **A base** é o `<VersaoBase>` do `Directory.Build.props`, no último commit da linha principal que mudou o **valor** desse elemento (mexer só no comentário não recomeça a conta). O nome mudou (era `<Version>`) para que quem ainda lesse o número antigo falhasse na hora (*inferido*).
2. **Cada merge depois dele**, na ordem, sobe pelo tipo do título (no corpo do merge commit; num squash, no assunto, que termina em `(#N)`): `feat` sobe o do meio e zera o último; os outros tipos, ou um título fora do padrão, sobem o último. Commit direto, como o `chore: versão X.Y.Z` da publicação, não sobe.
3. **Não sobem** os merges que trazem a `main` ou uma `release/*` de volta (o conteúdo já está contado).
4. **Quebra de verdade:** o PR muda a `<VersaoBase>` para `X+1.0.0`, depois de perguntar ao usuário. O `feat(banco)!` continua como na 0039: marca esquema novo e não sobe o primeiro número.
5. **O rótulo** continua `X.Y.Z-dev.<execução>` fora da tag, e `X.Y.Z` na tag (0053 e 0029 sem mudança no formato).
6. **Sem o histórico do git** (cópia em `.zip`, clone raso ou sem o `git`), o número é a base, com o sufixo `-sem-git`.
7. **Na tag**, o commit é o merge da `release/X.Y.Z` na `main`: o número conferido é o do commit da release (o segundo pai), que saiu do `develop`.

O cálculo é em Go, em `src/backend/cmd/versao`, porque roda no Linux, no Windows (sem Python, 0011) e na CI; o mesmo programa monta as seções do CHANGELOG por versão e junta `changelog.d/` na publicação. O arquivo em `changelog.d/` é obrigatório só em `feat` e `fix` (o que muda para quem usa); as outras PRs entram nas notas pelo título. Na publicação, o `CHANGELOG.md` ganha uma seção por versão publicada.

## Consequências

- **Positivas:**
  - acaba o conflito de versão e de CHANGELOG entre PRs paralelos, inclusive no botão de merge do GitHub, e com ele o `atualizar-pr.sh` em cascata;
  - o PR do Dependabot ganha número como qualquer outro (o `build(deps)` sobe o último), sem exceção no check;
  - sai o check "versão subiu", e cerca de 870 linhas de scripts e testes, trocadas por um programa com teste; em cada PR, só o título no tipo certo e um arquivo em `changelog.d/`;
  - o gerenciador continua com uma versão por merge (0053), e as notas mantêm o formato.
- **Negativas:**
  - **O número depende do histórico do git.** Todo build precisa do clone inteiro (`fetch-depth: 0` na CI) e do `git` instalado; a cópia em `.zip` sai com a base e o `-sem-git`. Reescrever a linha principal do `develop` (um force push) muda os números.
  - **O título do PR decide o número, sem conferência.** Um `fix` que era `feat` sai com o número errado; trocar o título antes do merge resolve, depois só a base.
  - **O formato do merge commit vira contrato.** Se o repositório trocar a mensagem padrão do merge (ou passar a usar só squash), o cálculo precisa acompanhar. O teste cobre os dois formatos de hoje.
  - **O PR não sabe o número que vai ganhar**, e a entrada em `changelog.d/` não tem número até o merge.
  - **O `CHANGELOG.md` do `develop` não mostra o que ainda não saiu:** isso fica em `changelog.d/` e nas notas do pré-lançamento "desenvolvimento".
  - **Na publicação, uma seção por versão publicada** (*inferido*): quem estava numa versão de desenvolvimento e instala a publicada vê, no "O que mudou", também o que já tinha.
  - **O `go run` do cálculo** soma alguns segundos aos scripts de instalação (*inferido*).
- **Na prática:** no PR, o título no tipo certo e, em `feat` e `fix`, um arquivo em `changelog.d/`; não mexer no `Directory.Build.props`, salvo a quebra de verdade. O passo a passo está no [`CONTRIBUINDO.md`](../../CONTRIBUINDO.md#versão-e-changelog) ("Versão e CHANGELOG" e "Publicar uma versão") e no [`changelog.d/LEIA-ME.md`](../../changelog.d/LEIA-ME.md).
