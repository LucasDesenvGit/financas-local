# 0016. Ícones do Lucide empacotados no build

- **Status:** Aceita
- **Data:** 2026-10-16 (registrada depois da decisão, no PR #29)
- **Relacionadas:** [0004](0004-front-end-publicado-no-wwwroot-da-api.md) (front-end na API, CSP restritiva)

## Contexto

Menu, botões e cartões usavam caracteres de texto como ícone (⟳, 👁, ☰), que mudam de aparência entre sistemas e não combinam entre si (issue #26). A CSP do projeto não permite carregar nada de terceiros no navegador, então fonte de ícones por CDN estava fora.

## Alternativas consideradas

- **Continuar com caracteres/emoji** — zero dependência; visual inconsistente entre Windows, Linux e celular.
- **Fonte de ícones por CDN (Material Icons, Font Awesome)** — muito usada; quebra a CSP e cria saída de rede no front-end.
- **SVGs desenhados à mão no projeto** — sem dependência; trabalhoso de manter e sem consistência de traço.
- **`lucide-angular` (licença ISC)** — biblioteca de ícones em SVG, só os usados entram no bundle (tree-shaking), sem acesso à rede.

## Decisão

Usamos `lucide-angular` na versão **fixa** `1.0.0`, empacotado no build. Os ícones usados ficam registrados num lugar só (`compartilhado/icones.ts`) e são desenhados pelo componente `app-icone`, decorativo (`aria-hidden`) e na cor do texto. O nome do botão continua no `aria-label`/texto, não no ícone.

## Consequências

- **Positivas:** visual consistente; nada baixado de fora (CSP inalterada); trocar um ícone é uma linha em `icones.ts`.
- **Negativas:**
  - dependência de terceiros no front-end; o pacote `lucide-angular` foi marcado como descontinuado em favor de `@lucide/angular` (o `npm ci` já avisa) — em algum momento será preciso migrar;
  - versão fixa: atualizações (inclusive de segurança) dependem de alguém subir a versão à mão;
  - ícone sem texto exige `aria-label` e dica (`appDica`) em cada botão — esquecer deixa o botão sem nome para leitor de tela.
- **Na prática:** não importar ícone direto do `lucide-angular` nas telas — registrar em `icones.ts` e usar `<app-icone>`. Botão só com ícone leva `aria-label` + `appDica` com o mesmo texto.
