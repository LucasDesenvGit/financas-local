/**
 * #284: os eventos de "Acessos recentes" (Configurações › Segurança) em português comum. A API grava o nome do evento
 * como está no código ("LoginSucesso", "RegraDescricaoCriada (22)"); aqui ele vira texto de gente (Nielsen 2, sem
 * jargão). O detalhe entre parênteses continua. Evento novo que não estiver no mapa aparece como veio, para nada sumir.
 */
const NOMES_EVENTOS: Record<string, string> = {
  LoginSucesso: 'Entrou no site',
  LoginFalha: 'Senha errada ao entrar',
  LoginBloqueado: 'Login bloqueado (tentativas demais)',
  Logout: 'Saiu do site',
  SenhaAlterada: 'Senha alterada',
  Configuracao: 'Primeiro acesso: usuário criado',
  ParametrosAlterados: 'Parâmetros alterados',
  CredencialPluggyAlterada: 'Credencial da Pluggy alterada',
  CredencialPluggyRemovida: 'Credencial da Pluggy removida',
  ConexaoAdicionada: 'Conexão do Open Finance adicionada',
  ConexaoRemovida: 'Conexão do Open Finance removida',
  CategoriaEmLote: 'Categoria trocada em várias transações',
  CategoriaEmLoteDesfeita: 'Troca de categoria em várias transações desfeita',
  TransacaoRenomeada: 'Transação renomeada',
  RegraDescricaoCriada: 'Regra de descrição criada',
  RenomearEmLote: 'Várias transações renomeadas',
  RenomearDesfeito: 'Renomear desfeito',
  ConferenciaInvestimentos: 'Conferência de investimentos aplicada',
  ConferenciaInvestimentosDesfeita: 'Conferência de investimentos desfeita',
  TrocosAutomaticos: 'Trocos automáticos aplicados',
  TrocosAutomaticosDesfeitos: 'Trocos automáticos desfeitos'
};

/** "RegraDescricaoCriada (22)" → "Regra de descrição criada (22)"; evento desconhecido volta igual. */
export function descreverEvento(evento: string): string {
  const m = /^(\w+)(\s*\(.*\))?$/.exec(evento.trim());
  const nome = m ? NOMES_EVENTOS[m[1]] : undefined;
  return nome ? nome + (m![2] ? ' ' + m![2].trim() : '') : evento;
}

/** Endereços do próprio computador: o site só aceita conexões dele (bind em 127.0.0.1). */
const ENDERECOS_LOCAIS = new Set(['127.0.0.1', '::1', '::ffff:127.0.0.1', 'localhost']);

/** "127.0.0.1" → "Acesso local"; outro endereço aparece como veio (para não esconder nada estranho). */
export function origemDoAcesso(ip: string | null | undefined): string {
  const valor = (ip ?? '').trim();
  if (!valor) return '';
  return ENDERECOS_LOCAIS.has(valor.toLowerCase()) ? 'Acesso local' : valor;
}
