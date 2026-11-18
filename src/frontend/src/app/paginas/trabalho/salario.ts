import { CalculoIrrf, Emprego, RegimeEmprego, SalarioVigencia, SalvarEmprego } from '../../core/modelos';

/**
 * #210 (ADR 0065): as regras da tela Trabalho › Salário, sem Angular (ADR 0059). O cálculo do INSS e do IRRF é da API;
 * aqui só o que a tela mostra e o formulário.
 */

/** O emprego atual: o primeiro sem saída (a API manda os atuais primeiro); sem nenhum, o mais recente. */
export function empregoAtual(lista: Emprego[]): Emprego | null {
  return lista.find((e) => !e.saida) ?? lista[0] ?? null;
}

export const REGIMES: { valor: RegimeEmprego; rotulo: string }[] = [
  { valor: RegimeEmprego.Clt, rotulo: 'CLT' },
  { valor: RegimeEmprego.Estagio, rotulo: 'Estágio' },
  { valor: RegimeEmprego.Pj, rotulo: 'PJ' }
];

export function rotuloRegime(r: RegimeEmprego): string {
  return REGIMES.find((x) => x.valor === r)?.rotulo ?? '—';
}

/** Uma linha do histórico: o salário e quanto mudou em relação ao anterior (null no primeiro). */
export interface LinhaHistorico { salario: SalarioVigencia; variacao: number | null; }

/** O histórico do mais novo para o mais antigo, com a variação de cada aumento em relação ao salário de antes. */
export function historicoDeSalarios(salarios: SalarioVigencia[]): LinhaHistorico[] {
  return salarios.map((s, i) => {
    const anterior = salarios[i + 1];
    return { salario: s, variacao: anterior && anterior.salarioBruto > 0 ? s.salarioBruto / anterior.salarioBruto - 1 : null };
  });
}

/** "2026-11-01" → "novembro de 2026". */
export function nomeDoMes(iso: string): string {
  const [ano, mes] = iso.split('-').map(Number);
  return `${MESES[mes - 1]} de ${ano}`;
}
const MESES = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro',
  'novembro', 'dezembro'];

/** { ano: 2026, mes: 3 } → "2026-03", o ?mes da estimativa. */
export function mesDaApi(p: { ano: number; mes: number }): string {
  return `${p.ano}-${String(p.mes).padStart(2, '0')}`;
}

/** 0.075 → "7,5%"; 0.133145 → "13,3145%" (as casas que a alíquota tiver). */
export function aliquota(a: number): string {
  return `${(a * 100).toLocaleString('pt-BR', { maximumFractionDigits: 4 })}%`;
}

/** Variação em texto: 0.2 → "+20,0%"; -0.05 → "−5,0%". */
export function variacao(v: number): string {
  const texto = Math.abs(v * 100).toLocaleString('pt-BR', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  return `${v < 0 ? '−' : '+'}${texto}%`;
}

/** A frase da dedução do IRRF: qual das duas foi usada e por quê (Nielsen 10, ajuda no contexto). */
export function explicarDeducao(irrf: CalculoIrrf): string {
  return irrf.deducaoUsada === 'simplificada'
    ? 'Usei o desconto simplificado, maior que o INSS e os dependentes: dá menos imposto.'
    : 'Usei o INSS e os dependentes, maiores que o desconto simplificado: dá menos imposto.';
}

/** "12345678000190" → "12.345.678/0001-90"; outro tamanho fica como está. */
export function formatarCnpj(cnpj: string | null): string {
  if (!cnpj) return '';
  return cnpj.length === 14 ? cnpj.replace(/^(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})$/, '$1.$2.$3/$4-$5') : cnpj;
}

/** O formulário do emprego (o salário só ao criar). */
export interface FormularioEmprego {
  empregador: string;
  cnpj: string;
  admissao: string | null;
  saida: string | null;
  regime: RegimeEmprego;
  jornadaMensalHoras: number | null;
  dependentes: number | null;
  contaId: number | null;
  salarioBruto: number | null;
}

export function formularioVazio(): FormularioEmprego {
  return { empregador: '', cnpj: '', admissao: null, saida: null, regime: RegimeEmprego.Clt, jornadaMensalHoras: 220,
    dependentes: 0, contaId: null, salarioBruto: null };
}

export function formularioDe(e: Emprego): FormularioEmprego {
  return { empregador: e.empregador, cnpj: formatarCnpj(e.cnpj), admissao: e.admissao, saida: e.saida, regime: e.regime,
    jornadaMensalHoras: e.jornadaMensalHoras, dependentes: e.dependentes, contaId: e.contaId, salarioBruto: null };
}

/** O corpo do POST ou do PUT. A validação de verdade é da API, que responde a mensagem. */
export function paraSalvar(f: FormularioEmprego, criando: boolean): SalvarEmprego {
  const corpo: SalvarEmprego = {
    empregador: f.empregador.trim(), cnpj: f.cnpj.trim() || null, admissao: f.admissao ?? '', saida: f.saida || null,
    regime: Number(f.regime), jornadaMensalHoras: f.jornadaMensalHoras ?? 220, dependentes: f.dependentes ?? 0,
    contaId: f.contaId ? Number(f.contaId) : null
  };
  if (criando && f.salarioBruto !== null) corpo.salarioBruto = f.salarioBruto;
  return corpo;
}
