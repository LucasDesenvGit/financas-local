# changelog.d: uma entrada por PR

Cada PR que muda algo para quem usa o app põe aqui **um arquivo novo** com a entrada do CHANGELOG ([ADR 0070](../docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)). Dois PRs criam dois arquivos de nomes diferentes, então nunca conflitam, nem no botão de merge do GitHub.

## Quando é obrigatório

- **PR `feat` e `fix`:** o arquivo é obrigatório (a CI confere, `scripts/ci/versao/verificar-changelog.py`).
- **CI, testes, documentação interna, refatoração e Dependabot:** podem não ter. As notas dos Releases listam esses PRs pelo título.

## O nome

`changelog.d/<nome-da-branch>.md`, com a `/` da branch trocada por `-`: a branch `feature/relatorio-anual` vira `changelog.d/feature-relatorio-anual.md`.

## O formato

O arquivo começa pelo título do tipo, que é um destes três, e tem pelo menos uma entrada em cada tipo. Um tipo só aparece uma vez no arquivo.

```markdown
### Correções
- **O que mudou, em poucas palavras** (#123). O que muda para quem usa, em uma ou duas frases.
  - Técnico: o que só importa para quem mexe no código (rota, coluna, ADR), se importar.
```

Os tipos: `### Funcionalidades`, `### Correções` e `### Publicação e documentação`.

- **Não escreva a versão.** Ela é calculada quando o PR entra (`go run ./cmd/versao`, em `src/backend`), e quem escreve a entrada não sabe qual vai ser. Nada de `## [X.Y.Z]`.
- **Não mexa no `CHANGELOG.md` nem no `Directory.Build.props`** num PR comum. As seções que o `CHANGELOG.md` já tem são histórico.
- O texto é lido por quem usa o app (notas do Release e janela "O que mudou" do gerenciador). A regra do tamanho e do sub-item "Técnico" está na #722.

## O que acontece com os arquivos

- O `go run ./cmd/versao -changelog` monta as seções do que ainda não foi publicado, uma por versão (a do merge que trouxe o arquivo): são as notas do pré-lançamento "desenvolvimento" e o "O que mudou" do gerenciador de quem atualiza pelo `develop`.
- Para publicar, na `release/X.Y.Z`, `scripts/dev/lancar-versao.sh X.Y.Z` junta todos os arquivos numa seção `## [X.Y.Z] — data` do `CHANGELOG.md`, por tipo, apaga os arquivos e faz o commit `chore: versão X.Y.Z`. Sem nenhum arquivo, ele recusa: escreva uma entrada antes.
