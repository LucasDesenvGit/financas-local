import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { AvisoContaAgendada, ContaAgendada, FormaCobranca, SalvarContaAgendada, TipoConta } from '../../src/app/core/modelos';
import { ContasAgendadasComponent } from '../../src/app/paginas/contas-agendadas.component';
import { contaAgendada } from '../apoio/contas-agendadas';
import { abrirContas, conta, CONTAS } from '../apoio/contas';

/**
 * #158 (parte 3): os avisos das recorrentes em Contas a pagar, com a ação ao lado ("Atualizar o valor", "Atualizar o
 * catálogo"), e o alerta ao arquivar um cartão que ainda tem cobranças (Nielsen 5).
 */
describe('Regressão: avisos das recorrentes (#158)', () => {
  const netflix = contaAgendada({ id: 7, descricao: 'Netflix', valor: 39.9, contaId: 4, contaNome: 'Nubank',
    formaCobranca: FormaCobranca.CartaoCredito, textoCobranca: 'NETFLIX' });
  const aviso = (campos: Partial<AvisoContaAgendada>): AvisoContaAgendada => ({
    contaAgendadaId: 7, descricao: 'Netflix', tipo: 'aumentou', texto: 'Netflix aumentou na cobrança de 12/10.',
    valorAnterior: 39.9, valorNovo: 44.9, contaNovaId: null, contaNovaNome: null, data: '2026-10-12', ...campos
  });

  function abrir(avisos: AvisoContaAgendada[], itens: ContaAgendada[] = [netflix]) {
    const salvos: [number, SalvarContaAgendada][] = [];
    TestBed.configureTestingModule({
      providers: [provideRouter([]), {
        provide: ApiService,
        useValue: {
          contasAgendadas: () => of(itens), avisosContasAgendadas: () => of(avisos), sugestoesContasAgendadas: () => of([]),
          contas: () => of([]), categorias: () => of([]),
          atualizarContaAgendada: (id: number, dto: SalvarContaAgendada) => { salvos.push([id, dto]); return of(undefined); }
        }
      }]
    });
    const fixture = TestBed.createComponent(ContasAgendadasComponent);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const botao = (rotulo: string) => raiz.querySelector<HTMLButtonElement>(`[aria-label="${rotulo}"]`);
    return { fixture, raiz, botao, salvos };
  }

  it('"aumentou" mostra o valor de antes e o novo, e "Atualizar o valor" grava o novo', () => {
    const { raiz, botao, salvos } = abrir([aviso({})]);
    expect(raiz.querySelector('.avisos-recorrentes')!.textContent!.replace(/\s+/g, ' ')).toMatch(/R\$\s39,90 → R\$\s44,90/);
    botao('Atualizar o valor de Netflix')!.click();
    expect(salvos).toEqual([[7, expect.objectContaining({ valor: 44.9, contaId: 4, textoCobranca: 'NETFLIX' })]]);
  });

  it('"mudou de cartão" troca a conta da conta a pagar para a da última cobrança', () => {
    const { botao, salvos } = abrir([aviso({ tipo: 'mudou-de-cartao', texto: 'A última cobrança de Netflix veio em Itaú.',
      contaNovaId: 6, contaNovaNome: 'Itaú', valorAnterior: null, valorNovo: null })]);
    botao('Atualizar o catálogo de Netflix: cobra em Itaú')!.click();
    expect(salvos).toEqual([[7, expect.objectContaining({ contaId: 6, valor: 39.9 })]]);
  });

  it('"não veio" e "cartão arquivado" aparecem sem ação (o que fazer é fora do site)', () => {
    const { raiz } = abrir([aviso({ tipo: 'nao-veio', texto: 'A cobrança de Netflix esperada em 12/10 não apareceu.' })]);
    expect(raiz.querySelector('.avisos-recorrentes')!.textContent).toContain('não apareceu');
    expect(raiz.querySelectorAll('.avisos-recorrentes button')).toHaveLength(0);
  });

  it('sem avisos, o bloco não aparece', () => {
    const { raiz } = abrir([]);
    expect(raiz.querySelector('.avisos-recorrentes')).toBeNull();
  });

  it('marcar "Arquivada" num cartão com cobranças avisa o que ainda cobra nele, antes de salvar', async () => {
    const gold = conta({ id: 4, nome: 'Gold', tipo: TipoConta.CartaoCredito, saldo: -100, limiteCredito: 1000, limiteDisponivel: 900 });
    const t = await abrirContas('/contas', [...CONTAS, gold], {}, { contasAgendadas: () => of([netflix]) });
    t.tela.editar(gold);
    await t.atualizar();
    const alerta = () => t.raiz.ownerDocument.querySelector('[role="status"].texto-alerta');
    expect(alerta()).toBeNull();
    t.tela.form!.arquivada = true;
    await t.atualizar();
    expect(alerta()!.textContent).toContain('Ainda cobram aqui: Netflix');
    expect(alerta()!.textContent).toContain('Troque o cartão nos serviços');
  });
});
