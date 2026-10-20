# 0027. Conferência de investimentos: correções sugeridas pelo extrato, aplicadas só com confirmação

- **Status:** Aceita
- **Data:** 2026-10-17
- **Relacionadas:** [0006](0006-cliente-http-proprio-para-a-pluggy.md) (Pluggy), [0010](0010-reservas-estimadas-pelo-extrato.md) (estimar pelo extrato, conferir à mão), [0023](0023-edicao-em-massa-no-modo-edicao-de-transacoes.md) (desfazer guardado na API)

## Contexto

Na carteira real, o total de FIIs no site ficava cerca de R$ 3 mil abaixo do que a corretora informava. Três causas, todas vindas do Open Finance do Nubank:

1. **FII entrava como ação.** O Nubank manda FII com o subtipo `STOCK`, e o GARE11 veio sem emissor.
2. **Compras fora do histórico.** A posição da corretora tinha 224 cotas de GARE11, mas o histórico de movimentações (`/investments/{id}/transactions`) não trazia a compra de 15/06. O mesmo aconteceu com XPLG11 e XPML11. O extrato tinha os três débitos "Compra de Renda Variável".
3. **Créditos da corretora todos com a mesma descrição.** O Nubank debita um pouco a mais na compra e devolve a sobra no mesmo dia ou nos seguintes. Esse troco, o valor de vendas e os proventos chegam todos como "Valor recebido de Investimentos". A regra padrão leva tudo para Rendimentos: o troco (R$ 138,60 em 32 créditos) contava como receita.

O usuário pediu um botão que corrija esse tipo de coisa.

## Alternativas consideradas

- **Corrigir sozinho na sincronização.**
  - Prós: nada para o usuário fazer.
  - Contras: a separação entre troco e provento é por estimativa, e um erro some com uma receita sem ninguém ver; o usuário não fica sabendo do que mudou.
- **Ligar cada crédito ao provento esperado** (cotas na data-com × valor por cota).
  - Prós: mais preciso.
  - Contras: depende da data de pagamento, que a fonte de cotações nem sempre informa; não resolve compras que faltam nem venda.
- **Só o passo a passo no manual.**
  - Prós: nenhum código.
  - Contras: o usuário errou na primeira tentativa (mudou todos os créditos para Investimentos, e os proventos deixaram de contar como receita).
- **Conferência sob demanda, com prévia marcada e "Desfazer".**
  - Prós: as estimativas aparecem como sugestão; o usuário vê, desmarca o que não for, e pode voltar atrás.
  - Contras: a pessoa precisa lembrar de abrir. Por isso a tela avisa quando as cotas não batem.

## Decisão

Em Investimentos, **"Conferir investimentos"** abre uma prévia (`GET /api/carteira/conferencia`) com cinco grupos, cada correção já marcada.

