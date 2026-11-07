import { ErroSistema, ErroSistemaDetalhe, NOMES_NIVEL_ERRO } from '../../core/modelos';

/** A hora como na tela (dd/MM/yyyy HH:mm), sempre no fuso de Brasília: o texto não muda com o fuso do navegador. */
const formatoHora = new Intl.DateTimeFormat('pt-BR', {
  timeZone: 'America/Sao_Paulo', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit'
});

function hora(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : formatoHora.format(d).replace(',', '');
}

/**
 * #332: o texto que o botão "Reportar para o Claude" copia, para colar no Claude Code. Começa pelo /reportar-erro, que
 * chama a skill do projeto (.claude/skills/reportar-erro). Vai só o que ajuda a achar a causa:
 *   - a mensagem, a origem, o nível e a tela (rota);
 *   - quantas vezes aconteceu, a primeira e a última vez;
 *   - quem registrou e a pilha, quando há;
 *   - a versão do sistema.
 * O login de quem estava na sessão fica de fora. A mensagem e a pilha já vêm do banco sem tokens, senhas, CPF e
 * valores em reais (o Sanitizador troca por [removido] antes de gravar, ADR 0022).
 */
export function textoParaOClaude(e: ErroSistema, d: ErroSistemaDetalhe | null, versao: string | null | undefined): string {
  const linhas = [
    '/reportar-erro',
    'Erro registrado em Configurações › Erros do Finanças Local (já sem dados sensíveis).',
    '',
    `- Mensagem: ${e.mensagem}`,
    `- Origem: ${e.origem}`,
    `- Nível: ${NOMES_NIVEL_ERRO[e.nivel] ?? e.nivel}`,
    `- Tela: ${e.rota ?? '(sem tela)'}`,
    e.ocorrencias > 1
      ? `- Ocorrências: ${e.ocorrencias}, da primeira em ${hora(e.primeiraEm)} à última em ${hora(e.ultimaEm)}`
      : `- Ocorrências: 1, em ${hora(e.ultimaEm)}`,
    `- Situação: ${e.resolvido ? 'marcado como resolvido' : 'em aberto'}`
  ];
  if (d?.categoria) linhas.push(`- Registrado por: ${d.categoria}`);
  linhas.push(`- Versão do sistema: ${versao || '(não informada)'}`);
  if (d?.detalhe) linhas.push('', 'Pilha:', '```', d.detalhe, '```');
  return linhas.join('\n');
}
