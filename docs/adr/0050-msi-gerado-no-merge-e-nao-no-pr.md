# 0050. O .msi é gerado no merge para o develop, e não no PR

- **Status:** Aceita
- **Data:** 2026-10-22 (issue #344)
- **Relacionadas:** [0047](0047-ci-paralela-so-testes-afetados-no-pr.md) (substitui a parte do .msi no PR); [0029](0029-instalador-msi-com-wix-gerado-pela-ci.md) (o .msi na CI); [0039](0039-versao-sobe-a-cada-pr-pelo-tipo-do-commit.md) (a versão sobe em todo PR)

## Contexto

O workflow "Versão (instalador .msi)" (`versao.yml`) gera e testa o `.msi` num Windows limpo (~7 min: gerar, instalar,
atualizar, desinstalar e migrar de uma instalação pelos scripts). Pela ADR 0047 (#255, #307), ele rodava também no
PR que mexe no instalador, no gerenciador ou na inicialização da API (`scripts/ci/precisa-msi.py`).

Na prática, o PR é refeito várias vezes: novos pushes, "Update branch" depois de outro merge e cadeias de PR (um
contendo o outro, para o CHANGELOG e a versão não conflitarem). Cada vez, um `.msi` novo, que era jogado fora: o que
vale é o do develop, depois do merge. O usuário pediu que o instalador seja gerado só no merge.

## Alternativas consideradas

- **Manter no PR só quando mexe no instalador (ADR 0047)** — pega o problema antes do merge; mas o `.msi` roda de novo
  a cada atualização do PR, e o PR fica ~7 min mais lento.
- **Rodar no PR só no último push antes do merge** — não há como o GitHub saber qual é o último; exigiria um rótulo
  ou um comando no PR, um passo a mais para lembrar.
- **Só no merge para o develop e na tag, com o disparo manual na branch quando quiser testar antes** (escolhida).

## Decisão

O `.msi` é gerado e testado **no push do `develop`** (o merge de um PR), **na tag `vX.Y.Z`** e **à mão**
(`workflow_dispatch`), nunca no PR. O `versao.yml` sai do gatilho `pull_request`, e o `precisa-msi.py` é apagado.

## Consequências

- **Positivas:** o PR não gasta ~7 min de Windows a cada atualização; o `.msi` gerado é sempre o do código que entrou.
- **Negativas:** um problema no instalador, no gerenciador ou na inicialização da API que só o teste do `.msi` pega
  aparece **depois do merge**, no develop, e não no PR. A partir daí, o develop fica vermelho até a correção, e o
  pré-lançamento "desenvolvimento" não é atualizado (só é publicado depois de todos os testes).
- **Na prática:**
  - PR que mexe em `instalador/`, `src/gerenciador/`, `Program.cs` ou `EsquemaBanco.cs`: rode "Versão (instalador .msi)"
    à mão na branch (Actions → Run workflow) antes do merge, e diga no PR que rodou;
  - o job "API e banco" continua ligando a API em todo PR de back-end (`ligar-api.sh`, #309), o que pega boa parte dos
    erros de inicialização sem o `.msi`;
  - o título `ci:` (#307) deixa de importar para o `.msi`; continua sendo o tipo certo para mudança só de CI (ADR 0039).
