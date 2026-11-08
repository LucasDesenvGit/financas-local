---
name: revisar-no-celular
description: Confere uma mudança de tela do Finanças Local no celular (#328, ADR 0061) — largura sem rolagem para o lado, toque (alvo de 24 px, nada que dependa de passar o mouse), teclado virtual (tipo do campo, campo que não some atrás do teclado) e texto comprido que quebra. Roda no projeto "celular" do Playwright e, com o acesso pelo celular ligado, num celular de verdade. Usar em toda tarefa que muda tela, CSS ou componente do front-end, junto da testar-no-navegador, e quando pedirem "confere no celular" ou "como fica no celular".
---

# Revisar no celular (Finanças Local)

A `testar-no-navegador` confere que a tela abre sem erro, no computador e no celular. Esta skill olha o que só aparece no celular: o dedo no lugar do mouse, a tela estreita e o teclado virtual. Ela vem **depois** da `testar-no-navegador`, na mesma tarefa, e não a substitui.

Tudo roda em `src/frontend`, com o build de produção e a API falsa (nenhum dado real), como na `testar-no-navegador`. O projeto "celular" do Playwright é um Pixel 7 (412 px de largura, toque, `isMobile`).

## 1. Os testes da tela no celular

```bash
NAVEGADOR=chromium npx playwright test testes/navegador/<tela>.spec.ts --project=celular
```

Teste novo da tela: `--repeat-each=5`, nos dois projetos (regra do `CLAUDE.md`). O que só vale no celular usa `test.skip(({ isMobile }) => !isMobile, 'só no projeto de celular')`, como o `celular.spec.ts`.

## 2. Prints na largura do celular

Tire os prints da tela mudada no projeto "celular", claro e escuro, com um spec temporário (`zz-recorte-temp.spec.ts`, apagado depois). O caminho do print é do Windows (`C:\\...`) ou relativo: um caminho do Git Bash (`/c/...`) vira uma pasta `D:\c` no Node.

Olhe cada print. **Não diga que conferiu sem olhar.**

## 3. O que conferir, item por item

Cada item diz de onde vem. Escreva no fim quais conferiu e como.

| O que | Como conferir | Fonte |
|---|---|---|
| **Sem rolagem para o lado** | `document.documentElement.scrollWidth - window.innerWidth <= 1` na tela mudada; texto comprido sem espaço (URL, IP, impressão digital, nome de fundo) com `overflow-wrap: anywhere` | WCAG 2.2, 1.4.10 *Reflow* |
| **Alvo de toque ≥ 24 px** | `boundingBox()` de cada botão, link e caixa novos: altura e largura `toBeGreaterThanOrEqual(23.5)` (a folga de 0,5 da CI) | WCAG 2.2, 2.5.8 *Target Size (Minimum)* |
| **Nada só no hover** | Dica, menu ou botão que só aparece ao passar o mouse precisa de um toque ou de foco que mostre o mesmo; `title=` não serve | WCAG 2.2, 1.4.13 *Content on Hover or Focus* |
| **Teclado certo** | Valor em dinheiro com o `<app-campo-valor>` (`inputmode="decimal"`), data com o `<app-campo-data>`, e-mail com `type="email"`, senha com o `autocomplete` certo | Nielsen 5 (prevenção de erro) e 7 (eficiência) |
| **Campo visível com o teclado aberto** | Na janela ou formulário que abre embaixo, o campo focado não fica atrás do teclado (a janela rola, ou o campo sobe); no Playwright, `focus()` e conferir que o campo está dentro da `viewportSize()` menos ~40% da altura | *julgamento* (o teclado do emulador não aparece no Playwright) |
| **Fonte do campo ≥ 16 px** | Abaixo disso o Safari do iPhone dá zoom ao focar | *julgamento*, comportamento conhecido do iOS, não validado aqui |
| **O toque rola a página** | Componente que fecha ao rolar aguenta a rolagem do próprio toque (o calendário fechava antes de aparecer, #264) | `CLAUDE.md` |
| **Nada fixo cobrindo conteúdo** | Cabeçalho ou barra fixa não esconde o fim de uma janela nem o botão de salvar | Nielsen 8 |

## 4. Num celular de verdade (quando der)

O Playwright não tem o teclado virtual, o zoom do iPhone nem o dedo. Com o **acesso pelo celular** ligado (Configurações › Segurança, ADR 0061) e a versão nova publicada no computador da pessoa:
1. peça para ela abrir a tela no celular (o endereço `https://<IP>:5443` está no quadro "Acesso pelo celular");
2. diga o que olhar: os itens da tabela que o Playwright não confere (teclado, zoom, toque);
3. só escreva "conferido no celular" com a resposta dela.

Sem o acesso ligado, ou sem a pessoa por perto: diga que **não foi conferido num celular de verdade**, e quais itens ficaram só no emulador.

## 5. Como entregar

No fim da tarefa, junto do que a `testar-no-navegador` diz:
- os testes que rodaram no projeto "celular" (e se repetiu 5 vezes o teste novo);
- os prints que olhou;
- a tabela da seção 3 com ✔, ✘ ou "não se aplica" em cada linha;
- se foi ou não conferido num celular de verdade.
