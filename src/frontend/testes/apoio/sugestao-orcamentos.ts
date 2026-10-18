import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { ItemSugestaoOrcamento, NaturezaGasto, SugestaoOrcamento } from '../../src/app/core/modelos';
import { SugestaoOrcamentosComponent } from '../../src/app/paginas/sugestao-orcamentos.component';

export function item(campos: Partial<ItemSugestaoOrcamento>): ItemSugestaoOrcamento {
  return {
    categoriaId: 1, nome: 'Moradia', cor: '#a7a3f5', natureza: NaturezaGasto.Essencial, mediana: 1150, media: 1150,
    ultimoMes: 1150, tendencia: 'estavel', mesesComGasto: 6, pagoNoCartaoPercentual: 0, sugerido: 1210, atual: null,
    selecionado: true, motivos: ['mediana de 6 meses', '+5% de folga (essencial)'], ...campos
  };
}

/** Renda de R$ 8.000, sem parcelas, meta de 20%: R$ 6.400 disponíveis. */
export function sugestao(campos: Partial<SugestaoOrcamento> = {}): SugestaoOrcamento {
  return {
    ano: 2026, mes: 11, mesesBase: 6, mesesRenda: 12, rendaMedia: 8000, parcelasCartao: 0, parcelasFinanciamentos: 0,
    parcelasEmprestimos: 0, percentualPoupanca: 20, metaPoupanca: 1600, disponivel: 6400, totalSugerido: 0,
    faltaParaCaber: null, usoLimiteCartoes: null, nivelCartao: null, limiteAtencao: 30, limiteAlerta: 70,
    cartoes: [], avisos: [],
    itens: [
      item({}),
      item({ categoriaId: 2, nome: 'Mercado', sugerido: 1090, atual: 900, selecionado: false }),
      item({ categoriaId: 3, nome: 'Lazer', natureza: NaturezaGasto.EstiloDeVida, sugerido: 160, motivos: ['mediana de 6 meses'] }),
      item({ categoriaId: 4, nome: 'Viagens', natureza: NaturezaGasto.EstiloDeVida, sugerido: null, selecionado: false,
        mesesComGasto: 1, motivos: ['gasto em só 1 dos últimos 6 meses (jun/26): sem dados suficientes'] })
    ],
    ...campos
  };
}

/** Abre a janela de sugestão com a API falsa; `aplicados` guarda o que foi enviado para gravar. */
export function abrirSugestao(dados: SugestaoOrcamento = sugestao()) {
  const aplicados: { ano: number; mes: number; itens: { categoriaId: number; valor: number }[] }[] = [];
  TestBed.configureTestingModule({
    providers: [{
      provide: ApiService,
      useValue: {
        sugestaoOrcamentos: () => of(dados),
        aplicarSugestaoOrcamentos: (ano: number, mes: number, itens: { categoriaId: number; valor: number }[]) => {
          aplicados.push({ ano, mes, itens });
          return of({ transacoesAtualizadas: itens.length });
        }
      }
    }]
  });
  const fixture = TestBed.createComponent(SugestaoOrcamentosComponent);
  fixture.componentRef.setInput('ano', 2026);
  fixture.componentRef.setInput('mes', 11);
  const emitidos: number[] = [];
  fixture.componentInstance.aplicado.subscribe((n) => emitidos.push(n));
  fixture.detectChanges();
  const raiz = fixture.nativeElement as HTMLElement;
  const campo = (nome: string) => raiz.querySelector<HTMLInputElement>(`[aria-label="Valor sugerido para ${nome}"]`);
  const caixa = (nome: string) => raiz.querySelector<HTMLInputElement>(`[aria-label="Aplicar orçamento de ${nome}"]`)!;
  const total = () => raiz.querySelector('.rodape-sugestao p')!.textContent!.replace(/\s+/g, ' ').trim();
  const botaoAplicar = () => Array.from(raiz.querySelectorAll<HTMLButtonElement>('.rodape-sugestao button')).pop()!;
  const digitar = async (nome: string, valor: string) => {
    const c = campo(nome)!;
    c.value = valor;
    c.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const marcar = async (nome: string) => {
    caixa(nome).click();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  return { fixture, raiz, campo, caixa, total, botaoAplicar, digitar, marcar, aplicados, emitidos };
}
