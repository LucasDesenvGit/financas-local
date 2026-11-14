# 0067. Notícias: G1, BBC News Brasil e TabNews pelo site, só com o módulo ligado

- **Status:** Proposta
- **Data:** 2026-11-14
- **Relacionadas:** [0064](0064-modulos-compilados-no-executavel-e-ligados-no-banco.md), [0048](0048-cotacao-de-criptomoedas-pelo-mercado-bitcoin.md)

## Contexto

A #10 pede as notícias do dia no site: G1 (geral), BBC News Brasil (política) e TabNews (tecnologia). São três **saídas de rede novas**, para fora do assunto do site (finanças). O épico dos módulos (#190, ADR 0064) já decidiu que uma visualização assim é um **módulo**, desligado por padrão quando acessa a internet; esta ADR decide como as notícias chegam e o que pode sair.

Restrições: a CSP do site não deixa o navegador buscar nada de fora (nem imagem); tudo o que sai é por HTTPS e com o mínimo (`docs/SEGURANCA.md`); uma fonte que falha não pode derrubar as outras.

## Alternativas consideradas

- **O navegador busca os feeds:** a CSP bloqueia, e abri-la para três domínios de notícia é ruim. Rejeitada.
- **Buscar a cada vez que a tela abre:** depende da internet na hora e pode estourar o limite das fontes. Rejeitada.
- **O site busca de tempos em tempos e guarda** (escolhida): uma tarefa do módulo a cada 45 minutos, com a lista guardada no banco; a tela mostra o que está guardado.
- **Mostrar a imagem das notícias:** precisaria baixar e servir as imagens pelo site (a CSP não deixa carregar de fora). Rejeitada por agora: só o título, a fonte, o horário e o link.

## Decisão

- **Módulo `noticias`** (ADR 0064): `Destinos` = `g1.globo.com`, `feeds.bbci.co.uk` e `www.tabnews.com.br`; nasce **desligado**. Desligado, a tarefa não roda (nada sai) e as rotas respondem 409.
- **As fontes:**
  - G1: o RSS geral, `https://g1.globo.com/rss/g1/`;
  - BBC News Brasil: o RSS `https://feeds.bbci.co.uk/portuguese/rss.xml`. *Inferido:* não há um feed só de política da BBC News Brasil que se confirmou; fica o geral, e a tela não diz "política";
  - TabNews: a API pública `https://www.tabnews.com.br/api/v1/contents?strategy=relevant&per_page=20`.
- **O que sai:** só o pedido do feed, com `User-Agent: FinancasLocal/1.0` e nada do usuário (sem cookie, sem identificador).
- **Defesas na entrada:** só HTTPS, host fixo no código e redirecionamento só para o mesmo host; tempo limite de 15 s; resposta de no máximo 2 MB; o link de cada notícia só é guardado se for `https://` num domínio da fonte (`g1.globo.com`, `www.bbc.com`/`www.bbc.co.uk`, `www.tabnews.com.br`), o que barra `javascript:` e endereços estranhos; o título é texto (o Angular escapa).
- **Guardar:** a tabela `Noticia` (a fonte, o título, o link e a data de publicação), com as 20 mais novas de cada fonte, e `NoticiaFonte` (quando cada fonte foi atualizada e o último erro). Uma fonte que falha guarda o erro e deixa as outras como estão.
- **Na tela:** a lista com o título, a fonte e o horário, e o link abre fora do site (`target="_blank"`, `rel="noopener noreferrer"`). "Atualizar agora" busca na hora, no máximo uma vez a cada 5 minutos.

## Consequências

- **Positivas:** as notícias no mesmo lugar das finanças, sem abrir a CSP, e nada sai enquanto o módulo está desligado.
- **Negativas:**
  - Três saídas de rede novas, com o assunto de fora das finanças: a tabela "O que sai para a internet", o README e o modelo de ameaças crescem.
  - As fontes mudam o formato sem avisar: o leitor pode parar de achar notícias (o erro aparece por fonte, em Configurações › Erros e na tela).
  - As fontes ficam sabendo o IP de quem pede, a cada 45 minutos, com o módulo ligado.
- **Na prática:** pacotes `internal/modulos/noticias` (o módulo, as rotas, o cache e a tarefa) e `internal/modulos/noticias/fontes` (os clientes e a leitura dos feeds). Fonte nova é uma entrada na lista de fontes, com o host, o leitor e o domínio dos links, e uma linha nesta ADR e na tabela de `docs/SEGURANCA.md`.
