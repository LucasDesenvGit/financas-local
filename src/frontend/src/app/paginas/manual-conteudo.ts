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
          'À esquerda fica o menu com todas as telas, separadas por assunto: o Painel no topo; Movimentação (Transações, Contas e cartões, Contas a pagar); Planejamento (Orçamentos, Metas); Patrimônio (Investimentos, Empréstimos e financiamentos); Análises (Relatórios, Visão anual, Projeção); Ajustes (Categorias e regras, Configurações); e o Manual no rodapé.',
          'Clique no nome de um grupo (ou use Enter ou Espaço) para mostrar ou esconder as telas dele. A seta indica se o grupo está aberto (para baixo) ou fechado (para o lado), e vários grupos podem ficar abertos ao mesmo tempo. Na primeira vez, todos vêm abertos; feche os que você não usa. A escolha fica salva neste navegador.',
          'Ao entrar numa tela, o grupo dela abre sozinho. Se você fechar o grupo da tela em que está, o nome dele fica destacado, com um ponto, para mostrar onde você está.',
          'No celular, o menu abre pelo botão ☰ no topo. Tocar no nome de um grupo só mostra ou esconde as telas dele; tocar numa tela leva até ela e fecha o menu. Para fechar sem escolher uma tela, toque no ✕, toque fora do menu ou arraste-o para a esquerda.',
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
    id: 'corrigir-seus-dados',
    titulo: 'Corrigir seus dados',
    resumo: 'Depois de atualizar o site: o que já se acertou sozinho e o passo a passo para deixar tudo igual aos apps dos bancos.',
    secoes: [
      {
        id: 'o-que-ja-se-acertou',
        titulo: 'O que já se acertou sozinho',
        paraQueServe: 'Saber o que não precisa ser refeito à mão depois da atualização.',
        passos: [
          'Estorno no cartão (compra devolvida, cashback) deixou de contar como receita e passou a abater o gasto da categoria.',
          'Pix e TED para outra pessoa contam como despesa ou receita; os que são entre contas suas ficam fora dos gastos.',
          'Pagamento de fatura que o banco mandou como boleto ou “outros” saiu dos gastos quando o crédito do mesmo valor aparece no cartão.',
          'Aplicação e resgate de RDB, CDB e poupança, e a compra e a venda de Méli Dólar, foram para “Investimentos” e não contam mais como gasto nem receita.',
          'A projeção não conta mais as parcelas em dobro, e a fatura do mês do cartão segue o ciclo do banco.',
          'O lançamento pendente que o banco efetivou com outro código sai na próxima sincronização.'
        ],
        abrir: [{ rotulo: 'Ver as transações', rota: '/transacoes' }],
        dicas: ['O que você categorizou à mão não foi mexido: as correções só valem para o que veio automático.']
      },
      {
        id: 'passo-1-sincronizar',
        titulo: '1. Sincronize os bancos',
        paraQueServe: 'Trazer os dados novos que as correções usam: o valor reservado que o banco informa e as pendências que já viraram compra.',
        passos: [
          'Abra Configurações › Open Finance e clique em “Sincronizar agora”.',
          'Espere a barra de cima mostrar “Atualizado agora”.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }]
      },
      {
        id: 'passo-2-reservas',
        titulo: '2. Confira as reservas, as caixinhas e o Méli Dólar',
        paraQueServe: 'Fazer o dinheiro guardado entrar certo no patrimônio.',
        passos: [
          'Abra Investimentos, na aba Reservas.',
          'Reserva com “informado pelo banco”: já está certa, não precisa fazer nada. A linha “Reservado” com “restante do total do banco” também não se confere: ela se ajusta quando você confere as outras.',
          'Reserva “estimada” ou “para conferir”: clique em “conferir saldo” e digite o valor que o app do banco mostra, do jeito que aparece lá (ex.: 10.123,42).',
          'Méli Dólar: em “conferir saldo”, escolha se vai digitar em reais ou em dólares, como o app do Mercado Pago mostra.',
          'Confira depois o patrimônio no Painel: ele passa a somar essas reservas.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['ptax']
      },
      {
        id: 'passo-3-trocos',
        titulo: '3. Tire os trocos das compras de FIIs dos rendimentos',
        paraQueServe: 'Fazer os rendimentos dos FIIs baterem com o app da corretora.',
        passos: [
          'Se o Painel avisa “N créditos de troco ou venda de FIIs e ações contando como rendimento”, clique em “Conferir investimentos” no aviso (ou no botão de Investimentos).',
          'Revise a lista “Troco ou venda contando como rendimento” e clique em “Corrigir selecionados”.',
          'Nada muda sem você confirmar, e o aviso tem “Desfazer” por 10 minutos.'
        ],
        abrir: [{ rotulo: 'Abrir Conferir investimentos', rota: '/investimentos', params: { conferir: '1' } }],
        termos: ['trocoDeCompra']
      },
      {
        id: 'passo-4-limite',
        titulo: '4. Responda a pergunta do limite do cartão',
        paraQueServe: 'Não tratar como dívida o limite que você tirou do cartão no app.',
        passos: [
          'Abra Contas e cartões.',
          'O cartão em que você reduziu o limite no app pergunta “Você reduziu o limite no app?”. Clique em “responder” e em “Sim, ajustei o limite”.',
          'Se a diferença for compra em processamento ou parcela, responda “Não”.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['limiteUsado']
      },
      {
        id: 'passo-5-nome',
        titulo: '5. Informe o seu nome como aparece nos Pix',
        paraQueServe: 'Separar o Pix para outra conta sua do Pix para outra pessoa, quando o banco não informa.',
        passos: [
          'Abra Configurações › Open Finance.',
          'Em “Seu nome como aparece nos Pix”, digite o primeiro e o último nome (ex.: como aparece no comprovante) e clique em “Salvar nome”.',
          'As transferências são revistas na hora.'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }]
      },
      {
        id: 'passo-6-assinaturas',
        titulo: '6. Cadastre as assinaturas e os débitos automáticos',
        paraQueServe: 'Saber o que cobra em cada cartão e conta todo mês, e ser avisado quando algo muda.',
        passos: [
          'Abra Contas a pagar e veja as “Sugestões do extrato”: assinaturas nos cartões, débitos automáticos e contas que se repetem.',
          'Clique em “Cadastrar” nas que são suas; a forma de cobrança e o texto já vêm preenchidos. “ignorar” tira a sugestão.',
          'Na aba “Assinaturas e recorrentes”, confira onde cada uma cai e quanto vai por mês.'
        ],
        abrir: [
          { rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' },
          { rotulo: 'Abrir Assinaturas e recorrentes', rota: '/contas-a-pagar', params: { aba: 'recorrentes' } }
        ]
      },
      {
        id: 'passo-7-conferir',
        titulo: '7. Confira com os apps dos bancos',
        paraQueServe: 'Ter certeza de que o site bate com cada banco, e saber por que quando não bate.',
        passos: [
          'Abra Contas e cartões e clique em “Conferir com o banco”.',
          'Com o app de cada banco aberto, digite o saldo de cada conta, a fatura do mês de cada cartão e o valor de cada investimento e reserva.',
          '“bate”: está certo. Quando não bate, a pista com o mesmo valor da diferença vem primeiro, marcada “Explica a diferença”, com o link “ver” para corrigir.'
        ],
        abrir: [{ rotulo: 'Conferir com o banco', rota: '/conferir-banco' }],
        dicas: [
          'Fatura diferente do app? Veja se o app mostra a dívida total (com as parcelas futuras) ou compras que ainda estão pendentes: as duas aparecem como pista.',
          'Faça isso logo depois de sincronizar: o banco atualiza o Open Finance mais ou menos uma vez por dia.'
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
          'Em Configurações › Open Finance, cole o Item ID no campo “Adicionar conexão” e clique em “Adicionar”.',
          'Cada conexão mostra a situação em português: “Atualizada”, “Desatualizada”, “Erro de login no banco” ou “Precisa de ação sua” (aí é preciso entrar no Meu Pluggy). Os botões “Sincronizar”, “Pausar” e “Remover” ficam na própria linha.',
          'Clique em “Sincronizar agora”. Na primeira vez são buscados os últimos meses de histórico.',
          'Confira em Contas e cartões se as contas e cartões apareceram.',
          'Pelo Meu Pluggy, toda conexão tem o mesmo nome de conector (“MeuPluggy”). O site mostra o banco, tirado das contas da conexão, e o conector ao lado (“via MeuPluggy”).'
        ],
        abrir: [{ rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }, { rotulo: 'Ver as contas', rota: '/contas' }],
        dicas: [
          'Quanto histórico a primeira carga busca fica em Configurações › Parametrização › Sincronização.',
          'A conexão aparece com o nome errado? Escolha o banco certo numa das contas dela, em Contas e cartões › editar.'
        ]
      },
      {
        id: 'sincronizacao',
        titulo: 'Sincronização automática',
        paraQueServe: 'Manter as transações em dia sem clicar em nada.',
        passos: [
          'A sincronização roda sozinha de tempos em tempos (padrão: a cada 6 horas).',
          'A cada vez, ela relê os últimos dias (padrão: 30) para pegar o que mudou ou entrou atrasado.',
          'O status aparece na barra de cima: “Atualizado há…”, “Sincronizando…” ou “1 conexão com erro”.',
          'Para sincronizar na hora, use “Sincronizar agora” em Configurações › Open Finance.',
          'Lançamento pendente que o banco efetivou com outro código sai sozinho, mesmo que seja de antes dos dias relidos: o site acha a efetivada do mesmo valor na mesma conta (até 10 dias depois), para não contar em dobro.',
          'Conta em outra moeda (ex.: em dólar) gera um aviso no Painel: o site soma as contas como reais, sem conversão.'
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
          'Conta que não usa mais: marque “Arquivada” em vez de excluir.',
          'Para ver as arquivadas, ligue o interruptor “Mostrar arquivadas” no topo da tela; elas aparecem com o selo “Arquivada”.'
        ],
        abrir: [
          { rotulo: 'Abrir Contas e cartões', rota: '/contas' },
          { rotulo: 'Ver também as arquivadas', rota: '/contas', params: { arquivadas: '1' } }
        ]
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
          '“Fatura de <mês>” é a fatura do cartão que fecha no mês: as compras desde o fechamento anterior e a parcela das compras parceladas, menos os estornos. Depois de fechada, vale o total que o banco informou.',
          'Abaixo dela aparecem quando a fatura fecha, quando vence e a situação (paga, fechada ou em aberto). Ela aparece como paga quando o banco informa o pagamento ou quando o pagamento (por exemplo, “Pagamento recebido”) está nas transações do cartão, entre o vencimento anterior e o vencimento dela. Pagar antes do fechamento também conta.',
          'Depois que a fatura do mês fecha, aparece embaixo a “Próxima fatura”: o que já entrou na fatura seguinte, que ainda está aberta, com o dia em que ela fecha e vence.',
          '“Dívida total” é tudo o que você deve no cartão hoje, inclusive as parcelas que ainda vão vencer.',
          'A barra mostra quanto do limite está em uso e quanto ainda está disponível (ou quanto passou do limite).',
          '“Melhor dia de compra” é o dia seguinte ao fechamento: o que você compra a partir dele só entra na fatura seguinte.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['faturaDoMes', 'dividaCartao', 'melhorDiaCompra']
      },
      {
        id: 'conferir-banco',
        titulo: 'Conferir com o banco',
        paraQueServe: 'Ver se o site bate com o app de cada banco e, quando não bate, por quê.',
        passos: [
          'Em Contas e cartões, clique em “Conferir com o banco”.',
          'Abra o app de cada banco e digite o valor que aparece lá: o saldo de cada conta, a fatura do mês de cada cartão e o valor de cada investimento e reserva.',
          'O site compara na hora: “bate”, ou quanto o app (ou o site) mostra a mais.',
          'Quando não bate, aparecem as pistas: quando o banco atualizou, reservas da conta, lançamentos pendentes, parcelas, estornos e compras repetidas. A pista com o mesmo valor da diferença vem primeiro, marcada “Explica a diferença”.',
          'Cada pista tem o link “ver” para a tela com o detalhe. Nada é gravado: os valores digitados ficam só na tela.'
        ],
        abrir: [{ rotulo: 'Conferir com o banco', rota: '/conferir-banco' }],
        dicas: ['Faça isso depois de sincronizar: o banco atualiza o Open Finance mais ou menos uma vez por dia, e o app mostra o que aconteceu depois disso.']
      },
      {
        id: 'limite-reduzido',
        titulo: 'Limite reduzido no app do banco',
        paraQueServe: 'Não confundir o limite que você tirou do cartão no app com dívida.',
        passos: [
          'Se você reduz o limite no app do banco (por segurança, enquanto não usa), o “limite usado” que o banco informa fica bem maior que a dívida do cartão.',
          'Quando isso acontece, o cartão em Contas e cartões pergunta: “Você reduziu o limite no app?”. Clique em “responder” (ou em “editar”).',
          '“Sim, ajustei o limite”: o site usa o limite que ficou no app, e o cartão mostra “limite reduzido por você no app”.',
          '“Não”: a diferença conta como compromisso (compras em processamento ou parcelas). A barra passa a usar o limite aprovado e inclui essa diferença.',
          'Mudou de ideia? Em “editar”, clique em “desfazer”. Depois de um “não”, a pergunta só volta se a diferença mudar bastante.'
        ],
        abrir: [{ rotulo: 'Abrir Contas e cartões', rota: '/contas' }],
        termos: ['limiteUsado']
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
          'Clique na linha da transação. Pelo teclado, chegue ao nome dela com Tab e aperte Enter.',
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
          'As regras ficam em Categorias › Regras automáticas › “Regras de descrição”. “+ Regra de descrição” cria uma direto (com prévia de quantas transações renomeia), e dá para editar, desligar ou excluir.',
          'Mesma descrição do banco e valores diferentes (ex.: duas contas do condomínio)? Na regra de descrição, preencha “E o valor”: “De” e “Até” (um dos dois basta). Crie uma regra para cada valor, cada uma com o seu nome. A regra com valor vem antes das outras.'
        ],
        abrir: [
          { rotulo: 'Ver as descrições pouco informativas', rota: '/transacoes', params: { generica: '1' } },
          { rotulo: 'Ver as regras de descrição', rota: '/categorias' }
        ],
        dicas: [
          'Para PIX e boletos com número que muda todo mês, use “contiver” e deixe só a parte fixa (o site já sugere o texto sem os números).',
          'A busca acha a transação pelo nome novo e pela descrição do banco. A categoria continua vindo da descrição do banco (as regras de categoria não mudam).',
          'Uma transação renomeada sai do filtro “Descrição pouco informativa”.',
          'O valor da regra é sem o sinal: “De R$ 150,00 Até R$ 250,00” pega a despesa de R$ 200,00. Deixe dar uma folga (a conta de R$ 200 pode vir R$ 203 com juros).'
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
      },
      {
        id: 'selecionar-varias',
        titulo: 'Selecionar várias transações',
        paraQueServe: 'Aplicar a mesma ação (categoria, nome, emergência…) a várias transações de uma vez.',
        passos: [
          'Clique na caixa à esquerda da linha (ou em qualquer lugar daquela célula). A linha fica destacada.',
          'Para um intervalo: marque uma, segure Shift e clique em outra. Todas entre as duas são marcadas (ou desmarcadas).',
          'A caixa do cabeçalho seleciona todas desta página. Com só algumas marcadas, ela mostra um traço; clique nela para marcar todas.',
          'A barra de ações aparece no topo e acompanha a rolagem. Ela diz quantas estão selecionadas e quantas estão em outras páginas.',
          'Esc ou “✕ Limpar seleção” desmarcam tudo.'
        ],
        dicas: ['Trocar de página mantém a seleção: dá para juntar transações de várias páginas e aplicar a ação de uma vez.'],
        erros: ['Trocar o filtro (busca, período, categoria…) limpa a seleção, com um aviso. Assim nenhuma ação em lote pega uma transação que não aparece no filtro atual.'],
        abrir: [{ rotulo: 'Abrir Transações', rota: '/transacoes' }]
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
          'Mandou dinheiro para uma conta sua que não está conectada? O site não vê o par: crie uma categoria do tipo transferência (ex.: “Entre minhas contas”) e use nela. Outras categorias do tipo transferência ficam sempre fora.',
          'Seu banco não informa se o Pix foi para você mesmo? Em Configurações › Open Finance, preencha “Seu nome como aparece nos Pix”: Pix e TED que citam o seu primeiro e último nome passam a contar como entre contas suas.',
          'Pagamento de fatura que o banco mandou como boleto ou “outros” fica fora dos gastos sozinho quando o crédito do mesmo valor aparece num cartão seu em até 5 dias. Sem esse crédito, ele fica como está: “fatura” também pode ser conta de telefone ou de luz.'
        ],
        abrir: [{ rotulo: 'Ver os pagamentos de fatura', rota: '/transacoes', params: { lancamento: 'fatura' } },
          { rotulo: 'Abrir Configurações › Open Finance', rota: '/configuracoes' }],
        termos: ['tipoLancamento']
      },
      {
        id: 'estornos',
        titulo: 'Estornos no cartão',
        paraQueServe: 'Entender por que um estorno aparece em verde, mas não aumenta a sua receita.',
        passos: [
          'No cartão, todo crédito que não é o pagamento da fatura é estorno: compra devolvida, “crédito de confiança”, cashback.',
          'O estorno abate o gasto da categoria no mês (ex.: compra de R$ 100 em Compras + estorno de R$ 100 = R$ 0 em Compras) e não conta como receita.',
          'O crédito do pagamento da fatura no cartão (“Pagamento recebido”) fica fora dos gastos, mesmo quando chega sem a categoria “Pagamento de fatura”.',
          'Estorno que o banco manda como transferência (ex.: “Estorno de Uber - NuPay”) ganha a categoria do texto (Uber → Transporte) ou fica sem categoria para você escolher.',
          'Os estornos que já estavam no site foram acertados sozinhos.'
        ],
        abrir: [{ rotulo: 'Ver os estornos', rota: '/transacoes', params: { lancamento: 'cartao' } }],
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
          'A Visão anual e a sugestão de orçamentos usam isso na divisão 50/30/20: até 50% para essenciais, 30% para estilo de vida e pelo menos 20% para guardar.',
          'Na Visão anual, clique em Essenciais, Estilo de vida ou Guardado (na barra ou na legenda) para ver em Transações o que forma cada parte.'
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
          'O Painel junta os orçamentos estourados num card, com o link para as despesas de cada categoria. Dá para ocultar até o mês que vem.',
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
        paraQueServe: 'Saber quanto guardar este mês em cada meta, pelo dinheiro que de fato entra e sai, contando a fatura do cartão.',
        passos: [
          'Com pelo menos 3 meses de extrato, o topo de Metas mostra a sobra do mês (receitas − fatura que vence − outras despesas) e quanto guardar no total.',
          'Em “Guardar X% da sobra”, escolha quanto da sobra vai para as metas; o resto fica livre para imprevistos.',
          'Use ↑ e ↓ em cada meta para definir a prioridade.',
          'Cada meta mostra “Este mês: guarde R$ X” e, embaixo, os degraus dos meses seguintes: menos nos meses de fatura alta, mais quando ela cai.',
          'Em “Consigo guardar outro valor”, diga quanto dá para guardar este mês: o resto do plano é refeito na hora e mostra se ainda fecha no prazo.',
          'Quando o prazo não cabe, a meta mostra a data provável, quanto os próximos meses precisariam e onde mexer (prazo, valor ou gastos).',
          '“Plano mês a mês” mostra os próximos 12 meses; “Como calculei” explica a conta.'
        ],
        abrir: [{ rotulo: 'Abrir Metas', rota: '/metas' }, { rotulo: 'Ver a Projeção', rota: '/projecao' }],
        termos: ['sobraDoCaixa', 'parteDaSobra', 'recomendacaoMetas'],
        dicas: [
          'Dinheiro que vai para cofrinho ou investimento precisa estar como transferência; lançado como despesa, ele diminui a sobra.',
          'O valor de “Consigo guardar” é só uma simulação: para valer, registre o aporte na meta.'
        ]
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
    resumo: 'Cadastrar vencimentos, pagar, data limite, assinaturas no cartão, débitos automáticos e as sugestões que vêm do extrato.',
    secoes: [
      {
        id: 'cadastrar-conta',
        titulo: 'Cadastrar uma conta a pagar',
        paraQueServe: 'Ser lembrado dos vencimentos no Painel.',
        passos: [
          'Abra Contas a pagar e clique em “+ Nova conta”.',
          'Informe a descrição, o valor, o “Próximo vencimento” e a recorrência (mensal, anual ou única).',
          'Para conta que acaba (escola até dezembro, parcelas de um curso), preencha “Até (opcional)” ou “quantos pagamentos faltam”.',
          'Escolha a categoria e, se quiser, a conta de onde sai o pagamento.',
          'Em “Como é cobrada”, diga se você paga à mão (boleto, Pix), se é débito automático ou se cai no cartão de crédito. Escolha também o cartão ou a conta onde cai.',
          'Use “Observação” para o que quiser lembrar (ex.: “conta da família”, “cancelar em março”).'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }]
      },
      {
        id: 'cobrada-sozinha',
        titulo: 'Assinaturas e débitos automáticos (cobrados sozinhos)',
        paraQueServe: 'Saber qual assinatura cai em qual cartão e qual débito automático sai de qual conta, sem precisar clicar em “Pagar”.',
        passos: [
          'No cadastro, escolha “débito automático” ou “no cartão de crédito” e preencha o “Texto da cobrança no extrato”: o que a descrição do banco contém (ex.: NETFLIX).',
          'Quando a cobrança com esse texto chega no extrato, a conta se dá como paga sozinha: o último pagamento vai para a data dela e o vencimento avança.',
          'Por isso a conta cobrada sozinha não tem o botão “Pagar”. Sem o texto, ela continua com o “Pagar”.',
          'A lista mostra, em cada conta, como ela é cobrada e onde cai.',
          'O que cai no cartão não aparece nos “Próximos vencimentos” do Painel: entra na fatura, que já está lá.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        dicas: ['O texto vale pela descrição original do banco: renomear a transação (Transações › renomear) não atrapalha.']
      },
      {
        id: 'assinaturas-recorrentes',
        titulo: 'Assinaturas e recorrentes: onde cada cobrança cai',
        paraQueServe: 'Ver de uma vez qual assinatura cai em qual cartão e qual débito automático sai de qual conta, e quanto vai por mês.',
        passos: [
          'Em Contas a pagar, abra a aba “Assinaturas e recorrentes”.',
          'No topo: quanto vai por mês e por ano nas cobranças que se repetem (a anual entra dividida por 12).',
          'Embaixo, um bloco por cartão ou conta, com o total por mês e cada cobrança: como é cobrada, o dia, a última cobrança e o valor.',
          'Para achar onde cai uma assinatura, digite o nome em “Onde cai…?” (ex.: netflix).',
          'Filtre por conta ou cartão e pela forma de cobrança; “Mostrar as encerradas” traz também as que acabaram. Os filtros aparecem embaixo, com ✕ para tirar.',
          '“ver as cobranças” abre Transações com as cobranças do último ano, na conta ou cartão dela.',
          'Em Contas e cartões, cada cartão e conta mostra a linha “N assinaturas · R$ X/mês”, que abre esta aba já filtrada.'
        ],
        abrir: [{ rotulo: 'Abrir Assinaturas e recorrentes', rota: '/contas-a-pagar', params: { aba: 'recorrentes' } }],
        dicas: ['Trocou de cartão? Filtre pelo cartão antigo para ver o que ainda cobra nele e precisa ser trocado nos serviços.']
      },
      {
        id: 'avisos-recorrentes',
        titulo: 'Avisos das recorrentes',
        paraQueServe: 'Saber quando uma cobrança que se repete não veio, mudou de cartão, mudou de valor, ou cai num cartão que você arquivou.',
        passos: [
          'Os avisos aparecem no topo de Contas a pagar, com a ação ao lado quando dá para resolver no site.',
          '“Não veio”: a cobrança automática passou 5 dias do vencimento e não apareceu no extrato. Pode ter sido cancelada, ou o cartão recusou. Também aparece no Painel.',
          '“Mudou de cartão”: a última cobrança veio em outra conta ou cartão. “Atualizar o catálogo” passa a conta a pagar para lá.',
          '“Aumentou” (ou baixou): a última cobrança passou 5% do valor cadastrado. “Atualizar o valor” grava o novo. Conta que muda de valor todo mês, como a de luz, não gera esse aviso.',
          '“Cartão arquivado”: a cobrança ainda cai num cartão ou conta que você arquivou. Troque nos serviços. Também aparece no Painel.',
          'Ao marcar “Arquivada” num cartão ou conta em Contas e cartões, a tela lista o que ainda cobra ali, antes de salvar.'
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
        paraQueServe: 'Cadastrar em um clique os pagamentos que se repetem todo mês (ou todo ano).',
        passos: [
          'Em “Sugestões do extrato”, aparecem boletos, débitos automáticos e Pix que se repetem e ainda não estão na lista.',
          'Também aparecem as assinaturas dos cartões: a mesma cobrança, todo mês, com o valor igual (até 5% de diferença nas últimas). Parcelas e gastos frequentes (mercado, aplicativo de corrida) ficam de fora.',
          'Assinatura anual aparece quando o histórico importado tem as duas últimas cobranças, com um ano de diferença. Se não tiver, cadastre à mão.',
          'Cada sugestão diz como parece ser cobrada (à mão, débito automático ou no cartão) e já vem com o texto da cobrança.',
          'Use “ver pagamentos” para conferir no extrato.',
          '“Cadastrar” abre o formulário já preenchido; “ignorar” tira a sugestão.',
          'Pode trocar o nome ao cadastrar (ex.: “PIX ENVIADO LIVEPIX LTDA” vira “Manwha”): a conta guarda de qual pagamento do banco ela veio, e a sugestão não volta.',
          'Conta que você cadastrou à mão também tira a sugestão, quando o valor (até 15% de diferença), o dia do vencimento (até 3 dias) e a conta batem.',
          'Se não der para ter certeza (duas contas parecidas, ou a conta a pagar sem a conta escolhida), a sugestão mostra “parece ser …”. Clique em “é a mesma” para ligar as duas: a sugestão some e não volta.',
          'Conta em débito automático ou no cartão que dá baixa sozinha também fica ligada ao pagamento do banco na primeira cobrança.'
        ],
        abrir: [{ rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }],
        erros: ['Clicou em “é a mesma” por engano? Use “Desfazer” no aviso que aparece logo depois: a sugestão volta. Excluir a conta a pagar também a traz de volta.']
      },
      {
        id: 'mesmo-nome-valores-diferentes',
        titulo: 'Duas contas com a mesma descrição do banco',
        paraQueServe: 'Separar cobranças que chegam do banco com o mesmo nome e só mudam no valor, como duas contas do condomínio.',
        passos: [
          'Em Categorias e regras › Regras automáticas › “Regras de descrição”, clique em “+ Regra de descrição”.',
          'Em “Texto”, escreva a descrição do banco (ex.: CONDOMINIO). Em “E o valor”, a faixa da primeira conta (ex.: De R$ 800,00 Até R$ 900,00). Em “Mostrar como”, o nome dela (ex.: “Condomínio — taxa”). Salve.',
          'Repita para a outra conta, com a faixa e o nome dela (ex.: Até R$ 250,00, “Condomínio — fundo de reserva”).',
          'Em Contas a pagar, as “Sugestões do extrato” passam a mostrar as duas separadas, cada uma com o seu nome e o texto da cobrança igual a ele.',
          'Já tinha cadastrado as contas? Abra cada uma e troque o “texto da cobrança” pelo nome da regra (ex.: CONDOMINIO — TAXA). Assim a baixa sozinha e os avisos não confundem uma com a outra.'
        ],
        abrir: [
          { rotulo: 'Abrir Categorias e regras', rota: '/categorias' },
          { rotulo: 'Abrir Contas a pagar', rota: '/contas-a-pagar' }
        ],
        erros: [
          'Com um texto da cobrança que as duas têm (ex.: só CONDOMINIO), a baixa sozinha e o aviso “aumentou” misturam as duas: use o nome inteiro da regra.',
          'Conta que mudou de valor e saiu da faixa volta ao nome do banco. Edite a faixa da regra: as transações são renomeadas de novo.'
        ]
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
          'No topo, o card “orçamentos estourados” junta as categorias que passaram do limite no mês, as piores primeiro: quanto passou, em reais e em %, e uma barra (a parte clara é o limite). Clique no nome da categoria para ver as despesas dela no mês. “Ver todos” mostra as demais.',
          'Abaixo dele, o card de avisos mostra o resto do que precisa de atenção (conta vencida, consentimento expirando, falha de sincronização, erros).',
          '“Ocultar até o mês que vem” recolhe o card dos orçamentos numa linha só. Ele abre de novo sozinho no mês seguinte, se outra categoria estourar, ou se uma já estourada piorar de faixa (passar de 120%, 150%, 200% ou 300%). “Ocultar até aparecer um novo” faz o mesmo com os outros avisos. Fica a linha com a quantidade, e “Mostrar” abre de novo.',
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
          'Maiores despesas do período. Clique numa delas para abrir Transações no dia e na conta dela, com a despesa destacada.'
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
          '“Taxa da B3 que falta nas compras”: em cada dia de compras, o que saiu da conta menos o troco e menos cotas × preço é a taxa da bolsa. Ela entra no custo e no preço médio da compra (o “valor final da compra” do app). Só aparece depois que o troco teve tempo de voltar (5 dias).',
          'Desmarque o que não quiser mudar e clique em “Corrigir selecionados”. O aviso tem “Desfazer” por 10 minutos.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos', rota: '/investimentos' }],
        dicas: [
          'Quando a carteira tem cotas diferentes da corretora, a própria tela de Investimentos avisa no topo.',
          'O troco “sem dúvida” vai sozinho para “Investimentos” depois da sincronização: o dia de compras tem pelo menos 5 dias, todas as compras têm operação e a conta do dia sobra só a taxa da B3. O Painel avisa, com “Ver em Transações”, “Desfazer” e “Ok”.',
          'O resto (troco de compra recente, venda, ou o que não fecha a conta do dia) o Painel avisa e leva à conferência: você vê a prévia e escolhe.',
          'A compra lançada por aqui fica ligada ao débito do extrato e não trava as próximas compras que chegam pelo Open Finance.'
        ],
        erros: [
          'Mudar todos os “Valor recebido de Investimentos” para Investimentos de uma vez: os proventos deixam de contar como receita. A conferência separa o troco do rendimento.'
        ],
        termos: ['trocoDeCompra', 'taxaB3']
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
        id: 'mes-a-mes',
        titulo: 'Mês a mês dos FIIs',
        paraQueServe: 'Acompanhar cada mês da carteira de FIIs como no app da corretora: quanto entrou, quanto rendeu e quanto as cotas valorizaram.',
        passos: [
          'Abra Investimentos › FIIs e desça até “Mês a mês”. Escolha “12 meses” ou “24 meses”.',
          'O gráfico mostra, por mês, os rendimentos e a valorização; os aportes ficam na tabela, para um aporte grande não achatar o resto. A frase acima dele resume o último mês.',
          'Na tabela, o mês mais recente vem primeiro. “Aportes” são as compras do mês; subscrições e vendas aparecem embaixo, à parte.',
          '“Rendimentos” são os proventos que caíram na conta. O troco das compras e o valor das vendas, que a corretora credita com a mesma descrição, ficam de fora.',
          'No mês atual, “a receber (data-com)” é o provento dos fundos com data-com no mês, que costuma cair no mês seguinte.',
          '“por fundo”, embaixo dos rendimentos, abre quanto cada FII pagou no mês. O crédito que vem sem o código do fundo (“Valor recebido de Investimentos”) é ligado ao fundo pelo valor (cotas na data-com × valor por cota, pago até 40 dias depois); o que não liga aparece como “sem fundo identificado”, sem sumir da soma.',
          '“Valorização” é quanto as cotas ganharam ou perderam de preço no mês, sem contar aportes nem rendimentos. “Saldo no fim” é quanto a carteira valia no último dia do mês.',
          '“ver lançamentos”, em Aportes e em Rendimentos, abre Transações no mês e só com os lançamentos que formam aquele número. O filtro aparece em “Filtrando:”; o ✕ o tira, e o voltar do navegador leva de volta aos FIIs. Clicar num mês do gráfico faz o mesmo com os rendimentos.'
        ],
        abrir: [{ rotulo: 'Abrir o mês a mês dos FIIs', rota: '/investimentos', params: { aba: 'fii' } }],
        dicas: [
          'Para os rendimentos baterem com o app, confira os créditos da corretora em “Conferir investimentos”: ali o troco e o rendimento aparecem separados.',
          'Os aportes são cotas × preço, como as “Entradas” do app. A taxa da B3, de alguns centavos por compra, fica no custo e no preço médio: “Conferir investimentos” a calcula pelo extrato e lança com um clique.',
          'Compras lançadas à mão na carteira entram nos aportes, mas não aparecem em “ver lançamentos”: elas não vieram do banco.'
        ],
        erros: [
          'A valorização pode diferir um pouco da do app: o site usa o fechamento do mês de outra fonte de cotação (no mês atual, a cotação de hoje).'
        ],
        termos: ['valorizacaoMes', 'trocoDeCompra']
      },
      {
        id: 'caixinhas',
        titulo: 'Caixinhas, CDBs e outros investimentos do banco',
        paraQueServe: 'Achar a caixinha do Nubank, o CDB do Mercado Pago e o que mais o banco informa pelo Open Finance.',
        passos: [
          'Abra Investimentos, na aba Open Finance. Cada investimento que o banco informa é uma linha, com o saldo, a rentabilidade e a comparação com a Selic.',
          'O nome mostra o produto e o banco: a caixinha do Nubank aparece como “CDB · Nubank”, e o CDB do Mercado Pago como “CDB · Mercado Pago”. Embaixo fica o nome que o banco usa (“no banco: …”), para achar no app.',
          'Clique em “renomear” para dar o seu nome (ex.: “Caixinha viagem”). A sincronização não muda o nome que você escolher.',
          'Para voltar ao nome automático, abra “renomear” e clique em “Voltar ao nome automático” (ou salve com o campo vazio).',
          'A coluna Instituição mostra o banco da conexão.',
          '“Rendimento por mês”, embaixo da tabela, mostra quanto os investimentos renderam em cada mês: quanto o saldo cresceu além do valor aplicado (o que você guardou ou tirou não conta). “por investimento” abre o de cada um.'
        ],
        abrir: [{ rotulo: 'Abrir Investimentos › Open Finance', rota: '/investimentos', params: { aba: 'of' } }],
        dicas: [
          'O rendimento da caixinha fica no saldo dela e não vira lançamento no extrato. Os “Valor recebido de Investimentos” do extrato do Nubank são proventos dos FIIs ou troco das compras (veja “Conferir investimentos”).',
          'O Open Finance manda só o saldo de hoje: o site guarda um por dia desde a atualização que trouxe o “Rendimento por mês”. Os meses de antes não aparecem, e o primeiro conta a partir daquele dia.'
        ]
      },
      {
        id: 'reservas',
        titulo: 'Reservas (cofrinhos do Mercado Pago)',
        paraQueServe: 'Contar no patrimônio o dinheiro guardado em cofrinhos e reservas da conta.',
        passos: [
          'Quando o banco informa pelo Open Finance o valor de uma reserva, pelo nome, ele vale: só essa reserva aparece como “informado pelo banco” e não precisa de conferência.',
          'Quando o banco informa só o total reservado da conta, cada reserva continua estimada e com “conferir saldo”. O que o total passa da soma delas aparece numa linha “Reservado” (“restante do total do banco”): conferir as outras reservas ajusta esse valor.',
          'Reserva do extrato que não está na lista que o banco manda continua estimada e com “conferir saldo”. Se ela já aparece com outro nome, confira esta com zero para não contar duas vezes.',
          'Sem nada informado pelo banco, o saldo de cada reserva é estimado pelos lançamentos “Dinheiro reservado” e “Dinheiro retirado” do extrato.',
          'Na aba Reservas, informe o saldo real de vez em quando para corrigir a estimativa. Um valor muito diferente do esperado pede confirmação antes de salvar.',
          'Reservas marcadas “para conferir” geram aviso no Painel.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        termos: ['reservaEmergencia']
      },
      {
        id: 'meli-dolar',
        titulo: 'Méli Dólar (dólar do Mercado Pago)',
        paraQueServe: 'Contar no patrimônio os dólares comprados no Mercado Pago, que o Open Finance não informa.',
        passos: [
          'Abra Investimentos, na aba Reservas. O Méli Dólar aparece como uma reserva da conta do Mercado Pago, com a quantidade em dólar e a PTAX do dia.',
          'A quantidade vem das compras e vendas do extrato: pelo “MUSD” da descrição, quando o banco manda, ou pelo valor em reais dividido pela PTAX do dia.',
          'O saldo em reais é essa quantidade pela última PTAX. Ele entra no patrimônio e no total investido.',
          'Para acertar o valor, clique em “conferir saldo” e informe o que o app do Mercado Pago mostra. Pode ser em reais ou em dólares: escolha em “Informado em”.',
          'Compra e venda de Méli Dólar ficam na categoria Investimentos e não contam como gasto nem receita.'
        ],
        abrir: [{ rotulo: 'Abrir as reservas', rota: '/investimentos', params: { aba: 'reservas' } }],
        dicas: [
          'A PTAX é a cotação do Banco Central, um pouco diferente da cotação do Mercado Pago. Por isso a estimativa pode ter alguns centavos de diferença; a conferência acerta.',
          'Uma compra ou venda que você colocou à mão em outra categoria continua como está. Para mudar, edite a transação ou use “Reaplicar regras” em Categorias e regras.'
        ],
        termos: ['ptax']
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
          'Todos os valores ficam escondidos, inclusive nas frases dos gráficos. O olho fica riscado e o botão destacado enquanto os valores estão escondidos.',
          'Clique de novo para mostrar.'
        ]
      },
      {
        id: 'backup',
        titulo: 'Backup',
        paraQueServe: 'Não perder os seus dados se o computador der problema.',
        passos: [
          'Rode o script scripts/geral/4-backup.sh (de preferência agendado) para copiar o banco para a pasta “backups”.',
          'Copie de vez em quando a pasta “backups” para um disco externo ou outro lugar seguro.',
          'Ative a criptografia do disco (BitLocker no Windows, LUKS no Linux): o arquivo do banco não é criptografado.'
        ]
      },
      {
        id: 'atualizar',
        titulo: 'Atualizar e parar o sistema',
        paraQueServe: 'Manter o site em dia e saber desligar tudo.',
        passos: [
          'Para atualizar, rode scripts/geral/3-publicar-atualizar.sh (ou o instalador do Windows de novo): ele faz backup do banco antes.',
          'Para parar tudo: parar-windows.cmd no Windows ou scripts/geral/parar.sh no Linux.',
          'De tempos em tempos, rode scripts/geral/1-preparar-ambiente.sh para receber as correções de segurança do .NET.'
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
  configuracoes: { capitulo: 'configuracoes' },
  'conferir-banco': { capitulo: 'contas', secao: 'conferir-banco' }
};
