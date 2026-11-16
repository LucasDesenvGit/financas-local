# 0070. Versão calculada no build pelos merges do develop, e o CHANGELOG com um arquivo por PR

- **Status:** Proposta
- **Data:** 2026-11-16 (issue #721, item 1 da #689; anda junto com a #722, grupo 5 da #690)
- **Relacionadas:** substituirá a [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) quando for aceita; ajustará a [0053](0053-receber-versoes-de-desenvolvimento-por-escolha.md) (o número de cada merge passa a ser calculado); [0050](0050-msi-gerado-no-merge-e-nao-no-pr.md) (o `.msi` só no merge e na tag); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o `.msi` na CI); [0011](0011-ferramentas-de-instalacao-portateis-sem-python.md) (sem Python na instalação)

Enquanto for Proposta, a 0039 continua valendo: todo PR sobe a versão, inclusive o desta ADR.

## Contexto

Pela 0039, todo PR sobe o número no `Directory.Build.props`, pelo tipo do título, e escreve a entrada numa seção própria do `CHANGELOG.md` (`## [X.Y.Z] — data · desenvolvimento`). As duas mudanças ficam nas mesmas linhas para todo PR. Com vários PRs abertos, cada merge deixa os outros em conflito.

- **O custo medido** (levantamento da #689): desde 15/10, 341 PRs com merge e **210 merges do `develop` dentro das branches**, quase todos por esse conflito. Nas PRs abertas em 15/11, o 3.13.0 aparecia duas vezes e o 3.14.0, três. Em 16/11, cinco PRs seguidas precisaram do `scripts/dev/atualizar-pr.sh` em cascata, e uma delas teve conflito de verdade no meio.
- **O que existe para conviver com isso:** o check "versão subiu" (`ci.yml`) roda o `verificar-versao.py`, o `verificar-changelog.py` e os testes do `atualizar-versao-do-pr.py` e do `notas.sh`. Somam 867 linhas, com os testes, entre `scripts/ci/versao/` e `scripts/dev/`, mais três regras no `CLAUDE.md` e um item no modelo de PR.
- **Quem lê o número hoje:** o `versao.yml` (que monta o rótulo `X.Y.Z-dev.<execução>` e confere a tag), o `instalador/gerar-msi.ps1` (que grava o número no site e no gerenciador pelo `-ldflags`), o `3-publicar-atualizar.sh`, o `instalar-windows.ps1`, os dois `instalar-gerenciador`, o teste de contrato (`rodar.sh` e `contrato_test.go`), o `teste-instalacao-linux.yml` e o gerenciador, no "Atualizar para a última do develop" (`develop.go`, que lê o `Directory.Build.props` e o `CHANGELOG.md` do clone).
- **Quem lê o CHANGELOG:** o `notas.sh` monta as notas do Release e do pré-lançamento "desenvolvimento". O gerenciador separa, nessas notas, as seções `### X.Y.Z — ` mais novas que a versão instalada (`internal/atualizacao/notas.go`). Ou seja, a janela "O que mudou" depende de cada entrada estar sob o número da versão em que entrou.
- **A proteção do `develop`** (conferida pela API em 16/11): só um check obrigatório, o `build`, para quem não é administrador. O "versão subiu" não está entre os obrigatórios.
- **Os merges guardam o tipo:** desde 15/10, todo PR entrou por merge commit (só dois commits diretos), e o corpo do merge traz o título do PR (`feat(web): …`). Dá para saber o tipo de cada merge só pelo git, sem a API do GitHub.

Na direção do aval de 15/11, o PR não mexe mais no número, e o número é **calculado** no build, sem um job que commite no `develop`.

## Alternativas consideradas

### Para o número da versão

- **(a) Manter a 0039.** Prós: nada a fazer; o número aparece no próprio PR. Contras: o conflito a cada merge, que é o problema; o `atualizar-pr.sh` só automatiza a resolução, e cada rodada roda a CI do PR de novo.
- **(b) Um job do `develop` que grava a versão depois do merge.** Prós: o número continua num arquivo, legível sem git. Contras:
  - o push precisa passar pela proteção. O commit feito com o `GITHUB_TOKEN` não dispara workflows ([documentação do GitHub](https://docs.github.com/en/actions/concepts/security/github_token#when-github_token-triggers-workflow-runs)), então o `build` nunca roda nele, e o push seria recusado pelo check obrigatório (*inferido*, não testado). Saída: um token de administrador ou um app com permissão de passar por cima da proteção, guardado nos segredos;
  - dois merges seguidos disputam o mesmo push (o segundo precisa refazer);
  - o `.msi` do merge sairia com o número antigo, a menos que o `versao.yml` espere o commit do bot;
  - a 0039 já tinha descartado essa saída duas vezes (20/10 e 02/11).
- **(c1) Última tag mais a contagem de merges** (`git describe`). Prós: o mais simples de calcular. Contras: o número deixa de dizer se entrou funcionalidade. E a última tag é a `v3.3.14`, de 05/11: o número cairia abaixo do `3.15.8` do `develop` de hoje, e o gerenciador não ofereceria a versão a quem já tem uma de desenvolvimento (a 0053 exige número maior).
- **(c2) Base mais o tipo de cada merge, calculado no build:** escolhida. O número parte de uma base gravada no repositório e sobe, merge a merge na linha principal do `develop`, pela mesma tabela da 0039 (`feat` → o do meio; o resto → o último). Prós: o mesmo commit dá o mesmo número em qualquer máquina; nenhum PR mexe no número; os números continuam dizendo o tamanho da mudança. Contras: precisa do histórico do git em todo build, e o PR não sabe o número que vai ganhar.

Conferido em 16/11: aplicando a regra da (c2) aos 36 merges desde a `v3.3.14`, sai **3.14.8**, e não o 3.15.8 do `develop`. A diferença vem de subidas que não seguiram a regra merge a merge, na cascata e nas escolhas à mão (*inferido*; não conferido PR a PR). Por isso a base precisa ser acertada na troca, acima de tudo o que já saiu.

### Para o CHANGELOG

- **(d1) Uma seção "Não lançado" única.** Prós: um arquivo só, como no Keep a Changelog. Contras: dois PRs que acrescentam uma linha no mesmo lugar (o topo do mesmo `### Correções`) **ainda conflitam**. Conferido com o git 2.56.1: os dois lados inserem no mesmo ponto, e o merge para. Além disso, as entradas perdem o número, e o gerenciador não sabe o que é mais novo que a versão instalada.
- **(d2) A (d1) com `merge=union` no `.gitattributes`.** Prós: no git local, as duas linhas entram sem conflito (conferido). Contras: **no GitHub, não vale**, segundo os relatos: a [discussão #9288](https://github.com/orgs/community/discussions/9288), aberta desde 2021 e sem resposta da equipe do GitHub, cita uma resposta do suporte de 2017 ("o GitHub não considera os `.gitattributes` do usuário"), e ninguém relata que passou a funcionar. Não há documentação oficial que confirme num sentido ou no outro: *não confirmado*. A atualização de 02/11 da 0039 já dizia o mesmo. Teria de ser testado num PR de verdade antes de alguém contar com isso.
- **(d3) Um arquivo por PR em `changelog.d/`, juntado no lançamento:** escolhida. Cada PR cria um arquivo novo, com nome próprio (o da branch), e dois arquivos novos diferentes não conflitam nunca. Prós: sem conflito nem no botão do GitHub; o merge que acrescentou o arquivo dá o número da versão dele. Contras: a pasta precisa de um script que a junte, e quem lê o `CHANGELOG.md` do `develop` não vê o que ainda não saiu.

### O que acontece com cada peça

| | (a) manter | (b) job grava no `develop` | (c2) calculado no build (+ d3) |
|---|---|---|---|
| **`.msi` e tag `vX.Y.Z`** (0050) | como hoje | o `.msi` do merge precisa esperar o commit do bot; a tag confere o arquivo | o `versao.yml` calcula o número (checkout com o histórico inteiro); na tag, confere que ela bate com o cálculo |
| **Notas do gerenciador** (0053) | uma versão por merge, como hoje | igual | igual: cada merge tem o seu `X.Y.Z`, e o `notas.sh` monta as mesmas seções `### X.Y.Z — `, agora a partir de `changelog.d/` |
| **Check "versão subiu"** | fica | sai | sai; fica só a conferência do formato das entradas |
| **Verificadores e `atualizar-pr.sh`** | ficam (867 linhas) | saem o `verificar-versao.py` e o `atualizar-pr.sh`; o bot é uma peça nova | saem o `verificar-versao.py`, o `atualizar-pr.sh`, o `atualizar-versao-do-pr.py` e o teste dele; o `verificar-changelog.py` passa a conferir `changelog.d/` |
| **O que lembrar em cada PR** | subir pelo tipo, criar a seção, rodar o `atualizar-pr.sh` a cada merge de outro PR | o título no tipo certo e a entrada | o título no tipo certo e um arquivo em `changelog.d/` |
| **Quebra de verdade** | quem faz o PR sobe o primeiro número, depois de perguntar ao usuário | precisa de um sinal que o bot leia (rótulo ou título) | o PR troca a base para `X+1.0.0`, depois de perguntar ao usuário; o cálculo recomeça dela |

## Decisão (proposta)

**O número da versão é calculado no build: parte da base do `Directory.Build.props` e sobe a cada merge na linha principal do `develop`, pelo tipo do título do PR, com a tabela da 0039. Nenhum PR mexe no número, salvo a quebra de verdade. A entrada do CHANGELOG vai num arquivo próprio em `changelog.d/`, juntado no `CHANGELOG.md` quando a versão é publicada.**

A regra do cálculo:

1. **A base** é o `<VersaoBase>` do `Directory.Build.props`, no último merge da linha principal que mudou esse arquivo (`git log --first-parent -1 -- Directory.Build.props`). O nome da propriedade muda (era `<Version>`) para que quem ainda lê o número antigo falhe na hora, em vez de ler uma base como se fosse a versão (*inferido*).
2. **Cada merge depois dele**, na ordem, sobe a versão pelo tipo do título (que está no corpo do merge commit; num squash, no assunto): `feat` sobe o do meio e zera o último; os outros tipos, ou um título fora do padrão, sobem o último.
3. **Não sobem** os merges que trazem a `main` ou uma `release/*` de volta (o conteúdo já está contado) (*inferido*).
4. **Quebra de verdade:** o PR muda a `<VersaoBase>` para `X+1.0.0`, como hoje, depois de perguntar ao usuário. Só dois PRs de quebra ao mesmo tempo conflitam, e o projeto teve três versões maiores na história toda.
5. **O rótulo** continua `X.Y.Z-dev.<execução>` fora da tag, e `X.Y.Z` na tag (0053 e 0029 sem mudança no formato).
6. **Sem o histórico do git** (uma cópia baixada em `.zip`), o número é a base, com o sufixo `-sem-git` (*inferido*).

### Passos, depois do aval

Na ordem, depois que a pilha #697–#707 entrar:

1. **O cálculo, em Go:** `src/backend/cmd/versao` (um programa só, sem dependência nova), com teste em tabela (lista de merges → número) e no git de verdade, num repositório temporário. Em Go porque roda no Linux, no Windows (sem Python, 0011) e na CI. Ele também imprime as seções do CHANGELOG por versão (`-changelog`), juntando `changelog.d/` com o número de cada merge. Pacote novo: tabela do teste de arquitetura e `docs/arquitetura/componentes.md`.
2. **`Directory.Build.props`:** `<Version>` vira `<VersaoBase>`, com a base acima da maior versão do `develop` e das PRs abertas na hora (subindo o do meio), e o comentário explicando a regra.
3. **Quem lê o número** passa a chamar o cálculo: `versao.yml` (job `gerar`, com `fetch-depth: 0`; na tag, confere a tag com o cálculo do commit do `develop` que entrou), `instalador/gerar-msi.ps1`, `scripts/geral/3-publicar-atualizar.sh`, `scripts/windows/instalar-windows.ps1`, `scripts/linux/instalar-gerenciador.sh`, `scripts/windows/instalar-gerenciador.ps1`, `src/backend/testes/contrato/rodar.sh` e `contrato_test.go`, `teste-instalacao-linux.yml` e os checkouts dos workflows de instalação.
4. **CHANGELOG:** criar `changelog.d/` com um `LEIA-ME.md` e o modelo do arquivo: `changelog.d/<nome-da-branch>.md`, com um título de tipo (`### Funcionalidades`, `### Correções` ou `### Publicação e documentação`) e a entrada, no formato da #722. As seções que já existem no `CHANGELOG.md` ficam como estão (são histórico). O `notas.sh` passa a ler as seções do `-changelog`, e o `test_notas.py` acompanha.
5. **Publicar:** um `scripts/dev/lancar-versao.sh X.Y.Z`, na `release/X.Y.Z`, junta os arquivos de `changelog.d/` numa seção `## [X.Y.Z] — data` do `CHANGELOG.md`, por tipo, e apaga os arquivos, no commit `chore: versão X.Y.Z`. O passo a passo do `CONTRIBUINDO.md` ("Publicar") muda junto.
6. **Gerenciador:** o "Atualizar para a última do develop" (`develop.go`, `internal/atualizacao/changelog.go`) para de ler o `Directory.Build.props` e o `CHANGELOG.md` do clone; pega o número do site publicado e as seções do `-changelog`. A separação das notas (`notas.go`) não muda, porque o formato das notas continua o mesmo, e os gerenciadores já instalados seguem lendo as notas novas (*inferido*). Rodar o "Versão (instalador .msi)" à mão na branch (0050). A #722 (esconder o "Técnico") entra junto ou logo depois.
7. **CI:** sai o job "versão subiu" do `ci.yml` (não é check obrigatório, então a proteção do branch não muda). A conferência de `changelog.d/` e o teste das notas vão para um job que já existe. Saem `scripts/ci/versao/verificar-versao.py`, `scripts/dev/atualizar-pr.sh`, `scripts/dev/atualizar-versao-do-pr.py` e `scripts/dev/test_atualizar_versao_do_pr.py`, e o que ficar sem uso em `scripts/ci/comum.py`. O `scripts/README.md` acompanha.
8. **Documentação, no mesmo PR:** esta ADR passa a Aceita; a 0039 fica "Substituída por 0070"; a 0053 ganha uma atualização (cada merge continua com uma versão, agora calculada); o índice em `docs/adr/README.md`; no `CLAUDE.md`, as regras de versão e de CHANGELOG viram uma linha cada; no `CONTRIBUINDO.md`, "Publicar" e "CHANGELOG"; no modelo de PR, os dois itens de versão e CHANGELOG viram "um arquivo em `changelog.d/`"; e as skills que citam o `Directory.Build.props`.

## Consequências

- **Positivas:**
  - acaba o conflito de versão e de CHANGELOG entre PRs paralelos, inclusive no botão de merge do GitHub, e com ele o `atualizar-pr.sh` em cascata;
  - o PR do Dependabot ganha número como qualquer outro (o `build(deps)` sobe o último), sem a exceção do check;
  - menos o que lembrar em cada PR, e cerca de 870 linhas de scripts e testes saem, trocadas por um programa com teste;
  - o gerenciador continua com uma versão por merge (0053), e as notas mantêm o formato.
- **Negativas:**
  - **O número depende do histórico do git.** Todo build precisa do clone com o histórico inteiro (`fetch-depth: 0` na CI) e do `git` instalado. A cópia em `.zip` sai com a base e o `-sem-git`. Reescrever a linha principal do `develop` (um force push) muda os números.
  - **O título do PR decide o número, sem conferência.** Um `fix` que era `feat` sai com o número errado, e o merge não tem como voltar atrás. Hoje o check pelo menos cobra o mínimo. Trocar o título antes do merge resolve; depois, só a base.
  - **O formato do merge commit vira contrato.** Se o repositório trocar a mensagem padrão do merge (ou passar a usar só squash), o cálculo precisa acompanhar. O teste cobre os dois formatos de hoje.
  - **O PR não sabe o número que vai ganhar:** a descrição não diz "3.16.0", e a entrada em `changelog.d/` não tem número até o merge.
  - **O `CHANGELOG.md` do `develop` não mostra o que ainda não saiu:** isso fica em `changelog.d/` e nas notas do pré-lançamento "desenvolvimento".
  - **Na publicação, uma seção por versão publicada** (*inferido*): quem estava numa versão de desenvolvimento e instala a publicada vê, no "O que mudou", também o que já tinha.
  - **A troca exige mexer no gerenciador e no instalador** de uma vez, com o `.msi` rodado à mão, e as PRs abertas na hora precisam tirar a subida de versão e mover a entrada para `changelog.d/`.
  - **O `go run` do cálculo** soma alguns segundos aos scripts de instalação (*inferido*; medir no PR).
- **Na prática (depois de aceita):**
  - no PR: o título no tipo certo e um arquivo em `changelog.d/<nome-da-branch>.md`; não mexer no `Directory.Build.props`;
  - quebra de verdade: perguntar ao usuário e mudar a `<VersaoBase>` no PR;
  - para saber a versão de um commit: `go run ./cmd/versao` em `src/backend`.

## Perguntas para o aval

1. **A quebra de verdade pela base no `Directory.Build.props`**, ou pelo `!` no título, como no Conventional Commits? O `!` exigiria trocar a convenção do `feat(banco)!`, que hoje marca mudança de esquema sem subir o primeiro número.
2. **`changelog.d/` no lugar da seção "Não lançado"** que o aval citou: a "Não lançado" ainda conflita no GitHub, e o `merge=union` não ajuda lá (*não confirmado*).
3. **Toda PR precisa de um arquivo em `changelog.d/`?** Ou só as que mudam algo para quem usa, com o Dependabot e as de CI de fora (as notas listam os PRs sem entrada pelo título)?
4. **Na publicação, uma seção por versão publicada** ou uma por versão de desenvolvimento, como hoje?
5. **O `.zip` sem git** sai com a base e o `-sem-git`, ou a instalação pelos scripts passa a exigir o clone?
6. **Os merges da `main` e das `release/*` de volta ao `develop`** ficam fora da contagem?
7. **O cálculo em Go** (`src/backend/cmd/versao`), ou outra linguagem? O Python não está na instalação do Windows (0011), e bash mais PowerShell seria a mesma regra em dois lugares, o que o item 2 da #689 está tirando.
