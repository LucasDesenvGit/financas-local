# 0055. Documentação de arquitetura em diagramas Mermaid, do HLD ao LLD

- **Status:** Aceita (#462, épico #465)
- **Data:** 2026-10-31
- **Relacionadas:** [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md) (a fase da migração que os diagramas mostram), [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md) (o gerenciador em migração)

## Contexto

`docs/ARQUITETURA.md` começava com um diagrama em texto (ASCII) que deixava de fora o gerenciador, o serviço do sistema, as chaves e o instalador. O resto era texto por pasta: bom para o detalhe, ruim para ver o todo. Nada mostrava onde cada parte roda no Windows e no Linux.

O épico #465 pede a arquitetura em desenho, da visão de cima (HLD) ao detalhe (LLD). Faltava decidir como desenhar, até onde ir e quem mantém o desenho em dia. As restrições: o repositório é público, o GitHub é onde a documentação é lida, nos temas claro e escuro, e o back-end está trocando de linguagem um assunto por vez.

## Alternativas consideradas

- **`flowchart` do Mermaid com a notação do C4 escrita nas caixas** (nome, `[Contêiner: Go]`, uma linha do que faz) e `subgraph` para as fronteiras: o texto fica no repositório, o diff mostra o que mudou, o GitHub desenha sem ferramenta à parte e segue o tema de quem lê. O custo: a disposição fica por conta do Mermaid, e as caixas não têm a cara padrão do C4.
- **Sintaxe própria do Mermaid para o C4** (`C4Context`, `C4Container`, `C4Deployment`): é a notação do C4, mas a [documentação do Mermaid](https://mermaid.js.org/syntax/c4.html) lista recursos inacabados, como legendas e links, que não serão suportados a curto prazo. Testada com o Mermaid 11.4 no diagrama de Contexto, com o tema escuro: usa cores fixas, os rótulos das setas ficam quase invisíveis no fundo escuro e um deles cai em cima de uma caixa. Descartada.
- **Imagens feitas em outra ferramenta** (draw.io, Structurizr): boa disposição, mas o diff não mostra o que mudou, a imagem não acompanha o tema, e quem altera o código precisa da ferramenta. Descartada.
- **Diagrama de Componentes (nível 3 do C4) já no HLD:** ficaria velho a cada PR da migração para Go. Vai para o LLD, só do Go, depois da remoção do .NET (#464).

## Decisão

Usamos **diagramas Mermaid em `flowchart`, com a notação do C4 escrita nas caixas**, versionados junto do código, porque é a opção que se lê nos dois temas e muda no mesmo PR que muda o sistema.

| | HLD | LLD |
|---|---|---|
| **Pergunta** | Quais são as partes, onde rodam e com quem falam? | Como uma parte funciona por dentro? |
| **O que tem** | C4 nível 1 (Contexto) e 2 (Contêineres), mapa de contextos, implantação e decisões em resumo | C4 nível 3 (Componentes, só do Go, #464), modelo de dados e fluxos em sequência (#463) |
| **Onde mora** | `docs/ARQUITETURA.md` | `docs/BANCO-DE-DADOS.md`, `docs/ARQUITETURA.md` ou `docs/arquitetura/` |

- O nível 4 do C4 (Código) fica de fora.
- Cada página de diagramas diz a data e a fase da migração, para quem lê saber de quando é o desenho.
- **Quem atualiza:** quem muda. Mudou um contêiner, uma saída de rede ou uma fronteira? Os diagramas de Contexto, de Contêineres e de implantação mudam no mesmo PR, como o Manual (#177). Passou um assunto para o Go? O mapa de contextos e a data também.

## Consequências

- **Positivas:** o desenho está no diff do PR e é revisado com o código. Quem chega vê o todo antes de abrir o código.
- **Negativas:**
  - a disposição é do Mermaid: em diagrama com muitas setas, é preciso agrupar. O mapa de contextos usa setas para a moldura, que dizem menos que uma seta por assunto;
  - o desenho só fica em dia se a regra for seguida: não há teste que compare o diagrama com o código;
  - os diagramas de Contêineres e de implantação mudam quando o .NET sair (#86).
- **Na prática:**
  - use `flowchart`, sem cor fixa nas caixas, para acompanhar o tema (o tracejado por `classDef` vale nos dois);
  - confira o desenho nos temas claro e escuro antes do PR;
  - as setas do mapa de contextos vêm do código: `go list -f '{{.ImportPath}} {{.Imports}}' ./internal/...` no Go e os construtores no C#;
  - o [modelo de ameaças](../MODELO-DE-AMEACAS.md) é revisado quando muda uma fronteira: saída de rede nova, processo, porta, pasta `dados`, login e a remoção do .NET (#86);
  - a regra está no `CLAUDE.md`, na skill `migrar-para-go` e no agente `dev-go`.
