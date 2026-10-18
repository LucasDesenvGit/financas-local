import { ChaveGlossario } from '../compartilhado/glossario';

/**
 * Manual do site (#113, ADR 0026). Conteúdo em dados, num lugar só: a página de Manual desenha, a busca procura e
 * os testes conferem que todo link "Abrir a tela" aponta para uma rota que existe e que toda tela do menu tem seção.
 * Mudou o jeito de usar uma tela? Atualize a seção aqui (checklist do PR em CONTRIBUINDO.md).
 *
 * Linguagem: a de quem usa (Nielsen 2), passos curtos e concretos (Nielsen 10). Nomes de botões e campos entre
 * aspas curvas, exatamente como aparecem na tela.
 */
export interface LinkManual {
  rotulo: string;
  /** Rota do site (ex.: '/transacoes'). */
  rota: string;
  params?: Record<string, string>;
}

export interface SecaoManual {
  /** Âncora da seção (/manual/<capitulo>#<id>). */
  id: string;
  titulo: string;
  /** Para que serve, em uma frase. */
  paraQueServe: string;
  passos: string[];
  /** "Abrir a tela": leva direto, já filtrada quando faz sentido (detalhe por navegação). */
  abrir?: LinkManual[];
  dicas?: string[];
  erros?: string[];
  /** Termos do glossário explicados com ⓘ nesta seção. */
  termos?: ChaveGlossario[];
}

export interface CapituloManual {
  /** /manual/<id> */
  id: string;
  titulo: string;
  /** Uma linha para o índice. */
  resumo: string;
  secoes: SecaoManual[];
}

