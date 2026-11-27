# changelog.d: uma entrada por PR

Cada PR `feat` e `fix` põe aqui **um arquivo novo**, `changelog.d/<nome-da-branch>.md` (a `/` vira `-`: `feature/relatorio-anual` é `feature-relatorio-anual.md`); a CI confere (`scripts/ci/versao/verificar-changelog.py`). Dois PRs, dois arquivos: nunca conflitam ([ADR 0070](../docs/adr/0070-versao-calculada-no-build-e-changelog-um-arquivo-por-pr.md)). CI, testes, documentação interna, refatoração e Dependabot podem não ter: as notas dos Releases os listam pelo título.

## O formato

Um título de tipo, uma só vez cada, com pelo menos uma entrada: `### Funcionalidades`, `### Correções` ou `### Publicação e documentação`.

```markdown
### Correções
- **O que mudou, em poucas palavras** (#123). O que muda para quem usa, em uma ou duas frases.
  - Técnico: o que só importa para quem mexe no código (rota, coluna, ADR), se importar.
```

- **Para quem usa, até ~60 palavras** por entrada, sem nome de coluna, rota ou ADR. Passou disso, a CI avisa (não barra).
- **`  - Técnico: ...`** é opcional, numa linha só, recuada com dois espaços. O gerenciador e as notas dos Releases a escondem; ela fica no `CHANGELOG.md`.
- **Sem versão** (`## [X.Y.Z]`): ela é calculada no merge (`go run ./cmd/versao`, em `src/backend`). Não mexa no `CHANGELOG.md`.

## Na publicação

O `go run ./cmd/versao -changelog` monta as seções do que ainda não foi publicado (notas do pré-lançamento "desenvolvimento" e "O que mudou" de quem atualiza pelo `develop`). Na `release/X.Y.Z`, o `scripts/dev/lancar-versao.sh X.Y.Z` junta os arquivos numa seção `## [X.Y.Z] — data` do `CHANGELOG.md`, por tipo, apaga os arquivos e faz o commit `chore: versão X.Y.Z`; sem arquivo, recusa.
