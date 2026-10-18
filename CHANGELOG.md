# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e [versionamento semântico](https://semver.org/lang/pt-BR/).

## [Não publicado]

### Funcionalidades
- **Listas suspensas no visual do site:** o campo fechado ganhou uma seta própria, no lugar da caixa do sistema, e fica numa linha só. A lista que abre continua a do navegador, e o teclado, o leitor de tela e o celular funcionam como antes.
- **O que aparece depois entra com um fade curto,** em vez de surgir de uma vez: botões, avisos, etiquetas, cards, blocos, menus que abrem, listas vazias e mensagens de erro. Nada anima com "reduzir movimento" no sistema.
- **Caixa de marcar nova e seleção de várias linhas melhor** (#133, ADR 0032).
  - **Caixa nova em todo o site:** no visual do site, nos temas claro e escuro, com os estados marcada, parcial (traço) e desabilitada.
  - **Nas tabelas:** a célula inteira é clicável e a linha selecionada fica destacada (Transações, Investimentos › importar do extrato, Sugestão de orçamentos, Conferir investimentos).
  - **Em Transações:**
    - Shift+clique seleciona um intervalo; Esc limpa;
    - "Selecionar todas desta página" mostra o estado parcial;
    - a barra diz "5 selecionadas (2 em outras páginas)", abre com animação, sem empurrar a tabela de uma vez, e fica fixa no topo ao rolar.
  - **Trocar de página mantém a seleção;** trocar o filtro limpa, com o aviso "Seleção limpa ao trocar o filtro".
  - **Leitor de tela:** cada caixa diz qual é a transação ("Selecionar: PIX ENVIADO JOAO, 12/10/2026, R$ 150,00"), e a contagem é anunciada.
- **Campo de valor em reais padrão** (#132, ADR 0032), em todos os campos de dinheiro: Transações, Contas e cartões, Contas a pagar, Metas, Orçamentos, Investimentos e Conferir investimentos.
  - **Entende o jeito brasileiro:** "1.234,56", "1234,56", "R$ 1.234,56" (colado do app do banco) e "1234.56". "1.500" é mil e quinhentos, não 1,5.
  - **Formata enquanto você digita:** "124214" já aparece como "124.214", com o cursor no lugar. Ao sair do campo, completa as casas ("124.214,00").
  - O símbolo da moeda fica dentro do campo. Investimentos aceitam outra moeda e até 4 casas.
  - **No celular**, abre o teclado numérico. **No computador**, a rodinha do mouse não muda mais o valor, e as setinhas saíram.
  - **Erro em texto, perto do campo:** "Informe um valor maior que zero.".
  - **Orçamentos:** continua salvando ao sair da linha, e agora também com Enter.
  - Os valores enviados à API não mudam.
- **Interruptor para o que muda a tela na hora** (#130, ADR 0032).
  - "Mostrar arquivadas" (Contas e cartões), "Mostrar encerradas" (Investimentos › Posições) e "Agrupar subcategorias" (Relatórios) viraram interruptores. As caixas de marcar de formulário, com "Salvar", não mudam.
  - **"Mostrar arquivadas":**
    - o estado fica na URL (`?arquivadas=1`), e recarregar, voltar ou abrir um link salvo mostra a tela do mesmo jeito;
    - conta ou cartão arquivado mostra o selo **"Arquivada"** em texto.
  - Os filtros de Transações continuam caixas de marcar, por consistência com a barra de filtros (o porquê está na ADR 0032).
- **Esconder o menu lateral no computador** (#131).
  - **Botão ☰:** o mesmo botão da barra de cima, que antes só aparecia no celular, agora também esconde e mostra o menu no computador. O conteúdo passa a usar a largura toda, e os gráficos se redesenham sozinhos.
  - **Nome acessível:** fixo, "Menu de navegação", e o leitor de tela ouve pelo `aria-expanded` se o menu está aberto ou fechado.
  - **Menu escondido:** sai da ordem do Tab (`inert`), tanto no computador quanto com a gaveta do celular fechada.
  - **A escolha fica salva neste navegador** (`financas.menuRecolhido`, ADR 0017), e o padrão é o menu aberto. No celular a gaveta continua começando fechada.
  - **Com o menu escondido, a página ganha a largura do menu** (até 1632 px) e fica **centralizada**, sem encostar à esquerda com um vazio grande à direita em tela larga.
  - **Animação:**
    - ao esconder, o menu desliza enquanto a coluna encolhe;
    - ao mostrar, a coluna abre de uma vez e o conteúdo, o ☰ e o título deslizam junto com o menu, só com `transform` (sem o pisca-pisca de refazer o layout a cada quadro);
    - nada disso acontece com movimento reduzido no sistema.
  - **Gaveta do celular:** dá para fechar sem escolher uma tela. As formas são:
    - o **✕ "Fechar menu"**;
    - **tocar no fundo escuro**;
    - **arrastar para a esquerda**;
    - **Esc**, que também devolve o foco ao ☰.

    A gaveta ocupa no máximo 85% da tela. Aberta, o foco vai para o ✕ e o conteúdo atrás sai do Tab.
- **A marca "Finanças" leva ao Painel:** no topo do menu e no título da barra de cima (que aparece no celular e com o menu escondido). No celular, clicar nela também fecha a gaveta.
  - O título da barra tem o **cofrinho** da marca e entra e sai com fade.
- **Transição entre telas:** a tela nova entra com um fade curto e sobe 8 px.
  - Só ao trocar de tela; filtros, página da tabela e âncoras do manual não animam.
  - Sem animação com movimento reduzido no sistema.
- **Gerenciador também na instalação pelos scripts** (Windows, ADR 0028).
  - **Como instalar:** `scripts\instalar-gerenciador.ps1` publica o gerenciador em `publicado\gerenciador` e cria o atalho no Menu Iniciar, sem administrador.
  - **Como acha o site:** pelo serviço `FinancasLocal`, e o banco pelo `appsettings` da instalação.
  - **Versão nova:** nessa instalação, ele não oferece o `.msi` e diz como atualizar pelos scripts.
- **Visual novo do gerenciador e do instalador**, com a identidade do site.
  - **Gerenciador:** cabeçalho com a marca, cartões, botões arredondados e um ponto colorido com a situação. Segue o tema claro ou escuro do Windows, inclusive na barra de título. As preferências viraram interruptores (liga/desliga), e o botão em destaque é a próxima ação provável: "Abrir o site" quando está no ar, "Ligar" quando está desligado.
  - **Instalador `.msi`:** imagens com o degradê da marca e o cofrinho nas telas de boas-vindas, de conclusão e no topo das demais (`instalador/imagens`).
- **Instalador `.msi` do `develop`, com link fixo** (ADR 0029). A cada merge no `develop`, depois dos testes de instalação, a CI publica o `FinancasLocal-desenvolvimento.msi` no pré-lançamento [desenvolvimento](https://github.com/LucasDesenvGit/financas-local/releases/tag/desenvolvimento).
  - **Versão mostrada:** `X.Y.Z-dev.N`, no site e no gerenciador.
  - **Troca entre as duas:** o `.msi` de mesma versão `X.Y.Z` substitui o instalado, e a versão publicada e a de desenvolvimento trocam uma pela outra sem ficarem as duas instaladas. A CI testa essa troca.
  - **Avisos do gerenciador:** continuam só para as versões publicadas.
- **Mais fácil sair da instalação pelos scripts:** o gerenciador dessa instalação diz que o `.msi` traz o banco e as chaves e tem o botão **Baixar o instalador .msi**.
- **Atalho do gerenciador na área de trabalho**, para abrir com um clique.
  - **`.msi`:** o atalho vai para a área de trabalho de todos os usuários. Com `ATALHOAREADETRABALHO=0`, instala sem ele. A desinstalação remove o atalho.
  - **`instalar-gerenciador.ps1`:** o atalho vai para a área de trabalho do usuário. Com `-SemAtalhoNaAreaDeTrabalho`, não cria.

### Correções (layout)
- **Painel: cards do mês reorganizados.**
  - **Antes:** Receitas, Despesas e Resultado esticavam até a altura de "Faturas do mês" (que lista os cartões), com o valor solto no meio de um card alto e vazio.
  - **Agora:** os três ficam em colunas iguais e com a mesma altura, com o valor logo abaixo do título. "Faturas do mês" vai para uma linha própria, na largura toda, e os cartões dividem essa largura lado a lado (2 cartões = metade cada).
  - **No celular**, tudo fica numa coluna.
- **Divisórias em todas as telas.** Linhas finas nos dois temas:
  - abaixo do cabeçalho de cada página;
  - entre as seções de uma tela ("Situação hoje" e "Mês de …" no Painel; "Contas" e "Cartões de crédito" em Contas e cartões);
  - abaixo do título de cada bloco: os de `.bloco-titulo`, os de Investimentos e FIIs e os que começam com o título solto (Visão anual, Relatórios, Projeção, Configurações, Aparência, Parametrização, Manual…);
  - no Painel, em "Faturas do mês", abaixo do total e entre os cartões.
- **Contas e cartões: cards do mesmo tamanho e alinhados.**
  - **Cartões de crédito:** os da mesma linha têm a mesma altura, com "transações · editar" sempre no rodapé. Abrir o histórico de faturas de um cartão também estica os vizinhos da linha.
  - **Contas:** cabeçalho, saldo, "saldo atual" e rodapé ficam na mesma altura em todos os cards da linha, mesmo quando um nome de banco longo quebra em várias linhas.
- **Campos de data com ano de 4 dígitos.**
  - **Antes:** o calendário do navegador aceitava ano com até 6 dígitos ("13/02/275760").
  - **Agora:** todo campo de data vai de 01/01/1900 a 31/12/2099, ou usa o limite da própria tela (ex.: não aceitar data no futuro em Investimentos). Assim o navegador limita o ano a 4 dígitos.
  - Data fora do intervalo não deixa salvar.
- **Lista vazia numa grade de cartões** (ex.: Metas sem nenhuma meta): o aviso ocupa a linha toda, numa linha só e centralizado. Antes ficava preso na primeira coluna, quebrado em duas linhas e fora do meio.

## [2.1.0] — 2026-10-18

Primeira versão com o instalador `.msi` para Windows e o Gerenciador do Finanças Local.

### Funcionalidades
- Transações: **renomear com um nome claro, também para as próximas** (#123, ADR 0030).
  - **Botão ✎ "Renomear"** em cada transação. A janela mostra a descrição do banco e oferece "Só nesta transação" ou "Também nas outras com esta descrição do banco e nas próximas que chegarem".
  - **Regra:** a descrição do banco "for igual a", "contiver" ou "começar com" um texto. Para "contiver", o site sugere o texto sem os números que mudam a cada mês.
  - **Antes de salvar:** prévia com quantas transações vão mudar. Depois, aviso com **Desfazer** e "Voltar à descrição do banco".
  - **Na lista**, o nome novo fica em cima e a **descrição do banco embaixo**, menor ("do banco: …").
  - **Busca** pelos dois nomes. A categoria continua vindo da descrição do banco.
  - **Em massa:** "✎ Renomear" na barra de seleção dá o mesmo nome às selecionadas. Pode ser só nelas, ou com uma regra "é igual a" por descrição do banco, que vale nas outras iguais e nas próximas. O aviso tem "Desfazer".
  - **Onde ficam as regras:** Categorias › Regras automáticas › "Regras de descrição". Lá dá para criar a regra direto com "+ Regra de descrição" (com prévia de quantas transações renomeia), editar, desligar e excluir. Nome escolhido à mão nunca é trocado por regra.
  - **Banco:** tabela nova `RegraDescricao` e colunas `Transacao.RegraDescricaoId` e `Transacao.DescricaoManual` (rodar `scripts/2-criar-banco.sh`).
- Transações: o **valor do que está "fora dos gastos"** (pagamento de fatura, transferência) aparece em cor neutra, para não parecer receita ou despesa.
- **Instalador `.msi` para Windows** (#119, ADR 0029).
  - **Instalação:** dois cliques e "Avançar", sem .NET, Node, Git nem terminal. O site e o gerenciador já vêm compilados para `C:\Program Files\FinancasLocal`.
  - **Dados:** ficam em `C:\ProgramData\FinancasLocal\dados`, só para o sistema e os administradores.
  - **O que o instalador configura:** serviço `FinancasLocal` com reinício automático, certificado HTTPS confiável para `127.0.0.1` e `localhost`, atalhos no Menu Iniciar e, no fim, uma caixa para abrir o gerenciador.
  - **Atualizar:** pelo gerenciador ou abrindo o `.msi` novo; os dados ficam.
  - **Desinstalar:** mantém os dados (`APAGARDADOS=1` apaga também).
  - **Migração:** quem instalou pelos scripts é migrado sozinho. Banco e chaves são copiados, a pasta antiga fica.
  - **Workflow "Versão (instalador .msi)":** gera e testa o pacote num Windows limpo (instalar, atualizar mantendo o usuário, desinstalar mantendo os dados e migrar da instalação pelos scripts). Na tag `vX.Y.Z`, publica o `.msi` e o `SHA256SUMS.txt` no Release. Passo a passo em `CONTRIBUINDO.md`.
  - **Nova configuração na API:** `Dados:Pasta` define banco e chaves numa pasta só e lê o `configuracao.json` dela (certificado). A instalação pelos scripts não muda.
- **Gerenciador do Finanças Local** para Windows (#119, ADR 0028). Instalado pelo `.msi`, roda como o usuário e pede administrador só quando precisa.
  - **Situação do site em palavras:** "no ar" (com a versão do site), "desligado" ou "ligado, mas não responde".
  - **Ações:** "Abrir o site", "Ligar/Desligar" e "Reiniciar".
  - **Versão nova:** procura no GitHub, mostra o que mudou e "Atualizar para x.y.z". Baixa, confere o SHA256 e abre o instalador.
  - **Em "Mais opções":** salvar uma cópia do banco onde você escolher, ver os erros do site, abrir o manual, iniciar com o Windows na área de notificação (com aviso quando o site cai) e procurar versão nova ao abrir.
- **Versão do site num lugar só:** `Directory.Build.props` (`2.1.0`) vale para a API e o gerenciador. Ela aparece em Configurações › Parametrização › Sistema, em `GET /api/auth/estado` (campo `versao`, sem login, só para o gerenciador mostrar junto de "no ar") e no gerenciador.
- **A API cria e atualiza o banco sozinha ao iniciar** (#119, ADR 0003 atualizada).
  - **Os mesmos scripts:** `01-estrutura.sql`, as colunas novas do `2-criar-banco.sh` e, só no banco novo, os dados iniciais, embutidos no build.
  - **Antes de mudar um banco existente:** cópia em `dados/backups/FinancasLocal-antes-atualizacao-*.db`, com as 5 mais recentes guardadas.
  - **Reinício sem mudança de scripts:** não faz nada (`PRAGMA user_version`).
  - **Para que serve:** é a base do instalador `.msi`, que não tem `sqlite3` nem bash. Também deixa o banco pronto no primeiro "F5" do Visual Studio.
- Investimentos: **"Conferir investimentos"** (ADR 0027).
  - **O que faz:** compara a carteira com a posição da corretora e com o extrato, e mostra as correções já marcadas. Nada muda até "Corrigir selecionados", e o aviso tem **Desfazer** por 10 minutos.
  - **FII cadastrado como outro tipo:** reconhecido pelo nome do fundo ou pelo emissor da corretora; passa a ser FII.
  - **Cotas diferentes da corretora:** quando faltam cotas, sugere o débito "Compra de Renda Variável" que nenhuma operação explica. O preço é o débito menos o troco, dividido pelas cotas, comparado com o fechamento do dia. A compra fica ligada ao débito. Sem débito, "Lançar a compra à mão"; com cotas a mais, "Ver as operações".
  - **Troco ou venda contando como rendimento:** o Nubank usa "Valor recebido de Investimentos" para o troco das compras, para o valor das vendas e para os proventos. Troco e venda vão para "Investimentos", com ⓘ "Troco da compra" no glossário.
  - **Rendimento contando como investimento:** os outros créditos voltam para a categoria das regras (Rendimentos).
  - **Aviso no topo de Investimentos** quando a corretora informa outra quantidade de cotas.
  - **Manual:** seção nova em Investimentos.
  - API: `GET /api/carteira/conferencia`, `POST /api/carteira/conferencia/corrigir` e `POST /api/carteira/conferencia/desfazer/{chave}`.
- **Manual do site** (#113, ADR 0026).
  - **Página "Manual"** no menu, com índice de 14 capítulos e busca (sem acento nem maiúscula fazendo diferença, `?busca=` na URL).
  - **Cada seção:** para que serve, passo a passo, "Abrir a tela" (já filtrada quando faz sentido), dicas, erros comuns e termos do glossário.
  - **Endereço próprio por seção:** `/manual/<capitulo>#<secao>`, com navegação entre capítulos.
  - **Botão "Ajuda"** na barra de cima: abre a parte do manual que explica a tela aberta.
  - **Testes que mantêm o manual em dia:** toda tela do menu tem capítulo, e todo link aponta para uma tela que existe.
- Transações: **tipo de cada lançamento** (#111, ADR 0025).
  - **Em cada linha:** etiqueta com ícone e texto: compra no cartão, estorno, pagamento de fatura, Pix enviado ou recebido, transferência, **entre suas contas**, boleto ou débito automático, compra no débito, saque, depósito, tarifa ou juros, rendimento, aplicação ou resgate, salário, empréstimo ou crédito, dinheiro, outro.
  - **De onde vem:** do Open Finance (`operationType`/`paymentMethod` da Pluggy; Pix com o mesmo titular nas duas pontas vira "entre suas contas"); quando o banco não informa, é reconhecido pela descrição.
  - **Fora dos totais:** "fora dos gastos" quando não entra nos totais. **"Somando nos gastos"** ou **"somando nas receitas"** quando um pagamento de fatura ou uma transferência entre contas suas está contando em dobro.
  - **Filtro "Lançamento"** com chip e na URL (`?lancamento=pix`), que vale também no "todas do filtro" do modo edição.
  - **Coluna Conta:** selo do banco e "cartão de crédito" quando é cartão.
  - **Dados guardados:** só o meio e o "mesmo titular" (nunca nomes ou CPF de terceiros), nas colunas novas `Transacao.MeioPagamento` e `Transacao.MesmaTitularidade` (rodar `scripts/2-criar-banco.sh`). As transações antigas ganham o dado do banco quando a sincronização as relê.
- **Contas e cartões** reorganizada (ADR 0024).
  - **Banco de cada conta:** selo com a cor e a marca do banco (Nubank, Itaú, Mercado Pago e mais 25), reconhecido pelo nome da conta e da instituição, ou escolhido em "editar" ("Outro" desliga). O cartão pode ser ligado à conta dele e herda o banco dela, e o cartão desenhado fica com a cor do banco. O selo também aparece em "Faturas do mês" no Painel.
  - **Resumo no topo:** saldo em contas, faturas do mês, dívida total nos cartões e limite disponível.
  - **No cartão:** **fatura do mês** (o que entrou no mês, a mesma conta do Painel, com o vencimento e a situação da fatura) separada da **dívida total** (com parcelas futuras), com ⓘ do glossário; uso do limite (ou "acima do limite em R$ X") e **melhor dia de compra**.
  - **Histórico de faturas** recolhido no próprio cartão, com situação (paga, fechada, em aberto) e "mostrar mais antigas". `/contas?faturas=ID` abre o histórico daquele cartão.
  - Atalho para as transações de cada conta e cartão. Contas do mesmo banco ficam juntas.
  - Colunas novas `Conta.Banco` e `Conta.ContaVinculadaId` (rodar `scripts/2-criar-banco.sh`).
- Transações: filtro **Descrição pouco informativa** (#9), com ⓘ do glossário. Entram as descrições que não dizem o que foi ("PIX ENVIADO", "PAGAMENTO", "TED 123456", "COMPRA CARTAO DEB", só o nome do banco): tirando números soltos, pontuação e palavras genéricas (tipo de operação, bancos, conectivos), sobram menos letras que o mínimo em Parametrização › Transações (padrão 3). Ficam de fora as de categoria escolhida à mão e as marcadas como revisadas. Na URL, `?generica=1`, com chip; cada linha tem "marcar como revisada" (com desfazer) e a barra de seleção marca várias. O Painel mostra um aviso discreto (sem ⚠) com quantas são nos últimos 90 dias, um exemplo e "Revisar". Funciona junto com "Editar categorias" (#87). Coluna nova `Transacao.DescricaoRevisada` (rodar `scripts/2-criar-banco.sh`).
- Transações: **modo edição de categorias** (#87, ADR 0023). "Editar categorias" (fica na URL, `?editar=1`) troca a categoria direto na lista: a troca fica pendente (linha marcada "alterada") até "Salvar alterações (N)", com "Descartar" e Esc para desfazer uma linha; trocar de página ou de filtro com pendências pergunta antes. A barra de seleção marca a troca nas selecionadas, e "Todas as N do filtro" troca de uma vez, inclusive as das outras páginas (com confirmação). Depois de salvar, o aviso tem **Desfazer** (10 min; a API guarda o estado anterior e só aceita a categoria) e, se várias transações iguais foram para a mesma categoria, a tela oferece criar a regra. No celular, cada transação vira um bloco com o seletor em largura total. API: `PATCH /api/transacoes/em-lote` e `POST /api/transacoes/em-lote/desfazer/{chave}`.
- Avisos com ação (ex.: "Desfazer") ficam 10 s e não somem enquanto o mouse ou o foco estão neles.
- **Erros do sistema** em Configurações › Erros (#11, ADR 0022). Avisos e erros da API, da sincronização do Open Finance, das cotações e do câmbio, e falhas da tela no navegador, gravados no banco sem dados sensíveis: query strings, tokens, senhas, CPF e valores em R$ viram `[removido]`. Lista com filtros por situação, período, origem e nível (na URL), "N vezes desde…" para repetições, detalhe com a pilha sob demanda, "Resolvido"/"Reabrir" e "marcar todos do filtro como resolvidos". O Painel avisa quando há erros em aberto na última semana. Os mais antigos que o prazo em Parametrização › Erros do sistema (padrão 90 dias) são apagados sozinhos. Tabela nova `ErroSistema` (rodar `scripts/2-criar-banco.sh`).
- Contas a pagar: **data limite** (#69). Campo "Até (opcional)" nas contas mensais e anuais, ou "quantos pagamentos faltam", que calcula a data. Ao pagar o último, a conta é **encerrada** sozinha: sai de "Próximos vencimentos" no Painel e fica na lista como "encerrada em dd/MM/aaaa". A lista mostra "até dez/2026 · faltam 3" (ou "último pagamento"), e o pagamento do último avisa que a conta será encerrada. Data limite antes do próximo vencimento é recusada pela API e apontada no formulário. Sem data limite, nada muda. Coluna nova `ContaAgendada.DataLimite` (rodar `scripts/2-criar-banco.sh`).
- Metas: ⓘ **Como calculei** no topo do assistente (#90): a conta da recomendação e o aviso de que dinheiro guardado lançado como despesa diminui a sobra prevista.
- Metas: **assistente de quanto guardar** (#90). No topo, a sobra prevista do mês (a mesma da Projeção, com as parcelas já descontadas) menos uma margem de segurança (padrão 10%, em Configurações › Parametrização › Metas) e quanto guardar no total; em cada meta, "Este mês: guarde R$ X", a divisão igual como referência e um **plano mês a mês** (12 meses, aberto sob demanda). As metas têm **prioridade** (↑/↓ no cartão, `PUT /api/metas/prioridades`): metas com prazo recebem primeiro o necessário para cumpri-lo, a sobra vai às sem prazo na ordem de prioridade; quando não dá para todas, vale a prioridade e a meta fora do prazo mostra a data provável e quanto os próximos meses precisam para mantê-lo. Mês que fecha no vermelho recomenda R$ 0 e aponta para a Projeção; com menos de 3 meses de extrato, fica só a divisão igual (`GET /api/metas/plano`). Coluna nova `Meta.Prioridade` (rodar `scripts/2-criar-banco.sh`).
- Metas: acompanhar um **cofrinho** (Reserva do Mercado Pago), não só a conta (#89). Ao escolher uma conta com cofrinhos, aparece "Qual cofrinho (reserva)?" com o nome e o saldo de cada um; o progresso passa a ser o valor inicial + o saldo do cofrinho, sem somar o disponível da conta. Saldo só estimado mostra "confira em Reservas" (`/investimentos?aba=reservas`, que agora abre direto na aba); cofrinho que sumiu do extrato vira aviso com "Editar a meta", sem apagar o vínculo. Coluna nova `Meta.ReservaNome` (rodar `scripts/2-criar-banco.sh`).
- Painel em duas seções (UX-12, #88): **Situação hoje** (posição de hoje, não muda com o mês) e **Mês de …** (o seletor de mês fica nela). "Faturas de cartão" passa a se chamar **Dívida total nos cartões**, com ⓘ do glossário (inclui parcelas futuras; não é a fatura do mês), e o mês ganha o cartão **Faturas do mês**: por cartão, só as transações lançadas no mês escolhido (compras e a parcela do mês; parcelas futuras e pagamento de fatura ficam de fora), quanto disso é parcela e, como referência, a fatura que vence no mês e a situação dela (paga, fechada ou em aberto), com link que abre as faturas do cartão em Contas (`/contas?faturas=ID`).
- Orçamentos: **✨ Sugerir orçamentos** (#39). Para cada categoria, sugere um valor a partir da mediana dos últimos 6 meses (sem emergências, compras parceladas e picos isolados), com o motivo de cada valor. A soma cabe em renda média − parcelas (cartão, financiamentos e empréstimos) − meta de poupança: corta primeiro o estilo de vida, depois o que está sem classificação, e nunca os essenciais sozinho (a tela diz quanto falta). Com os cartões acima do nível de atenção de uso do limite, aperta o estilo de vida pago no cartão. Tudo é revisado numa janela com valores editáveis, total do mês e distribuição da renda (50/30/20) atualizados na hora; categoria que já tem orçamento vem desmarcada. Os percentuais ficam em Configurações › Parametrização › Orçamentos.
- Transações: filtro de **várias categorias** de uma vez, com caixas de marcar (cada categoria inclui as subcategorias, e dá para somar "Sem categoria"). Um chip por categoria em "Filtrando:", cada um com o seu ✕; na URL, `?categoria=12,20,sem` (links antigos com uma categoria continuam valendo). A API aceita `categoriaIds` repetido, mantendo `categoriaId`.
- Empréstimos e financiamentos importados pelo Open Finance (`GET /loans` da Pluggy): saldo devedor, parcelas, taxa, CET e histórico de pagamentos de cada contrato.
- Nova página **Empréstimos e financiamentos**, com financiamentos (imóvel, veículo, bens) e empréstimos sempre separados: totais, contratos e pagamentos por mês.
- Painel: patrimônio líquido passa a descontar o saldo devedor de financiamentos e de empréstimos (cada um em seu cartão), com alerta de parcelas em atraso.
- Câmbio: cotação PTAX oficial do Banco Central (`GET /api/cambio/moedas` e `GET /api/cambio/cotacao?moeda=USD&data=2026-10-15`), guardada no banco para uso offline (`GET /api/cambio/cotacoes`) e atualizada automaticamente uma vez por dia e de novo depois do fechamento do Banco Central (~13h30).
- Investimentos em moeda estrangeira (Open Finance) entram no patrimônio, na reserva e nos totais da carteira convertidos para reais; a tela de investimentos mostra o valor original e o convertido. Moeda sem cotação fica fora dos totais, com aviso.
- Ativos no exterior na carteira (ex.: AAPL, VOO): em **+ Lançar compra/venda**, escolha a moeda; o câmbio vem preenchido com a PTAX do dia e pode ser trocado pelo da corretora. Custo e resultado em reais por operação, valor atual pelo câmbio de hoje e proventos pelo câmbio da data-com.
- Visão anual: o comprometimento da renda soma as parcelas de cartão, financiamentos e empréstimos, mostradas separadamente.
- Configurações em abas (**Open Finance**, **Parametrização**, **Segurança**), com link direto por `?aba=`.
- Nova aba **Parametrização**: sincronização automática, intervalo, dias relidos e histórico da primeira carga; dias de próximos vencimentos e de aviso de consentimento no painel; tentativas de login e tempo de bloqueio. Cada parâmetro mostra a faixa válida e o padrão, vale na hora (sem reiniciar) e é validado pela API (`GET/PUT /api/parametros`). Bloco **Sistema** somente leitura com versão, caminhos, tamanho do banco e totais (`GET /api/parametros/sistema`).
- Tentativas de login e tempo de bloqueio saem do `appsettings.json` para a Parametrização; o valor do `appsettings.json`, se existir, vira o padrão.
- Reservas do Mercado Pago ("cofrinhos"), cujo saldo não vem pelo Open Finance: estimadas pelos lançamentos "Dinheiro reservado/retirado" do extrato, com conferência opcional do saldo real na nova aba **Investimentos › Reservas**. Entram no total investido, no patrimônio e na reserva de emergência. Reservas que precisam ser conferidas (possível saldo anterior à janela importada ou conferência com mais de 30 dias) são destacadas e geram alerta no Painel (#25, ADR 0010).
- Nova aba **Aparência**: paletas prontas (Lavanda, Oceano, Floresta, Grafite, Pôr do sol), cada uma com versão clara e escura, ou uma cor personalizada (tons calculados para manter contraste de 4,5:1), com prévia. Verde, vermelho, laranja de emergência e as cores de financiamento/empréstimo não mudam. Fica salva no navegador.
- Ícones do [Lucide](https://lucide.dev) (licença ISC, empacotados no build) no menu, nos botões de sincronizar, ocultar valores e tema, e nos cartões do painel.
- Novo logo e ícone do site (cofrinho), com versões PNG para atalho no celular e na área de trabalho e `manifest.webmanifest`.
- **Contas a pagar** sugere cadastrar os pagamentos que se repetem todo mês no extrato (boleto, débito automático, Pix), com vencimento estimado pelo dia habitual e valor pela mediana; cada sugestão pode ser cadastrada (formulário preenchido), conferida em Transações ou ignorada (`GET /api/contas-agendadas/sugestoes`, #62).

- Script para parar tudo o que o projeto deixou rodando (serviço, API fora do serviço, `ng serve`) e conferir se o banco ficou livre: `parar-windows.cmd` no Windows, com dois cliques e sem bash, e `scripts/parar.sh` no Linux/macOS. `--desativar` tira o serviço da inicialização automática. Só procura processos com a pasta do projeto na linha de comando (não derruba outro app nem um `dotnet build`) e sai com código 1 se algo ficou rodando. Os testes de instalação no Windows e no Linux param e sobem o sistema de novo.

### Correções (layout)
- Abas no celular (ex.: Configurações): as que não cabem descem de linha, em vez de sair da tela.
- Mensagem de API fora do ar falava em IIS; agora pede para conferir o serviço Financas Local.
- Contas a pagar no celular: a página ficava mais larga que a tela e o formulário abria cortado à direita. O texto do cabeçalho só para leitor de tela escapava da tabela com rolagem; agora fica preso nela (vale para todas as tabelas com rolagem).
- Avisos de lista vazia (ex.: "Nenhum empréstimo ou financiamento encontrado.") ficavam fora do meio do cartão em telas largas: o texto era centralizado dentro de uma caixa de 78 caracteres encostada à esquerda. Agora a caixa também é centralizada, em todas as telas (#46).
- Contraste da cor principal da paleta Lavanda (texto dos botões) passa de 3,5:1 para 4,7:1.
- Arquivos estáticos sem hash no nome (`favicon.svg`, ícones, manifesto) deixam de ficar em cache por um ano: antes, uma troca de ícone não aparecia no navegador.

### Banco
- Novas tabelas `Emprestimo` e `PagamentoEmprestimo` em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh` antes de publicar.
- Nova tabela `CotacaoMoeda` (cache das cotações de câmbio) em `banco/01-estrutura.sql`.
- Novas colunas `Ativo.Moeda` e `OperacaoAtivo.Cambio`: rode `scripts/2-criar-banco.sh` (primeiro uso de `adicionar_coluna_se_ausente`).
- Nova tabela `ReservaConferencia` (saldo conferido das reservas) em `banco/01-estrutura.sql`: rode `scripts/2-criar-banco.sh`.

### Correções
- Relatórios: **transferência para outra pessoa conta como despesa ou receita** (ADR 0031). Antes, todo Pix ou TED na categoria "Transferências" ficava fora dos relatórios, inclusive salário e pagamentos a terceiros.
  - **Continua fora só o que é entre contas suas:** o par aparece na outra conta (mesmo valor, sinal oposto, até 5 dias), o banco informa o mesmo titular, a transferência foi lançada pelo site ou é dinheiro guardado numa reserva.
  - **Quando roda:** na sincronização, ao iniciar o site e depois de mudar categorias.
  - **Outras categorias do tipo transferência** (Investimentos, Pagamento de fatura e as criadas por você) continuam sempre fora.
- Investimentos e Open Finance: **compra lançada à mão não trava mais a importação.** Antes, se você lançasse a compra que faltou no histórico da corretora, aquele ativo parava de receber as próximas compras e vendas do Open Finance. Agora, num ativo que já tem histórico da corretora, o seu lançamento só completa o que faltou. Se a corretora mandar depois a mesma operação (mesmo tipo e quantidade, até 5 dias de diferença), as duas são ligadas, sem duplicar. Ativos só com lançamentos seus continuam fora da importação, como antes. Também deixou de acontecer: uma falha da Pluggy ao listar as movimentações de um ativo com histórico registrava a posição inteira como uma compra a mais.
- Investimentos: **FII do Nubank entrava como ação.** Pelo Open Finance, o Nubank manda FII com o subtipo "STOCK", e o ativo novo virava ação, ficando fora do total e da aba de FIIs. Agora o emissor ("... FDO INV IMOB - FII") ou o nome oficial do fundo, que chega com a primeira cotação, faz o ativo virar FII. Os que já entraram errado são corrigidos por "Conferir investimentos".
- Janelas sobrepostas (ex.: editar uma operação a partir da lista de operações): a aberta por último fica por cima, e o Esc fecha só ela — antes, a edição abria atrás e um Esc fechava todas.
- Investimentos: a tela não carregava (erro 500) quando o Banco Central ou outra fonte demorava mais que o tempo limite; agora segue sem o dado, como já fazia sem internet. Mesmo ajuste nas cotações, no câmbio, na importação pelo extrato e na sincronização do Open Finance.
- Publicação no Linux: `scripts/3-publicar.sh` passa a se chamar `scripts/3-publicar-atualizar.sh` e reinicia o serviço quando ele já está rodando (antes, continuava com a versão antiga em memória).
- Instalação num Windows novo: o `1-preparar-ambiente.sh` deixa de depender de Python (que no Windows costuma ser só o atalho da Microsoft Store) e de `unzip`, baixa o `sqlite3` portátil oficial quando falta, confere as ferramentas do terminal antes de começar e mostra as versões instaladas no final. O `3-publicar-atualizar.sh` também não usa mais Python, funciona com `--pular-build` sem o .NET no PATH e avisa logo no início quando o Git Bash não está como Administrador (ADR 0011).
- Instalação em um passo: `scripts/instalar.sh` roda as três etapas em sequência. No Windows, basta dar dois cliques em `instalar-windows.cmd`, que pede Administrador e roda `scripts/instalar-windows.ps1` — só PowerShell, sem Git, bash nem Python (ADR 0012).
- Publicação no Windows: o serviço é parado antes de copiar os arquivos novos (antes, a cópia falhava com as DLLs em uso).
- Instalador do Windows: o log guarda também as mensagens de erro do npm/dotnet e os eventos do Windows quando a API não sobe; o erro final diz em que etapa parou; API que não responde em 30 s reprova a instalação (antes, terminava em verde); certificado confiável para todas as contas do computador, sem janela de confirmação; clique na janela não congela mais a instalação (QuickEdit desligado); no fim, abre o site e fecha sozinho (só fica aberto se der erro); o log acumula as execuções.
- Atualização mais segura (Windows e bash): cópia do banco em `backups/FinancasLocal-antes-instalacao-*.db` antes de mudar a estrutura (ficam as 5 mais recentes), `sqlite3` espera em vez de falhar com `database is locked` se a API estiver gravando; no Windows, se a versão nova não subir, a anterior volta sozinha.
- Backup e criação do banco funcionam em pastas com apóstrofo no caminho (ex.: `C:\Users\D'Avila`).
- Backup agendado no Windows (`4-backup.sh --agendar`): a tarefa era criada com caminhos do Git Bash (`/usr/bin/bash`, `/c/...`), que o Agendador de Tarefas não entende, e não rodava. Agora usa caminhos do Windows e o `bash.exe` do Git for Windows.
- Validação do código da B3 volta ao formato estrito (ex.: PETR4, MXRF11); ativos de fora da B3, como AOK, entram escolhendo a moeda (ADR 0009).

### Documentação
- README: **instalar pelo `.msi`** (passo a passo, aviso do SmartScreen, conferir o SHA256, o que fica onde, o gerenciador), **atualizar o site** (pelo gerenciador, pelo `.msi` novo, saindo da instalação por scripts para o instalador, ou continuando pelos scripts), **desinstalar** e problemas comuns do instalador. A carteira de renda variável passa a explicar o lançamento manual depois do #117 e o "Conferir investimentos".
- Pasta `docs/` restaurada (fora apagada por engano na migração para SQLite) e atualizada para SQLite/Kestrel e scripts bash.
- Decisões de arquitetura passam a ser registradas como ADRs em `docs/adr/`, com as decisões já tomadas registradas retroativamente.

### Infraestrutura
- Testes automatizados do front-end com **Jest** (`npm test` em `src/frontend`): 84 testes de regressão (filtros de Transações na URL, balão do glossário dentro da tela, utilidades de datas e links) e de usabilidade (nome e estado acessível, teclado e Esc, "Mais análises" que só cria o conteúdo ao abrir, filtros ativos visíveis, status da sincronização anunciado). Rodam na CI (ADR 0013).
- Novo workflow **Teste de instalação no Linux**: roda `scripts/instalar.sh` num runner Ubuntu sem dotnet, node, sqlite3 e Python no PATH, confere o serviço `systemd --user` e a API, testa a atualização e o `4-backup.sh`.
- Novo workflow **Teste de instalação no Windows**: instala do zero num runner Windows (sem as ferramentas do runner no PATH), confere o serviço e a API, e testa a atualização pelo `instalar-windows.cmd`.
- Backend migrado de SQL Server Express/IIS para SQLite/Kestrel — o projeto agora roda em Windows e Linux com os mesmos scripts.
- Nova ferramenta `Financas.MigrarSqlite` para migrar os dados de uma instalação antiga com SQL Server para o SQLite novo.
- Scripts `.sh` (bash) substituindo os `.ps1` de instalação, criação do banco, publicação e backup.

## [1.0.0] — 2026-10-15

Primeira versão versionada no Git.

### Funcionalidades
- Painel com patrimônio líquido (com e sem renda variável), saldos, faturas, receitas × despesas, gastos por categoria, orçamentos, vencimentos e alertas.
- Open Finance via Pluggy/Meu Pluggy: contas, cartões, faturas, transações (`/v2/transactions`) e investimentos, com sincronização agendada.
- Transações com filtros, categorização em lote, regras automáticas, sugestões de categoria pelo histórico e marcação de gastos de emergência.
- Contas e cartões (limite efetivo, faturas), orçamentos com destaque laranja para estouro por emergência, metas, contas a pagar.
- Relatórios: fluxo mensal com emergências, categorias, maiores despesas, projeção mês a mês e visão anual com indicadores de saúde financeira.
- Investimentos: carteira de renda variável (preço médio, proventos, DY, yield on cost), comparação com a Selic, aba de FIIs com gráficos, importação pelo Open Finance e pelo extrato.
- Calculadora flutuante, modo escuro, opção de ocultar valores.

### Infraestrutura
- API .NET 10 no IIS (in-process), Angular 20, SQL Server 2025 Express local.
- Scripts de instalação, criação do banco (sem migrations), publicação e backup.
