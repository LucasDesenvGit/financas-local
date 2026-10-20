import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';
import { ApiService } from '../../src/app/core/api.service';
import { RegraDescricao, SalvarRegraDescricao, TipoCorrespondencia } from '../../src/app/core/modelos';
import { CategoriasComponent } from '../../src/app/paginas/categorias.component';

/** As duas contas do condomínio: a mesma descrição do banco, uma regra por faixa de valor. */
const regras: RegraDescricao[] = [
  { id: 1, texto: 'CONDOMINIO', tipoCorrespondencia: TipoCorrespondencia.Contem, novaDescricao: 'Condomínio — taxa',
    ativa: true, transacoes: 6, valorMinimo: 800, valorMaximo: 900 },
  { id: 2, texto: 'CONDOMINIO', tipoCorrespondencia: TipoCorrespondencia.Contem, novaDescricao: 'Condomínio — fundo',
    ativa: true, transacoes: 6, valorMinimo: null, valorMaximo: 250 },
  { id: 3, texto: 'PIX ENVIADO', tipoCorrespondencia: TipoCorrespondencia.Contem, novaDescricao: 'Pix',
    ativa: true, transacoes: 2, valorMinimo: null, valorMaximo: null }
];

async function abrir() {
  const previas: unknown[][] = [];
  const salvas: SalvarRegraDescricao[] = [];
  const api = {
    categorias: () => of([]),
    regras: () => of([]),
    regrasDescricao: () => of(regras),
    previaRegraDescricao: (...args: unknown[]) => { previas.push(args); return of({ quantidade: 6, exemplos: [] }); },
    criarRegraDescricao: (dto: SalvarRegraDescricao) => { salvas.push(dto); return of({ ...regras[0], ...dto }); },
    salvarRegraDescricao: (_id: number, dto: SalvarRegraDescricao) => { salvas.push(dto); return of({ ...regras[0], ...dto }); }
  };
  TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: ApiService, useValue: api }] });
  const fixture = TestBed.createComponent(CategoriasComponent);
  const tela = fixture.componentInstance;
  tela.aba.set('regras');
  const assentar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
  await assentar();
  return { fixture, tela, raiz: fixture.nativeElement as HTMLElement, assentar, previas, salvas };
}

/**
 * #241: regra de descrição por valor, para separar cobranças com a mesma descrição do banco e valores diferentes (as
 * duas contas do condomínio) e, com isso, ter duas contas a pagar.
 */
describe('Regressão: regra de descrição por valor', () => {
  it('a lista mostra a faixa de cada regra, e "qualquer" sem faixa', async () => {
    const t = await abrir();
    const celulas = Array.from(t.raiz.querySelectorAll('td.faixa-valor')).map((c) => c.textContent!.replace(/\s+/g, ' ').trim());
    expect(celulas).toEqual(['de R$ 800,00 a R$ 900,00', 'até R$ 250,00', 'qualquer']);
  });

  it('a regra nova manda a faixa para a prévia e para a API', async () => {
    const t = await abrir();
    t.tela.novaRegraDescricao();
    const f = t.tela.formRegraDescricao!;
    expect(f.valorMinimo).toBeNull();
    expect(f.valorMaximo).toBeNull();
    Object.assign(f, { texto: 'CONDOMINIO', novaDescricao: 'Condomínio — fundo', valorMinimo: 150, valorMaximo: 250 });
    t.tela.pedirPreviaRegraDescricao();
    await new Promise((fim) => setTimeout(fim, 350));
    expect(t.previas.at(-1)).toEqual(['CONDOMINIO', TipoCorrespondencia.Contem, 150, 250]);
    t.tela.salvarRegraDescricao();
    expect(t.salvas).toEqual([{ texto: 'CONDOMINIO', tipoCorrespondencia: TipoCorrespondencia.Contem,
      novaDescricao: 'Condomínio — fundo', ativa: true, valorMinimo: 150, valorMaximo: 250 }]);
  });

  it('"De" maior que "Até" mostra o erro em texto, trava o Salvar e não chama a API', async () => {
    const t = await abrir();
    t.tela.novaRegraDescricao();
    Object.assign(t.tela.formRegraDescricao!, { texto: 'CONDOMINIO', novaDescricao: 'Condomínio', valorMinimo: 900, valorMaximo: 800 });
    await t.assentar();
    expect(t.raiz.querySelector('[role="alert"]')!.textContent).toContain('O valor "De" é maior que o "Até"');
    expect(t.tela.textoPreviaRegraDescricao()).toBe('Acerte o valor para ver quantas transações a regra renomeia.');
    const salvar = Array.from(t.raiz.querySelectorAll<HTMLButtonElement>('.rodape-form button')).find((b) => b.textContent!.trim() === 'Salvar')!;
    expect(salvar.disabled).toBe(true);
    t.tela.salvarRegraDescricao();
    expect(t.salvas).toHaveLength(0);
  });

  it('editar traz a faixa da regra, e o campo em branco volta como null (qualquer valor)', async () => {
    const t = await abrir();
    t.tela.editarRegraDescricao(regras[1]);
    expect(t.tela.formRegraDescricao).toMatchObject({ valorMinimo: null, valorMaximo: 250 });
    t.tela.formRegraDescricao!.valorMaximo = null;
    t.tela.salvarRegraDescricao();
    expect(t.salvas[0]).toMatchObject({ valorMinimo: null, valorMaximo: null });
  });

  it('os campos de valor têm rótulo visível dentro de um grupo com legenda (WCAG 3.3.2)', async () => {
    const t = await abrir();
    t.tela.novaRegraDescricao();
    await t.assentar();
    const grupo = t.raiz.querySelector('fieldset.faixa-regra')!;
    expect(grupo.querySelector('legend')!.textContent).toBe('E o valor (opcional)');
    const rotulos = Array.from(grupo.querySelectorAll('label')).map((l) => l.firstChild!.textContent!.trim());
    expect(rotulos).toEqual(['De', 'Até']);
    expect(grupo.querySelectorAll('app-campo-valor')).toHaveLength(2);
  });
});
