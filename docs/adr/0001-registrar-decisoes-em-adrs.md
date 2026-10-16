# 0001. Registrar decisões de arquitetura em ADRs

- **Status:** Aceita
- **Data:** 2026-10-16

## Contexto

As decisões técnicas do projeto estavam espalhadas entre `README.md`, `CLAUDE.md`, `docs/ARQUITETURA.md` e mensagens de commit, quase sempre registrando **o que** foi feito e raramente **por quê** nem quais alternativas foram descartadas. Na migração para SQLite a pasta `docs/` chegou a ser apagada por engano, e os links para ela ficaram quebrados sem ninguém perceber.

Parte do código é escrita com o Claude Code, que só respeita uma decisão se ela estiver escrita num lugar que ele consulta.

## Alternativas consideradas

- **Continuar só com `ARQUITETURA.md`** — um arquivo só, fácil de achar; mas mistura descrição do estado atual com justificativa, e o histórico da decisão se perde a cada edição.
- **Wiki ou Confluence** — melhor busca e edição; mas fica fora do repositório, não acompanha o pull request da mudança e não é lido pelo Claude Code.
- **ADRs em Markdown no repositório** — um arquivo por decisão, versionado junto com o código.

## Decisão

Registramos cada decisão de arquitetura como um ADR em `docs/adr/`, no formato de [0000-modelo.md](0000-modelo.md), incluído no mesmo pull request da mudança. O `CLAUDE.md` instrui o Claude Code a verificar as ADRs antes de mudanças desse tipo e a criar ou atualizar a correspondente.

## Consequências

- **Positivas:** o motivo de cada escolha fica junto do código e revisável no PR; decisões substituídas continuam legíveis.
- **Negativas:** mais um artefato para manter; ADR desatualizada engana mais que ADR inexistente, então a revisão do PR precisa conferir.
- **Na prática:** `docs/ARQUITETURA.md` descreve o estado atual e aponta para as ADRs; `docs/adr/README.md` define quando escrever uma e mantém o índice; o checklist do PR em `CONTRIBUINDO.md` pergunta pela ADR.
