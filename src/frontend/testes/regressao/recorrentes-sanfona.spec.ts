import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Conta, ContaAgendada, FormaCobranca, OrigemConta, Recorrencia, TipoConta } from '../../src/app/core/modelos';
import { AssinaturasRecorrentesComponent } from '../../src/app/paginas/assinaturas-recorrentes.component';
import { CHAVE_GRUPOS_FECHADOS } from '../../src/app/paginas/recorrentes';
import { contaAgendada } from '../apoio/contas-agendadas';

/**
 * #242: na aba "Assinaturas e recorrentes", cada conta ou cartão é uma sanfona (APG Disclosure): o título é um botão com
 * aria-expanded e aria-controls; vários abertos ao mesmo tempo, todos abertos no começo; os fechados ficam lembrados
 * neste navegador; "Fechar todos / Abrir todos". O selo do banco aparece no título e é decorativo.
 */
const conta = (id: number, nome: string, tipo: TipoConta, banco: string | null) =>
  ({ id, nome, tipo, banco, origem: OrigemConta.OpenFinance }) as unknown as Conta;

function abrir() {
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(AssinaturasRecorrentesComponent);
  const itens: ContaAgendada[] = [
    contaAgendada({ id: 1, descricao: 'Streaming', contaId: 2, contaNome: 'Cartão Nubank', formaCobranca: FormaCobranca.CartaoCredito }),
    contaAgendada({ id: 2, descricao: 'Luz', contaId: 1, contaNome: 'Conta Itaú', formaCobranca: FormaCobranca.DebitoAutomatico }),
    contaAgendada({ id: 3, descricao: 'Academia', contaId: null, contaNome: null, recorrencia: Recorrencia.Mensal })
  ];
  fixture.componentRef.setInput('itens', itens);
  fixture.componentRef.setInput('contas', [conta(1, 'Conta Itaú', TipoConta.ContaCorrente, 'itau'),
    conta(2, 'Cartão Nubank', TipoConta.CartaoCredito, 'nubank')]);
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const botoes = () => [...raiz.querySelectorAll<HTMLButtonElement>('.grupo-recorrentes-botao')];
  const botao = (nome: string) => botoes().find((b) => b.textContent!.includes(nome))!;
  const conteudo = (b: HTMLButtonElement) => raiz.querySelector<HTMLElement>(`#${b.getAttribute('aria-controls')}`)!;
  const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
  return { raiz, botoes, botao, conteudo, clicar };
}

describe('Regressão: Assinaturas e recorrentes em sanfona, com o selo do banco (#242)', () => {
  beforeEach(() => localStorage.clear());

  it('todos começam abertos; o título é um botão dentro do h2, com aria-expanded e aria-controls', () => {
    const { botoes, conteudo } = abrir();
    expect(botoes()).toHaveLength(3);
    for (const b of botoes()) {
      expect(b.closest('h2')).not.toBeNull();
      expect(b.getAttribute('aria-expanded')).toBe('true');
      expect(conteudo(b).classList).not.toContain('fechado');
      expect(conteudo(b).hasAttribute('inert')).toBe(false);
    }
  });

  it('fechar um não fecha os outros; fechado, sai do Tab (inert) e o título ainda diz quantas e quanto por mês', () => {
    const { botao, conteudo, clicar } = abrir();
    clicar(botao('Cartão Nubank'));
    expect(botao('Cartão Nubank').getAttribute('aria-expanded')).toBe('false');
    expect(conteudo(botao('Cartão Nubank')).classList).toContain('fechado');
    expect(conteudo(botao('Cartão Nubank')).hasAttribute('inert')).toBe(true);
    expect(botao('Cartão Nubank').textContent).toMatch(/1 assinatura.*\/mês/);
    expect(botao('Conta Itaú').getAttribute('aria-expanded')).toBe('true');
  });

  it('o fechado fica lembrado ao abrir a tela de novo', () => {
    const primeira = abrir();
    primeira.clicar(primeira.botao('Conta Itaú'));
    expect(JSON.parse(localStorage.getItem(CHAVE_GRUPOS_FECHADOS)!)).toEqual(['1']);
    TestBed.resetTestingModule();
    const segunda = abrir();
    expect(segunda.botao('Conta Itaú').getAttribute('aria-expanded')).toBe('false');
    expect(segunda.botao('Cartão Nubank').getAttribute('aria-expanded')).toBe('true');
  });

  it('"Fechar todos" fecha todos e vira "Abrir todos"', () => {
    const { raiz, botoes, clicar } = abrir();
    const acao = () => raiz.querySelector<HTMLButtonElement>('.acoes-grupos button')!;
    expect(acao().textContent!.trim()).toBe('Fechar todos');
    clicar(acao());
    expect(botoes().every((b) => b.getAttribute('aria-expanded') === 'false')).toBe(true);
    expect(acao().textContent!.trim()).toBe('Abrir todos');
    clicar(acao());
    expect(botoes().every((b) => b.getAttribute('aria-expanded') === 'true')).toBe(true);
  });

  it('o selo do banco fica no título, escondido do leitor de tela (o nome está escrito); sem conta, o selo neutro', () => {
    const { botao } = abrir();
    const selo = botao('Cartão Nubank').querySelector('app-logo-banco .logo-banco')!;
    expect(selo.tagName.toLowerCase()).toBe('svg');
    expect(selo.getAttribute('aria-hidden')).toBe('true');
    const neutro = botao('Sem conta').querySelector('app-logo-banco .logo-generico')!;
    expect(neutro.getAttribute('aria-hidden')).toBe('true');
  });
});