export const MANUAL: CapituloManual[] = [
  {
    id: 'primeiros-passos',
    titulo: 'Primeiros passos',
    resumo: 'O que é o Finanças Local, o primeiro acesso, onde fica cada coisa e por onde começar.',
    secoes: [
      {
        id: 'o-que-e',
        titulo: 'O que é o Finanças Local',
        paraQueServe: 'Um controle financeiro pessoal que roda no seu computador: seus dados ficam com você, não num serviço na internet.',
        passos: [
          'O site abre no seu navegador, mas quem responde é um programa instalado no seu computador (o endereço começa com https://localhost ou https://127.0.0.1).',
          'Os dados ficam num arquivo de banco de dados na pasta “dados” da instalação.',
          'Os extratos dos bancos chegam pelo Open Finance, através do Meu Pluggy, só quando você configura isso.',
          'Para a internet saem só pedidos de cotação e de taxas (códigos de ativos e datas) e a conversa com a Pluggy. Valores e dados pessoais não saem.'
        ],
        abrir: [{ rotulo: 'Abrir o Painel', rota: '/painel' }],
        dicas: ['Veja o capítulo “Privacidade, backup e atualização” para saber exatamente o que sai do computador.']
      },
      {
        id: 'primeiro-acesso',
        titulo: 'Primeiro acesso, entrar e sair',
        paraQueServe: 'Criar o seu usuário e proteger o acesso ao site.',
        passos: [
          'Na primeira vez que o site abre, ele pede para criar um usuário e uma senha (pelo menos 8 caracteres).',
          'Nas próximas vezes, entre com esse usuário e senha.',
          'Para sair, use o menu do usuário (as iniciais, no canto superior direito) e “Sair”.',
          'A sessão termina sozinha depois de um tempo sem uso; é só entrar de novo.'
        ],
        abrir: [{ rotulo: 'Trocar a senha', rota: '/configuracoes', params: { aba: 'seguranca' } }],
        erros: ['Muitas senhas erradas seguidas bloqueiam o login por alguns minutos. Espere e tente de novo; o limite e o tempo ficam em Configurações › Parametrização.']
      },
      {
        id: 'conhecendo-a-tela',
        titulo: 'Conhecendo a tela',
        paraQueServe: 'Saber onde ficam o menu, o status da sincronização e as ferramentas que valem em qualquer tela.',
        passos: [
          'À esquerda fica o menu com todas as telas. No celular, ele abre pelo botão ☰ no topo.',
          'No computador, o mesmo botão ☰ esconde o menu para sobrar mais espaço para tabelas e gráficos; clique de novo para ele voltar. A escolha fica salva neste navegador.',
          'Clicar em “Finanças” (no topo do menu, ou na barra de cima com o menu escondido) leva ao Painel, de qualquer tela.',
          'Na barra de cima: o status da sincronização (“Atualizado há 2 h”, “Sincronizando…” ou “1 conexão com erro”, que leva às Configurações).',
          'O botão do olho esconde todos os valores da tela — útil para mostrar a tela a alguém.',
          'A calculadora abre no botão ao lado ou com Alt+C.',
          'No menu do usuário ficam o tema (claro, escuro ou do sistema), as cores do site e o “Sair”.',
          'O botão “Ajuda” abre este manual na parte que explica a tela em que você está.',
          'Palavras com ⓘ têm explicação: clique (ou use Enter) no ⓘ para ver.'
        ]
      },
      {
        id: 'por-onde-comecar',
        titulo: 'Por onde começar',
        paraQueServe: 'Uma ordem que deixa os relatórios certos o quanto antes.',
        passos: [
          'Conecte seus bancos pelo Open Finance (capítulo “Conectar seus bancos”).',
          'Confira as contas e cartões: o banco de cada um e a conta de cada cartão.',
          'Categorize as transações do mês, começando pelas “Sem categoria”.',
          'Crie regras para o que se repete, para as próximas virem categorizadas.',
          'Monte os orçamentos (dá para pedir uma sugestão).',
          'Crie suas metas e veja quanto guardar por mês.'
        ],
        abrir: [
          { rotulo: 'Conectar os bancos', rota: '/configuracoes' },
          { rotulo: 'Ver as transações sem categoria', rota: '/transacoes', params: { categoria: 'sem' } }
        ]
      }
    ]
  },
  {
    id: 'conectar-bancos',
    titulo: 'Conectar seus bancos (Open Finance)',
    resumo: 'Meu Pluggy, credenciais, conexões, sincronização automática e o que fazer quando dá erro.',
    secoes: [
      {
        id: 'meu-pluggy',
        titulo: 'Autorizar seus bancos no Meu Pluggy',
        paraQueServe: 'O Open Finance precisa do seu consentimento em cada banco; o Meu Pluggy é o caminho para isso.',
        passos: [
          'Crie sua conta em meu.pluggy.ai.',
          'Conecte cada banco pelo fluxo oficial do Open Finance: o consentimento é dado no aplicativo do próprio banco.',
          'O consentimento vale por até 12 meses.'
        ],
        dicas: ['O Meu Pluggy atualiza os dados dos bancos mais ou menos uma vez por dia.']
      },
      {
        id: 'credenciais',
        titulo: 'Client ID e Client Secret',
        paraQueServe: 'As credenciais que deixam o Finanças Local buscar os seus dados na Pluggy.',
        passos: [
          'Em dashboard.pluggy.ai, crie uma conta de desenvolvedor.',
          'Adicione o conector “MeuPluggy” e crie uma aplicação de desenvolvimento.',
          'Copie o Client ID e o Client Secret da aplicação.',
          'No site, abra Configurações › Open Finance, cole os dois e clique em “Salvar”.',
          'Clique em “Testar conexão” para confirmar.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }],
        dicas: ['O Client Secret fica guardado criptografado e nunca é mostrado de novo. Para trocar, cole o novo; para manter, deixe o campo vazio.']
      },
      {
        id: 'conexoes',
        titulo: 'Adicionar as conexões (um Item ID por banco)',
        paraQueServe: 'Cada banco autorizado vira uma conexão, e as contas e cartões dele aparecem sozinhos.',
        passos: [
          'No dashboard da Pluggy, na aplicação de demonstração, conecte o “MeuPluggy” e autorize — uma vez para cada banco.',
          'Cada autorização gera um Item ID (aparece no dashboard).',
          'Em Configurações › Open Finance, cole o Item ID e clique em “Adicionar conexão”.',
          'Clique em “Sincronizar agora”. Na primeira vez são buscados os últimos meses de histórico.',
          'Confira em Contas e cartões se as contas e cartões apareceram.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }, { rotulo: 'Ver as contas', rota: '/contas' }],
        dicas: ['Quanto histórico a primeira carga busca fica em Configurações › Parametrização › Sincronização.']
      },
      {
        id: 'sincronizacao',
        titulo: 'Sincronização automática',
        paraQueServe: 'Manter as transações em dia sem clicar em nada.',
        passos: [
          'A sincronização roda sozinha de tempos em tempos (padrão: a cada 6 horas).',
          'A cada vez, ela relê os últimos dias (padrão: 30) para pegar o que mudou ou entrou atrasado.',
          'O status aparece na barra de cima: “Atualizado há…”, “Sincronizando…” ou “1 conexão com erro”.',
          'Para sincronizar na hora, use “Sincronizar agora” em Configurações › Open Finance.'
        ],
        abrir: [{ rotulo: 'Ajustar a sincronização', rota: '/configuracoes', params: { aba: 'parametrizacao' } }]
      },
      {
        id: 'consentimento-e-erros',
        titulo: 'Consentimento vencendo e erros de conexão',
        paraQueServe: 'Saber o que fazer quando um banco para de atualizar.',
        passos: [
          'O Painel avisa quando o consentimento de um banco vai expirar (padrão: 30 dias antes). Renove no Meu Pluggy.',
          'Se a barra de cima mostrar “1 conexão com erro”, clique nela: ela leva à lista de conexões, com o último erro de cada uma.',
          'Se pedir “ação no Meu Pluggy”, entre no Meu Pluggy e reautorize o banco.',
          'Detalhes técnicos de falhas ficam em Configurações › Erros.'
        ],
        abrir: [{ rotulo: 'Ver as conexões', rota: '/configuracoes' }, { rotulo: 'Ver os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }],
        erros: ['Erro 401 ou 403 ao sincronizar: confira o Client ID e o Client Secret e se o MeuPluggy foi autorizado para aquele banco.']
      }
    ]
  },
  {
    id: 'contas',
    titulo: 'Contas e cartões',
    resumo: 'Banco de cada conta, cartão ligado à conta, fatura do mês × dívida total e histórico de faturas.',
    secoes: [
      {
        id: 'tipos-de-conta',
        titulo: 'Contas do Open Finance e contas manuais',
        paraQueServe: 'Entender de onde vem o saldo de cada conta e quando criar uma conta à mão.',
        passos: [
          'Contas do Open Finance aparecem sozinhas depois da sincronização; saldo, limite e tipo vêm do banco.',
          'Para dinheiro em espécie, banco sem Open Finance ou um controle separado, use “+ Conta manual”.',
          'Na conta manual você informa o tipo, o saldo inicial e, no cartão, limite, dia de fechamento e de vencimento.',
          'Desmarque “Incluir no saldo total” para uma conta não somar no saldo (ex.: conta de outra pessoa).',
          'Conta que não usa mais: marque “Arquivada” em vez de excluir.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }]
      },
      {
        id: 'banco-da-conta',
        titulo: 'O banco de cada conta (o selo)',
        paraQueServe: 'Reconhecer de relance qual conta e qual cartão é de qual banco.',
        passos: [
          'Cada conta mostra o selo do banco, reconhecido pelo nome da conta e da instituição.',
          'Se o banco estiver errado (ou não aparecer), clique em “editar” na conta.',
          'No campo “Banco”, escolha o banco certo. “Detectar pelo nome” volta para o reconhecimento automático.',
          'Para não mostrar banco nenhum, escolha “Outro (sem banco)”.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        dicas: ['O cartão desenhado fica com a cor do banco; a cor escolhida na conta só vale quando o banco não é identificado.']
      },
      {
        id: 'cartao-e-conta',
        titulo: 'Ligar o cartão à conta dele',
        paraQueServe: 'Mostrar de qual conta o cartão é (e de onde a fatura é paga), e o cartão usar o banco da conta.',
        passos: [
          'Clique em “editar” no cartão.',
          'Em “Conta do cartão”, escolha a conta do mesmo banco.',
          'Salve. O cartão passa a mostrar “da conta …”, e a conta mostra “Cartões: …”.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }]
      },
      {
        id: 'fatura-e-divida',
        titulo: 'Fatura do mês × dívida total',
        paraQueServe: 'Não confundir o que entrou no cartão neste mês com tudo o que você deve nele.',
        passos: [
          '“Fatura de <mês>” é o que entrou no cartão no mês: compras e a parcela do mês das compras parceladas.',
          'Abaixo dela aparece quando vence a fatura do mês e a situação (paga, fechada ou em aberto).',
          '“Dívida total” é tudo o que você deve no cartão hoje, inclusive as parcelas que ainda vão vencer.',
          'A barra mostra quanto do limite está em uso e quanto ainda está disponível (ou quanto passou do limite).',
          '“Melhor dia de compra” é o dia seguinte ao fechamento: o que você compra a partir dele só entra na fatura seguinte.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['faturaDoMes', 'dividaCartao', 'melhorDiaCompra']
      },
      {
        id: 'historico-faturas',
        titulo: 'Histórico de faturas',
        paraQueServe: 'Ver as faturas anteriores de um cartão, sem sair da tela.',
        passos: [
          'No cartão, clique em “Histórico de faturas”.',
          'Aparecem as faturas mais recentes: vencimento, quando fechou, total e situação.',
          'Use “Mostrar mais antigas” para ver o resto.',
          'No Painel, clicar no nome de um cartão em “Faturas do mês” abre direto este histórico.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        erros: ['Cartão manual não tem histórico: as faturas vêm do banco pelo Open Finance.']
      }
    ]
  },
  {
    id: 'transacoes',
    titulo: 'Transações: encontrar e entender',
    resumo: 'Filtros, o que cada etiqueta quer dizer, lançamentos manuais, transferências e edição de uma transação.',
    secoes: [
      {
        id: 'filtros',
        titulo: 'Filtrar e buscar',
        paraQueServe: 'Encontrar rápido as transações que importam.',
        passos: [
          'Escolha o período em “De” e “Até” (o padrão é o mês atual).',
          'Filtre por conta, por uma ou várias categorias (inclusive “Sem categoria”), por tipo (despesas, receitas, transferências) e por “Lançamento” (Pix, compras no cartão, pagamento de fatura…).',
          'Marque “Só emergências” ou “Descrição pouco informativa” quando precisar.',
          'Digite em “Buscar” e aperte Enter para procurar na descrição, no estabelecimento e na observação.',
          'Os filtros ativos aparecem em “Filtrando:”; o ✕ de cada um tira só aquele, e “Limpar filtros” tira todos.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        dicas: ['O endereço da página guarda os filtros: dá para salvar nos favoritos ou mandar o link de uma lista filtrada.'],
        termos: ['tipoLancamento']
      },
      {
        id: 'etiquetas',
        titulo: 'O que cada etiqueta quer dizer',
        paraQueServe: 'Entender cada transação sem precisar abri-la.',
        passos: [
          'A primeira etiqueta diz como o dinheiro entrou ou saiu: “Compra no cartão”, “Pix enviado”, “Entre suas contas”, “Pagamento de fatura”, “Boleto ou débito automático”…',
          '“fora dos gastos”: a transação não entra nas despesas nem nas receitas (transferência entre contas suas, pagamento de fatura, investimento ou “ignorar nos relatórios”).',
          '“somando nos gastos” (ou “nas receitas”): um pagamento de fatura ou uma transferência entre contas suas está contando em dobro. Veja como corrigir no capítulo de categorização.',
          '“2/10”: parcela 2 de 10. “pendente”: ainda não efetivada. “emergência”: gasto marcado como emergência. “open finance”: veio do banco.',
          'Na coluna Conta, o selo do banco e “cartão de crédito” quando é cartão.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        termos: ['tipoLancamento']
      },
      {
        id: 'editar-transacao',
        titulo: 'Abrir e ajustar uma transação',
        paraQueServe: 'Corrigir a categoria, dar um nome melhor ou marcar algo especial numa transação.',
        passos: [
          'Clique na linha da transação.',
          'Mude a “Descrição” para um nome que você reconheça (a original do banco continua guardada e é ela que as regras usam).',
          'Escolha a “Categoria” — ou um dos atalhos em “Sugestões”. “Automática (regras)” volta para a categorização automática.',
          'Use “Observação” para lembrar de algo.',
          'Marque “Ignorar nos relatórios e totais” ou “Gasto de emergência” quando fizer sentido, e salve.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }],
        erros: ['Transações do Open Finance não podem ser excluídas (o banco mandaria de novo). Para tirar dos totais, marque “Ignorar nos relatórios”.']
      },
      {
        id: 'renomear',
        titulo: 'Renomear: um nome claro, também para as próximas',
        paraQueServe: 'Trocar descrições como “PIX ENVIADO 12345” por um nome que você entenda daqui a meses, e fazer as próximas iguais já chegarem com ele.',
        passos: [
          'Em Transações, clique no ✎ ao lado da descrição (ou use Tab até ele e Enter).',
          'Escreva o novo nome em “Nova descrição”. A descrição do banco aparece em cima, para comparar.',
          'Em “Onde usar este nome”, escolha “Só nesta transação” ou “Também nas outras com esta descrição do banco e nas próximas que chegarem”.',
          'Com “também nas outras”: escolha se a descrição do banco deve “for igual a”, “contiver” ou “começar com” o texto. O site mostra quantas transações vão mudar antes de salvar.',
          'Salve. O aviso tem “Desfazer”. Na lista, o nome novo fica em cima e a descrição do banco embaixo, menor (“do banco: …”).',
          'Para voltar, abra o ✎ de novo e use “Voltar à descrição do banco”.',
          'Várias de uma vez: marque as transações e use “✎ Renomear” na barra de seleção. Com “Nas selecionadas e nas próximas”, o site cria uma regra por descrição do banco.',
          'As regras ficam em Categorias › Regras automáticas › “Regras de descrição”. “+ Regra de descrição” cria uma direto (com prévia de quantas transações renomeia), e dá para editar, desligar ou excluir.'
        ],
        abrir: [
          { rotulo: 'Ver as descrições pouco informativas', rota: '/transacoes', params: { generica: '1' } },
          { rotulo: 'Ver as regras de descrição', rota: '/categorias' }
        ],
        dicas: [
          'Para PIX e boletos com número que muda todo mês, use “contiver” e deixe só a parte fixa (o site já sugere o texto sem os números).',
          'A busca acha a transação pelo nome novo e pela descrição do banco. A categoria continua vindo da descrição do banco (as regras de categoria não mudam).',
          'Uma transação renomeada sai do filtro “Descrição pouco informativa”.'
        ],
        erros: [
          'Nome trocado à mão (pelo ✎ “Só nesta” ou pelo formulário da transação) não é mudado por nenhuma regra. Para a regra valer nela, use “Também nas outras” de novo a partir dela.'
        ]
      },
      {
        id: 'lancamento-manual',
        titulo: 'Lançamento manual e transferência',
        paraQueServe: 'Registrar o que o banco não informa, como gastos em dinheiro.',
        passos: [
          'Crie antes uma conta manual (ex.: “Carteira”) em Contas e cartões.',
          'Em Transações, clique em “+ Lançamento”: tipo, conta, data, valor, descrição e categoria.',
          'Marque “Pendente” para algo que ainda vai acontecer.',
          'Para dinheiro que passou de uma conta para outra, use “⇄ Transferência”: ela não conta como gasto nem como receita.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }, { rotulo: 'Criar uma conta manual', rota: '/contas' }],
        dicas: ['A transferência é lançada só nas contas manuais; o lado do Open Finance chega pela sincronização.']
      },
      {
        id: 'ignorar-e-emergencia',
        titulo: 'Quando usar “ignorar nos relatórios” e “emergência”',
        paraQueServe: 'Deixar os totais e orçamentos refletindo a sua rotina.',
        passos: [
          '“Ignorar nos relatórios”: para o que não é gasto nem receita de verdade (ex.: um reembolso que entrou e saiu, um valor duplicado).',
          '“Gasto de emergência”: para despesas fora da rotina (conserto, remédio, viagem urgente). Elas continuam nos totais, mas aparecem em laranja nos orçamentos e relatórios.',
          'Em lote: selecione várias linhas e use “Marcar emergência” na barra que aparece.'
        ],
        abrir: [{ rotulo: 'Ver só as emergências', rota: '/transacoes', params: { emergencias: '1' } }]
      }
    ]
  },
  {
    id: 'categorizar',
    titulo: 'Categorizar suas transações',
    resumo: 'O que deixa os relatórios certos: corrigir uma, várias de uma vez, sugestões e zerar a fila.',
    secoes: [
      {
        id: 'por-que',
        titulo: 'Por que categorizar faz diferença',
        paraQueServe: 'Quase tudo no site depende da categoria de cada transação.',
        passos: [
          'Gastos por categoria no Painel e nos Relatórios.',
          'Orçamentos e a sugestão de orçamentos.',
          'O assistente de metas (a sobra prevista) e a distribuição da renda (50/30/20).',
          'Categoria errada distorce tudo isso — e “Sem categoria” some das análises por categoria.'
        ],
        dicas: ['Uma rotina de 10 minutos por semana revisando “Sem categoria” e “Descrição pouco informativa” mantém tudo em dia.']
      },
      {
        id: 'automatica-x-manual',
        titulo: 'Categoria automática × escolhida à mão',
        paraQueServe: 'Saber o que muda sozinho e o que fica como você deixou.',
        passos: [
          'Ao chegar, cada transação recebe uma categoria automática: primeiro pelas suas regras, depois pela categoria que o banco informa.',
          'Quando você escolhe a categoria, ela passa a ser “escolhida à mão” e não muda mais sozinha.',
          'Para devolver à categorização automática, escolha “Automática (regras)”.'
        ]
      },
      {
        id: 'modo-edicao',
        titulo: 'Várias de uma vez: “Editar categorias”',
        paraQueServe: 'Arrumar dezenas de transações rápido, sem abrir uma por uma.',
        passos: [
          'Em Transações, filtre o que quer arrumar (ex.: o mês, “Sem categoria”).',
          'Clique em “✎ Editar categorias”. Cada linha ganha um seletor de categoria.',
          'Troque a categoria nas linhas. A linha fica marcada “alterada” — nada é gravado ainda. Esc numa linha desfaz a troca dela.',
          'Para várias linhas iguais: marque-as e use “Trocar nas selecionadas” na barra de seleção.',
          'Para tudo o que está no filtro (inclusive outras páginas): use “Todas as N transações do filtro” e confirme.',
          'Clique em “Salvar alterações (N)”. O aviso que aparece tem “Desfazer”.',
          'Se várias transações iguais foram para a mesma categoria, a tela oferece “Criar regra” para as próximas.'
        ],
        abrir: [{ rotulo: 'Editar as sem categoria', rota: '/transacoes', params: { categoria: 'sem', editar: '1' } }],
        erros: ['Mudar de página ou de filtro com trocas não salvas pede confirmação — salve antes para não perder.']
      },
      {
        id: 'sugestoes',
        titulo: 'Sugestões de categoria',
        paraQueServe: 'Deixar o site propor a categoria pelo seu histórico, em grupos de transações parecidas.',
        passos: [
          'Em Transações, clique em “✦ Sugestões de categoria”.',
          'Escolha “Sem categoria” ou “Revisar automáticas”.',
          'Cada grupo mostra quantas transações tem, a sugestão e a confiança (alta, média, palavra-chave).',
          'Ajuste a categoria se precisar, marque a caixa da coluna “Regra” se quiser que as próximas venham sozinhas, e clique em “Aplicar”.',
          '“Aplicar as N de confiança alta” resolve de uma vez os grupos mais certos.'
        ],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }]
      },
      {
        id: 'zerar-a-fila',
        titulo: 'Zerar a fila: sem categoria e descrição pouco informativa',
        paraQueServe: 'Encontrar o que ainda não tem categoria certa.',
        passos: [
          'Abra Transações com o filtro “Sem categoria” e categorize (de preferência no “Editar categorias”).',
          'Marque o filtro “Descrição pouco informativa”: aparecem as transações cuja descrição não diz o que foi (“PIX ENVIADO”, “PAGAMENTO”).',
          'Para cada uma: escolha a categoria, dê um nome melhor na descrição ou clique em “✓ marcar como revisada”.',
          'O Painel avisa, discretamente, quantas descrições pouco informativas há nos últimos 90 dias.'
        ],
        abrir: [
          { rotulo: 'Ver as sem categoria', rota: '/transacoes', params: { categoria: 'sem' } },
          { rotulo: 'Ver as de descrição pouco informativa', rota: '/transacoes', params: { generica: '1' } }
        ],
        termos: ['descricaoPoucoInformativa']
      },
      {
        id: 'gasto-em-dobro',
        titulo: 'Pagamento de fatura e transferências contando em dobro',
        paraQueServe: 'Evitar que o pagamento da fatura (ou dinheiro passando entre suas contas) apareça como gasto novo.',
        passos: [
          'As compras do cartão já entram como gasto. Pagar a fatura não é um gasto novo — só tira dinheiro da conta para o cartão.',
          'Filtre “Lançamento” por “Pagamento de fatura” (ou “Entre suas contas”) e procure a etiqueta “somando nos gastos”.',
          'Entre no “Editar categorias”, use “Todas as N transações do filtro” e escolha a categoria “Pagamento de fatura” (ou “Transferências”).',
          'Essas categorias são do tipo transferência: as transações passam a ficar “fora dos gastos”.',
          'Na categoria “Transferências”, só fica fora o que é entre contas suas: o par aparece na outra conta (mesmo valor, até 5 dias), o banco informa o mesmo titular, a transferência foi lançada pelo site ou é dinheiro guardado numa reserva. Pix e TED para outra pessoa (ou recebidos de outra pessoa, como salário) contam como despesa ou receita.',
          'Mandou dinheiro para uma conta sua que não está conectada? O site não vê o par: crie uma categoria do tipo transferência (ex.: “Entre minhas contas”) e use nela. Outras categorias do tipo transferência ficam sempre fora.'
        ],
        abrir: [{ rotulo: 'Ver os pagamentos de fatura', rota: '/transacoes', params: { lancamento: 'fatura' } }],
        termos: ['tipoLancamento']
      }
    ]
  },
  {
    id: 'regras',
    titulo: 'Regras automáticas',
    resumo: 'Criar regras, prioridade, reaplicar, boas práticas e as categorias que vêm do banco.',
    secoes: [
      {
        id: 'o-que-e-regra',
        titulo: 'O que é uma regra',
        paraQueServe: 'Dizer uma vez “descrição contém UBER → Transporte” e nunca mais categorizar isso à mão.',
        passos: [
          'Uma regra olha a descrição original que veio do banco.',
          'Se a descrição bate com o texto da regra, a transação recebe a categoria da regra.',
          'As regras valem para as transações que chegam e para as que voltam para “Automática”.'
        ]
      },
      {
        id: 'criar-regra',
        titulo: 'Criar uma regra',
        paraQueServe: 'Categorizar sozinho o que se repete.',
        passos: [
          'Abra Categorias e regras › “Regras automáticas” e clique em “+ Regra”.',
          'Em “Quando a descrição”, escolha “contém”, “começa com” ou “é igual a”.',
          'Em “Texto”, escreva o trecho da descrição (sem acento ou maiúscula fazendo diferença).',
          'Escolha a “Categoria”.',
          'Em “Prioridade”, um número menor vence quando duas regras batem com a mesma transação.',
          'Salve. Para atualizar as transações que já existem, use “Reaplicar regras em todas as transações”.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }],
        dicas: ['Também dá para criar regra direto do “Editar categorias” e das “Sugestões de categoria”.']
      },
      {
        id: 'reaplicar',
        titulo: 'Reaplicar as regras',
        paraQueServe: 'Atualizar as transações antigas depois de criar ou mudar regras.',
        passos: [
          'Em Categorias e regras › “Regras automáticas”, clique em “Reaplicar regras em todas as transações”.',
          'Só mudam as transações com categoria automática; as escolhidas à mão ficam como estão.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }]
      },
      {
        id: 'boas-praticas',
        titulo: 'Boas práticas',
        paraQueServe: 'Regras que acertam e não atrapalham.',
        passos: [
          'Use um texto curto e específico: o nome da loja ou do serviço (“IFOOD”, “NETFLIX”).',
          'Evite palavras genéricas como “PIX”, “PAGAMENTO” ou “COMPRA”: elas pegam transações demais.',
          'Antes de criar, procure o texto na busca de Transações para ver o que ele pegaria.',
          'Se duas regras brigam, ajuste a prioridade ou deixe o texto mais específico.'
        ],
        abrir: [{ rotulo: 'Buscar em Transações', rota: '/transacoes' }]
      },
      {
        id: 'categorias-do-banco',
        titulo: 'Categorias que vêm do banco',
        paraQueServe: 'Aproveitar a categoria que o banco já informa, ligada às suas categorias.',
        passos: [
          'Abra Categorias e regras › “Categorias do banco”.',
          'Para cada categoria do Open Finance, escolha a sua categoria correspondente.',
          'Quando nenhuma regra sua bate, vale esse mapeamento.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }]
      }
    ]
  },
  {
    id: 'categorias',
    titulo: 'Categorias',
    resumo: 'Criar categorias e subcategorias, tipo, natureza do gasto e por que transferência não é gasto.',
    secoes: [
      {
        id: 'criar-categoria',
        titulo: 'Criar e organizar categorias',
        paraQueServe: 'Ter as categorias do seu jeito.',
        passos: [
          'Abra Categorias e regras › “Categorias” e clique em “+ Categoria”.',
          'Dê um “Nome”, escolha o “Tipo” (despesa, receita ou transferência) e uma cor.',
          'Para uma subcategoria (ex.: Lazer › Cinema), escolha a categoria em “Subcategoria de”.',
          'No filtro de Transações, escolher uma categoria inclui as subcategorias dela.'
        ],
        abrir: [{ rotulo: 'Abrir Categorias e regras', rota: '/categorias' }]
      },
      {
        id: 'tipo-transferencia',
        titulo: 'Por que transferência não conta como gasto',
        paraQueServe: 'Evitar contar o mesmo dinheiro duas vezes.',
        passos: [
          'Categorias do tipo transferência (como “Transferências” e “Pagamento de fatura”) ficam fora das despesas e receitas.',
          'Dinheiro que só muda de conta não é gasto; o pagamento da fatura não é gasto novo, porque as compras já entraram no cartão.',
          'Na lista de Transações, elas aparecem com a etiqueta “fora dos gastos”.'
        ]
      },
      {
        id: 'natureza',
        titulo: 'Essencial × estilo de vida (50/30/20)',
        paraQueServe: 'Ver quanto da renda vai para o necessário e quanto para o resto.',
        passos: [
          'Em cada categoria de despesa, escolha a “Natureza do gasto”: essencial (moradia, mercado, saúde) ou estilo de vida (lazer, restaurantes).',
          'A Visão anual e a sugestão de orçamentos usam isso na divisão 50/30/20: até 50% para essenciais, 30% para estilo de vida e pelo menos 20% para guardar.'
        ],
        abrir: [{ rotulo: 'Abrir a Visão anual', rota: '/anual' }],
        termos: ['comprometimento']
      }
    ]
  },
  {
    id: 'orcamentos',
    titulo: 'Orçamentos',
    resumo: 'Limite por categoria, copiar do mês anterior, sugestão automática e como acompanhar.',
    secoes: [
      {
        id: 'definir-limites',
        titulo: 'Definir os limites do mês',
        paraQueServe: 'Combinar consigo mesmo quanto gastar em cada categoria.',
        passos: [
          'Abra Orçamentos e escolha o mês no seletor.',
          'Na coluna “Limite (R$)”, digite o limite de cada categoria. Em branco = sem orçamento.',
          'Subcategorias somam no orçamento da categoria pai.',
          'Para repetir os limites, use “Copiar do mês anterior”.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }]
      },
      {
        id: 'sugerir',
        titulo: '✨ Sugerir orçamentos',
        paraQueServe: 'Partir de valores que cabem na sua renda, calculados pelos seus gastos.',
        passos: [
          'Clique em “✨ Sugerir orçamentos”.',
          'Cada categoria recebe um valor pela mediana dos últimos 6 meses (sem emergências, compras parceladas e picos isolados), com o motivo.',
          'A soma cabe na renda média menos as parcelas e a meta de poupança; quando não cabe, a tela diz quanto falta.',
          'Revise e edite os valores; categoria que já tem orçamento vem desmarcada.',
          'Clique em aplicar para gravar os marcados.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }],
        dicas: ['A meta de poupança e os níveis de uso do cartão ficam em Configurações › Parametrização › Orçamentos.']
      },
      {
        id: 'acompanhar',
        titulo: 'Acompanhar durante o mês',
        paraQueServe: 'Perceber cedo quando uma categoria está passando do limite.',
        passos: [
          'A barra de cada categoria mostra quanto do limite já foi.',
          'Laranja quer dizer que passou do limite só por causa de gastos de emergência.',
          'O Painel avisa os orçamentos estourados.',
          'Clique no gasto de uma categoria para ver as transações por trás dele.'
        ],
        abrir: [{ rotulo: 'Abrir Orçamentos', rota: '/orcamentos' }, { rotulo: 'Abrir o Painel', rota: '/painel' }]
      }
    ]
  },
  {
    id: 'metas',
    titulo: 'Metas',
    resumo: 'Criar metas, acompanhar o progresso, prioridade, quanto guardar por mês e ideias de metas.',
    secoes: [
      {
        id: 'criar-meta',
        titulo: 'Criar uma meta',
        paraQueServe: 'Dar nome, valor e prazo a um objetivo.',
        passos: [
          'Abra Metas e clique em “+ Nova meta”.',
          'Informe o nome, o “Valor alvo” e, se tiver, a “Data alvo”.',
          'Em “Valor inicial já guardado”, coloque o que você já tem para essa meta.',
          'Escolha uma cor e salve.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }]
      },
      {
        id: 'acompanhar-meta',
        titulo: 'Acompanhar o progresso',
        paraQueServe: 'Fazer a meta andar sozinha ou registrar cada aporte.',
        passos: [
          'Sem conta ligada: registre cada valor guardado em “+ aporte”.',
          'Com uma conta ligada (“Acompanhar saldo de uma conta”): o progresso é o valor inicial mais o saldo da conta.',
          'Na conta do Mercado Pago, dá para escolher um cofrinho (reserva): o progresso usa o saldo dele.',
          '“confira em Reservas” aparece quando o saldo do cofrinho é estimado: confira o saldo real em Investimentos › Reservas.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }, { rotulo: 'Conferir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }]
      },
      {
        id: 'quanto-guardar',
        titulo: 'Prioridade e quanto guardar por mês',
        paraQueServe: 'Saber quanto guardar este mês em cada meta, pela sobra prevista de verdade.',
        passos: [
          'Com pelo menos 3 meses de extrato, o topo de Metas mostra a sobra prevista do mês e quanto guardar no total.',
          'Use ↑ e ↓ em cada meta para definir a prioridade.',
          'Cada meta mostra “Este mês: guarde R$ X”; as com prazo recebem primeiro o necessário para cumpri-lo, e o resto vai pela prioridade.',
          'Quando o prazo não cabe, a meta mostra a data provável e quanto os próximos meses precisariam.',
          '“Plano mês a mês” mostra os próximos 12 meses; “Como calculei” explica a conta.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }, { rotulo: 'Ver a Projeção', rota: '/projecao' }],
        termos: ['sobraPrevista', 'margemSeguranca', 'recomendacaoMetas'],
        dicas: ['Dinheiro que vai para cofrinho ou investimento precisa estar como transferência; lançado como despesa, ele diminui a sobra prevista.']
      },
      {
        id: 'ideias-de-metas',
        titulo: 'Ideias de metas (e como definir valor e prazo)',
        paraQueServe: 'Começar por objetivos que costumam fazer diferença.',
        passos: [
          'Reserva de emergência: de 3 a 6 meses dos seus gastos essenciais (veja a média na Visão anual). Sem data, prioridade 1.',
          'Quitar o cartão ou um empréstimo: o valor da dívida total; prazo curto, porque juros de cartão crescem rápido.',
          'Viagem: o custo estimado (passagem, hospedagem, dia a dia) e a data da viagem.',
          'Entrada de imóvel: em geral 20% do valor do imóvel; prazo de alguns anos.',
          'Troca de carro ou estudos: o valor e a data em que vai precisar.'
        ],
        abrir: [{ rotulo: 'Criar uma meta', rota: '/metas' }],
        termos: ['reservaEmergencia']
      }
    ]
  },
  {
    id: 'contas-a-pagar',
    titulo: 'Contas a pagar',
    resumo: 'Cadastrar vencimentos, pagar, data limite e as sugestões que vêm do extrato.',
    secoes: [
      {
        id: 'cadastrar-conta',
        titulo: 'Cadastrar uma conta a pagar',
        paraQueServe: 'Ser lembrado dos vencimentos no Painel.',
        passos: [
          'Abra Contas a pagar e clique em “+ Nova conta”.',
          'Informe a descrição, o valor, o “Próximo vencimento” e a recorrência (mensal, anual ou única).',
          'Para conta que acaba (escola até dezembro, parcelas de um curso), preencha “Até (opcional)” ou “quantos pagamentos faltam”.',
          'Escolha a categoria e, se quiser, a conta de onde sai o pagamento.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }]
      },
      {
        id: 'pagar',
        titulo: 'Registrar o pagamento',
        paraQueServe: 'Marcar como paga e já ir para o próximo vencimento.',
        passos: [
          'Clique em “Pagar” na conta.',
          'Confira a data. Se a conta de pagamento for manual, dá para lançar a despesa junto.',
          'Confirme: o vencimento avança um mês (ou um ano). No último pagamento antes da data limite, a conta é encerrada sozinha.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }]
      },
      {
        id: 'sugestoes-extrato',
        titulo: 'Sugestões do extrato',
        paraQueServe: 'Cadastrar em um clique os pagamentos que se repetem todo mês.',
        passos: [
          'Em “Sugestões do extrato”, aparecem boletos, débitos automáticos e Pix que se repetem e ainda não estão na lista.',
          'Use “ver pagamentos” para conferir no extrato.',
          '“Cadastrar” abre o formulário já preenchido; “ignorar” tira a sugestão.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }]
      }
    ]
  },
  {
    id: 'relatorios',
    titulo: 'Relatórios e análises: como ler',
    resumo: 'Painel, Visão anual, Relatórios e Projeção — e as perguntas que cada um responde.',
    secoes: [
      {
        id: 'painel',
        titulo: 'Painel',
        paraQueServe: 'Responder “como estou hoje?” e “como está este mês?”.',
        passos: [
          '“Situação hoje”: patrimônio, saldo em contas, dívida total nos cartões, investimentos e próximos vencimentos — não muda com o mês escolhido.',
          '“Mês de …”: receitas, despesas, resultado e faturas do mês; troque o mês no seletor.',
          'Os avisos no topo mostram o que precisa de atenção (orçamento estourado, conta vencida, consentimento expirando, erros).',
          '“Mais análises” abre a evolução de 12 meses e as últimas transações.',
          'Clique num total por categoria para ver as transações por trás dele.'
        ],
        abrir: [{ rotulo: 'Abrir o Painel', rota: '/painel' }],
        termos: ['dividaCartoes', 'patrimonioSemRendaVariavel']
      },
      {
        id: 'visao-anual',
        titulo: 'Visão anual',
        paraQueServe: 'Responder “como foi o meu ano?”.',
        passos: [
          'Poupança acumulada, patrimônio estimado e para onde vai a sua renda (50/30/20).',
          'Despesas e receitas mês a mês, comparação de categorias com outro ano e o mapa de gastos.',
          'Clique nos valores para ver as transações.'
        ],
        abrir: [{ rotulo: 'Abrir a Visão anual', rota: '/anual' }],
        termos: ['comprometimento', 'taxaPoupanca']
      },
      {
        id: 'relatorios-tela',
        titulo: 'Relatórios',
        paraQueServe: 'Responder “para onde foi o meu dinheiro?”.',
        passos: [
          'Fluxo de caixa mensal: receitas, despesas e o saldo acumulado.',
          'Despesas (ou receitas) por categoria no período.',
          'Maiores despesas do período.'
        ],
        abrir: [{ rotulo: 'Abrir Relatórios', rota: '/relatorios' }]
      },
      {
        id: 'projecao',
        titulo: 'Projeção',
        paraQueServe: 'Responder “quanto vou ter no fim do mês (e nos próximos)?”.',
        passos: [
          'Os meses seguintes usam a média das receitas, a média das despesas à vista e as parcelas já contratadas.',
          'O mês atual junta o que já aconteceu com a média do que falta.',
          'Use o gráfico para ver a tendência e a tabela mês a mês para os números.'
        ],
        abrir: [{ rotulo: 'Abrir a Projeção', rota: '/projecao' }],
        termos: ['sobraPrevista']
      },
      {
        id: 'como-ler-graficos',
        titulo: 'Como ler os gráficos',
        paraQueServe: 'Tirar a conclusão certa de cada gráfico.',
        passos: [
          'Todo gráfico tem um título, uma frase com a conclusão e uma linha “como ler”.',
          'Com mais de uma série, a legenda diz o que é cada cor.',
          'Com “ocultar valores” ligado, a frase-resumo também esconde os valores.'
        ]
      }
    ]
  },
  {
    id: 'investimentos',
    titulo: 'Investimentos, reservas e crédito',
    resumo: 'Carteira de renda variável, importação, Selic, FIIs, reservas (cofrinhos), exterior e empréstimos.',
    secoes: [
      {
        id: 'carteira',
        titulo: 'Carteira de renda variável',
        paraQueServe: 'Acompanhar ações, FIIs, ETFs e BDRs com preço médio e proventos.',
        passos: [
          'Abra Investimentos e clique em “+ Lançar compra/venda”.',
          'Informe o código da B3 (ex.: MXRF11), a quantidade, o preço pago e a data.',
          'A carteira mostra preço médio, cotação atual, valorização e proventos.',
          'As abas separam o Resumo, os FIIs, a Renda variável e o que veio do Open Finance.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        termos: ['precoMedio', 'dy', 'dyNaCompra', 'yieldOnCost']
      },
      {
        id: 'importar',
        titulo: 'Importar compras e vendas',
        paraQueServe: 'Não digitar o que já está no extrato ou no Open Finance.',
        passos: [
          '“⇣ Importar do extrato” procura compras e vendas nas descrições das transações e deixa você escolher o que importar.',
          'Na aba Open Finance, “Registrar compras na carteira” transforma posições do banco em operações da carteira.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }]
      },
      {
        id: 'conferir',
        titulo: 'Conferir investimentos',
        paraQueServe: 'Achar e corrigir de uma vez o que costuma sair errado com ações e FIIs vindos do Open Finance.',
        passos: [
          'Em Investimentos, clique em “Conferir investimentos”. O site compara a carteira com a posição da corretora e com o extrato.',
          '“FII cadastrado como outro tipo”: o nome do fundo diz que é FII, mas ele está como ação e fica fora do total de FIIs.',
          '“Cotas que faltam”: a corretora informa mais cotas do que a carteira tem. Quando um débito de compra do extrato explica a diferença, ele já vem escolhido, com o preço calculado. Confira as cotas e o preço.',
          '“Troco ou venda contando como rendimento”: créditos da corretora que são troco de uma compra ou o valor de uma venda vão para “Investimentos”.',
          '“Rendimento contando como investimento”: os outros créditos da corretora são proventos e voltam a contar como receita.',
          'Desmarque o que não quiser mudar e clique em “Corrigir selecionados”. O aviso tem “Desfazer” por 10 minutos.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        dicas: [
          'Quando a carteira tem cotas diferentes da corretora, a própria tela de Investimentos avisa no topo.',
          'A compra lançada por aqui fica ligada ao débito do extrato e não trava as próximas compras que chegam pelo Open Finance.'
        ],
        erros: [
          'Mudar todos os “Valor recebido de Investimentos” para Investimentos de uma vez: os proventos deixam de contar como receita. A conferência separa o troco do rendimento.'
        ],
        termos: ['trocoDeCompra']
      },
      {
        id: 'selic-fiis',
        titulo: 'Comparação com a Selic e FIIs',
        paraQueServe: 'Saber se a carteira está rendendo mais que a renda fixa.',
        passos: [
          'O Resumo compara o seu dinheiro na renda variável com o mesmo valor aplicado na Selic no mesmo período.',
          'A aba FIIs mostra renda, compras × hoje, proventos por mês e rentabilidade × Selic e IFIX.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        termos: ['selic', 'ifix']
      },
      {
        id: 'reservas',
        titulo: 'Reservas (cofrinhos do Mercado Pago)',
        paraQueServe: 'Contar o dinheiro guardado em cofrinhos, que o Open Finance não informa.',
        passos: [
          'O saldo de cada reserva é estimado pelos lançamentos “Dinheiro reservado” e “Dinheiro retirado” do extrato.',
          'Na aba Reservas, informe o saldo real de vez em quando para corrigir a estimativa.',
          'Reservas marcadas “para conferir” geram aviso no Painel.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['reservaEmergencia']
      },
      {
        id: 'exterior',
        titulo: 'Ativos e moeda no exterior',
        paraQueServe: 'Somar investimentos em dólar e outras moedas no patrimônio em reais.',
        passos: [
          'Ao lançar a compra de um ativo no exterior (ex.: AAPL), escolha a moeda: o câmbio vem preenchido com a PTAX do dia e pode ser trocado pelo da corretora.',
          'Os valores são convertidos para reais pela cotação do Banco Central, guardada no computador.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }]
      },
      {
        id: 'emprestimos',
        titulo: 'Empréstimos e financiamentos',
        paraQueServe: 'Ver saldo devedor, parcelas e atrasos dos contratos que vêm do Open Finance.',
        passos: [
          'Abra Empréstimos e financiamentos: financiamentos (imóvel, veículo) e empréstimos aparecem separados.',
          'Cada contrato mostra saldo devedor, parcelas, taxa, CET e pagamentos.',
          'O patrimônio no Painel já desconta esses saldos devedores.'
        ],
        abrir: [{ rotulo: 'Abrir Empréstimos e financiamentos', rota: '/emprestimos' }],
        termos: ['cet']
      }
    ]
  },
  {
    id: 'configuracoes',
    titulo: 'Configurações',
    resumo: 'Parametrização, aparência, segurança e os erros do sistema.',
    secoes: [
      {
        id: 'parametrizacao',
        titulo: 'Parametrização',
        paraQueServe: 'Ajustar o comportamento do site sem mexer em arquivo nenhum.',
        passos: [
          'Abra Configurações › Parametrização.',
          'Cada parâmetro mostra o que faz, a faixa válida e o padrão: sincronização, Painel, segurança do login, orçamentos, metas, transações e erros.',
          'Mude o valor e salve: vale na hora.',
          'O bloco “Sistema” mostra a versão, os caminhos e o tamanho do banco.'
        ],
        abrir: [{ rotulo: 'Abrir a Parametrização', rota: '/configuracoes', params: { aba: 'parametrizacao' } }]
      },
      {
        id: 'aparencia',
        titulo: 'Aparência',
        paraQueServe: 'Deixar o site com as suas cores.',
        passos: [
          'Abra Configurações › Aparência.',
          'Escolha uma paleta pronta (cada uma tem versão clara e escura) ou uma cor personalizada.',
          'A escolha fica salva neste navegador.'
        ],
        abrir: [{ rotulo: 'Abrir a Aparência', rota: '/configuracoes', params: { aba: 'aparencia' } }]
      },
      {
        id: 'seguranca',
        titulo: 'Segurança',
        paraQueServe: 'Trocar a senha e ver os acessos recentes.',
        passos: [
          'Abra Configurações › Segurança.',
          'Em “Alterar senha”, informe a atual e a nova duas vezes. As outras sessões abertas são encerradas.',
          '“Acessos recentes” mostra logins, falhas e bloqueios.'
        ],
        abrir: [{ rotulo: 'Abrir a Segurança', rota: '/configuracoes', params: { aba: 'seguranca' } }]
      },
      {
        id: 'erros-do-sistema',
        titulo: 'Erros do sistema',
        paraQueServe: 'Ver o que deu errado (sincronização, cotações, telas) sem procurar em logs.',
        passos: [
          'Abra Configurações › Erros.',
          'Filtre por situação, origem, nível e período. O mesmo erro repetido aparece uma vez, com quantas vezes aconteceu.',
          '“detalhe” mostra a explicação técnica; “Resolvido” tira da lista (e “Reabrir” traz de volta).',
          'Tokens, senhas, CPF e valores são removidos antes de gravar.'
        ],
        abrir: [{ rotulo: 'Abrir os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }]
      }
    ]
  },
  {
    id: 'privacidade',
    titulo: 'Privacidade, backup e atualização',
    resumo: 'O que fica no computador, o que sai, como fazer backup e atualizar.',
    secoes: [
      {
        id: 'o-que-sai',
        titulo: 'O que fica e o que sai do computador',
        paraQueServe: 'Saber exatamente para onde vão os seus dados.',
        passos: [
          'Fica no computador: todas as transações, contas, categorias, metas, orçamentos e a carteira.',
          'Sai, por HTTPS: a conversa com a Pluggy (Open Finance), datas para a taxa Selic e a cotação de moedas no Banco Central, e códigos de ativos para as cotações (Yahoo Finance e brapi).',
          'Não sai: valores, quantidades e dados pessoais.',
          'Do Pix e da TED, só fica guardado o meio e se quem pagou e quem recebeu são a mesma pessoa — nunca nome ou CPF de terceiros.'
        ]
      },
      {
        id: 'ocultar-valores',
        titulo: 'Ocultar valores',
        paraQueServe: 'Mostrar a tela a alguém sem mostrar quanto você tem.',
        passos: [
          'Clique no botão do olho na barra de cima.',
          'Todos os valores ficam escondidos, inclusive nas frases dos gráficos.',
          'Clique de novo para mostrar.'
        ]
      },
      {
        id: 'backup',
        titulo: 'Backup',
        paraQueServe: 'Não perder os seus dados se o computador der problema.',
        passos: [
          'Rode o script scripts/4-backup.sh (de preferência agendado) para copiar o banco para a pasta “backups”.',
          'Copie de vez em quando a pasta “backups” para um disco externo ou outro lugar seguro.',
          'Ative a criptografia do disco (BitLocker no Windows, LUKS no Linux): o arquivo do banco não é criptografado.'
        ]
      },
      {
        id: 'atualizar',
        titulo: 'Atualizar e parar o sistema',
        paraQueServe: 'Manter o site em dia e saber desligar tudo.',
        passos: [
          'Para atualizar, rode scripts/3-publicar-atualizar.sh (ou o instalador do Windows de novo): ele faz backup do banco antes.',
          'Para parar tudo: parar-windows.cmd no Windows ou scripts/parar.sh no Linux.',
          'De tempos em tempos, rode scripts/1-preparar-ambiente.sh para receber as correções de segurança do .NET.'
        ]
      },
      {
        id: 'problemas-comuns',
        titulo: 'Problemas comuns',
        paraQueServe: 'Resolver sozinho os tropeços mais frequentes.',
        passos: [
          'O navegador alerta sobre o certificado: rode o 3-publicar-atualizar.sh de novo; ele recoloca o certificado nos confiáveis.',
          '“Não foi possível falar com a API”: confira se o serviço Financas Local está rodando.',
          'O Client Secret da Pluggy não abre mais (depois de trocar de computador): informe o Client Secret de novo em Configurações.',
          'Algo deu errado e você não sabe o quê: veja Configurações › Erros.'
        ],
        abrir: [{ rotulo: 'Ver os erros do sistema', rota: '/configuracoes', params: { aba: 'erros' } }]
      }
    ]
  }
];

/** Capítulo (e seção) do manual da tela de uma URL ("/transacoes?x=1" → transações); null fora das telas do menu. */
export function ajudaDaUrl(url: string): { capitulo: string; secao?: string } | null {
  const tela = url.split(/[/?#]/).filter(Boolean)[0] ?? '';
  return AJUDA_POR_ROTA[tela] ?? null;
}

/**
 * "Ajuda" na barra de cima: a seção do manual que explica a tela aberta (pelo primeiro trecho da rota).
 * Toda rota do menu precisa estar aqui (teste de regressão).
 */
export const AJUDA_POR_ROTA: Record<string, { capitulo: string; secao?: string }> = {
  painel: { capitulo: 'relatorios', secao: 'painel' },
  anual: { capitulo: 'relatorios', secao: 'visao-anual' },
  transacoes: { capitulo: 'transacoes' },
  contas: { capitulo: 'contas' },
  orcamentos: { capitulo: 'orcamentos' },
  metas: { capitulo: 'metas' },
  'contas-a-pagar': { capitulo: 'contas-a-pagar' },
  investimentos: { capitulo: 'investimentos' },
  emprestimos: { capitulo: 'investimentos', secao: 'emprestimos' },
  relatorios: { capitulo: 'relatorios', secao: 'relatorios-tela' },
  projecao: { capitulo: 'relatorios', secao: 'projecao' },
  categorias: { capitulo: 'categorias' },
  configuracoes: { capitulo: 'configuracoes' }
};
