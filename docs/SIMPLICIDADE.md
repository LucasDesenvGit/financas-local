# Simples e enxuto

A régua do projeto contra o over-engineering e o texto longo (#769). O `CLAUDE.md` traz o resumo; aqui ficam o detalhe e os casos reais.

**Por que existe:** entre 11 e 18/11/2026, a #689 (over-engineering) e a #690 (documentação) cortaram código e texto, e outra frente, sem conhecer a régua, acrescentou mais do que foi cortado. Uma regra que só está numa issue não chega a uma sessão nova.

## A régua

É um app local, de um usuário só, mantido por poucas pessoas. Uma peça nova precisa se pagar **nesse** tamanho.

| Antes de criar | Pergunta |
|---|---|
| Interface, camada, pacote | Já há um segundo uso real, fora do teste? Senão, função comum no pacote de quem usa. |
| Código parecido com outro | Já existe em outro pacote? Reuse ou mova para um lugar só (ex.: tabelas do INSS e do IRRF no `encargos`, leitura do corpo no `respostas`). |
| Dependência ou ferramenta | O que ela faz que poucas linhas não fazem? Quanto custa na CI? |
| Check de CI | Que falha de verdade ele pega? Um aviso não basta? |
| Texto | Já está escrito em outro lugar? Então só o link. |

## Limites do texto

- **ADR:** até ~1,5 mil palavras, só contexto, decisão, alternativas e consequências. Medições, passo a passo e "como ficou" vão no PR.
- **README de pasta:** de 10 a 20 linhas: o que é, como rodar, e o link para o `docs/ARQUITETURA.md` (#657).
- **Entrada do CHANGELOG (`changelog.d/`):** até ~60 palavras, o que muda para quem usa; detalhe técnico à parte (#722).
- **Manual do site:** frase curta e voz de quem usa; "como o site calcula por dentro" vira uma dica curta.
- **Tirar não é mover:** ao encurtar um documento, o histórico some (está no git); só a regra que ainda vale vai para o guia certo.

## Casos reais (para reconhecer)

- `sqlc` gerando 48 structs, das quais 1 era usada (#771).
- O mesmo trecho que lê as tabelas do INSS e do IRRF repetido em 6 pacotes do Trabalho (#772).
- A CI escolhendo os testes afetados do front-end sem poupar tempo (#774).
- Texto tirado de uma ADR e colado inteiro num README: o do gerenciador foi de 981 para 3.775 palavras (#779).
- Entradas do CHANGELOG com mediana de 142 palavras, lidas por quem usa o app (#722).

## Como medir

`wc -w` nos arquivos de texto; `go run golang.org/x/tools/cmd/deadcode@latest ./...` em `src/backend`; o tempo dos jobs com `gh run view <id> --json jobs`. O PR que corta ou acrescenta mostra o antes e o depois.