**Os grupos (`ConferenciaInvestimentosService`):**
- **FII cadastrado como outro tipo:** pelo nome do fundo ou pelo emissor (`ClassificacaoAtivo.PareceFii`).
- **Cotas diferentes da corretora:** quando faltam cotas, sugere o débito de compra sem operação cujo valor, sem o troco, dividido pelas cotas que faltam, fica até 3,5% do fechamento do dia. A compra é lançada ligada a esse débito (`Origem = Extrato`, `TransacaoId`).
- **Troco ou venda contando como rendimento:** vai para a categoria de transferência "Investimentos", como escolha manual.
- **Taxa da B3 que falta nas compras (#173 a):** em cada dia em que todos os débitos de compra têm operação, nenhuma operação tem taxa e o dia tem pelo menos 5 dias (o troco já voltou), a taxa é débitos − trocos − cotas × preço. Só é sugerida entre R$ 0,01 e o menor de 1% do valor e R$ 5; é dividida pelas operações do dia pelo valor de cada uma. O valor é recalculado na API ao aplicar (a tela manda só as operações), e o "Desfazer" volta a taxa anterior. A taxa fica no custo e no preço médio (o "valor final da compra" do app: na conferência com a carteira real, valor enviado − valor devolvido − cotas × preço deu, ordem a ordem, a "taxa da bolsa" que o app mostra); o mês a mês dos FIIs continua com cotas × preço, porque as "Entradas" do app não somam a taxa (março e junho batem ao centavo sem ela).
- **Rendimento contando como investimento:** volta para a categoria automática. Só entram créditos com a mesma descrição dos trocos, e só se as regras derem uma categoria de receita.

**Como o extrato é lido:**
- **Débito × ordem:** cada débito "renda variável" é ligado à operação (ou às execuções do mesmo ativo e dia) cujo custo fica logo abaixo dele (de −0,2% a +4%). As operações podem estar até 3 dias antes ou 1 dia depois do débito.
- **Um troco por compra:** os créditos da corretora do dia até 3 dias depois, primeiro os de data mais próxima e, no mesmo dia, os menores, até 2,5% do débito.
- **Sobra de centavos:** créditos até 5 dias depois, enquanto a soma dos trocos do dia não passar do que os débitos tiveram a mais que o custo.
- **Venda:** crédito com o valor da venda (até 2% ou R$ 0,05 de diferença), até 5 dias depois do pregão.

**Aplicar e desfazer:**
- `POST .../corrigir` grava tudo numa gravação só.
- O estado anterior fica na memória da API por 10 minutos para o "Desfazer" (`POST .../desfazer/{chave}`), como na ADR 0023.
- A tela de Investimentos avisa no topo quando a posição da corretora tem outra quantidade de cotas (`CarteiraDto.CotasDiferentes`).
- **Aviso no Painel (#170):** as compras novas trazem trocos novos, que voltavam a contar como receita até alguém lembrar de abrir a conferência. O Painel conta os trocos e vendas ainda como receita (`CreditosComoReceitaAsync`, a mesma análise da conferência) e avisa, com o link `/investimentos?conferir=1`. Rodar a correção sozinha depois da sincronização foi descartado: contrariaria a prévia desta ADR, e um provento menor que o troco do mesmo dia seria mudado sem ninguém ver.

**Na mesma linha (PRs anteriores, sem ADR própria):**
- O ativo que vem do Open Finance como `STOCK` vira FII pelo emissor ou pelo nome oficial.
- Lançamento seu num ativo que já tem histórico da corretora só completa esse histórico, sem travar a importação.

## Consequências

- **Positivas:**
  - Com os dados reais, a regra separou exatamente os R$ 561,04 de rendimentos que a corretora informa: 32 créditos de troco ou venda, somando R$ 138,60, de um total de R$ 699,64. Cada dia de compra fecha com a sobra dos débitos menos a taxa da B3 (~0,03%).
  - As correções viram uma lista que dá para revisar e desfazer (Nielsen 3 e 5), em vez de mudanças silenciosas.
- **Negativas:**
  - **As regras são estimativas** calibradas no Nubank (*inferido* para outras corretoras): descrições "renda variável" e "invest", troco menor que o provento do mesmo dia. Um provento menor que o troco, no dia de uma compra, pode ser marcado como troco. Por isso nada é aplicado sem a prévia.
  - Precisa de uma categoria de transferência; sem ela, o grupo do troco aparece travado.
  - A sugestão de compra consulta o fechamento diário no Yahoo (a mesma saída de rede da importação pelo extrato). Sem resposta, usa o fechamento do mês guardado no banco, que é menos preciso.
  - O "Desfazer" se perde se a API reiniciar nesses 10 minutos.
- **Na prática:**
  - Corretora nova com outra descrição: ajuste `EhCompra` e `EhCreditoDaCorretora` em `ConferenciaInvestimentosService`, e confira com dados reais.
  - Os limites (2,5%, 4%, 3,5%, dias) são constantes no topo do serviço, com o porquê de cada um.
