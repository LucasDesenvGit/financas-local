import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { CampoDataComponent, inicioDaPaginaDeAnos } from '../../src/app/compartilhado/campo-data.component';
import { MesAno, SeletorMesComponent } from '../../src/app/compartilhado/seletor-mes.component';

/**
 * #342 — regra do projeto (docs/UI-UX.md, padrões adotados): todo quadro de escolher data ou mês tem o título como botão que
 * sobe de nível (dia → mês → ano), para chegar a outro ano sem dezenas de cliques (Nielsen 7) e com o mesmo jeito em
 * todas as telas (Nielsen 4). O calendário dos campos de data já fazia (#264); o seletor de mês (Painel, Contas,
 * Orçamentos…) ficou de fora. Componente novo de data entra neste teste.
 */
describe('Usabilidade: escolher data ou mês sobe de nível pelo título (#342)', () => {
  @Component({
    imports: [SeletorMesComponent],
    template: `<app-seletor-mes [valor]="valor" (mudou)="valor = $event; mudancas.push($event)" />`
  })
  class SeletorHospedeiro { valor: MesAno = { ano: 2023, mes: 5 }; mudancas: MesAno[] = []; }

  @Component({
    imports: [FormsModule, CampoDataComponent],
    template: `<label>Data <app-campo-data [(ngModel)]="data" /></label>`
  })
  class CampoHospedeiro { data = '2023-05-18'; }

  function seletor() {
    const fixture = TestBed.createComponent(SeletorHospedeiro);
    fixture.detectChanges();
    const raiz = fixture.nativeElement as HTMLElement;
    const clicar = (el: HTMLElement) => { el.click(); fixture.detectChanges(); };
    clicar(raiz.querySelector<HTMLButtonElement>('.seletor-mes-rotulo')!);
    const titulo = () => raiz.querySelector('.seletor-mes-ano strong')!.textContent!.trim();
    const botaoTitulo = () => raiz.querySelector<HTMLButtonElement>('.seletor-mes-ano .calendario-titulo-botao');
    const opcoes = () => [...raiz.querySelectorAll<HTMLButtonElement>('.seletor-mes-opcao')];
    return { fixture, raiz, clicar, titulo, botaoTitulo, opcoes, h: fixture.componentInstance };
  }

  it('o seletor de mês: o ano é um botão que abre 12 anos; escolher um ano volta aos meses dele', () => {
    const t = seletor();
    expect(t.titulo()).toBe('2023');
    expect(t.botaoTitulo()!.tagName).toBe('BUTTON');
    // A mesma dica do calendário para o leitor de tela.
    expect(t.raiz.querySelector('#' + t.botaoTitulo()!.getAttribute('aria-describedby'))!.textContent).toBe('escolher outro ano');
    t.clicar(t.botaoTitulo()!);
    const inicio = inicioDaPaginaDeAnos(2023);
    expect(t.titulo()).toBe(`${inicio}–${inicio + 11}`);
    expect(t.opcoes().map((b) => Number(b.textContent))).toEqual(Array.from({ length: 12 }, (_, i) => inicio + i));
    expect(t.opcoes().find((b) => b.getAttribute('aria-pressed') === 'true')!.textContent!.trim()).toBe('2023');
    // ‹ › andam uma página de 12 anos.
    t.clicar(t.raiz.querySelector<HTMLButtonElement>('[aria-label="Próximos anos"]')!);
    expect(t.titulo()).toBe(`${inicio + 12}–${inicio + 23}`);
    t.clicar(t.opcoes().find((b) => b.textContent!.trim() === String(inicio + 12))!);
    expect(t.titulo()).toBe(String(inicio + 12));
    expect(t.opcoes()).toHaveLength(12);
    // Escolher o mês no ano novo muda o mês da tela.
    t.clicar(t.opcoes()[2]);
    expect(t.h.mudancas).toEqual([{ ano: inicio + 12, mes: 3 }]);
  });

  it('o seletor de mês: Esc nos anos volta aos meses; nos meses, fecha', () => {
    const t = seletor();
    t.clicar(t.botaoTitulo()!);
    const painel = t.raiz.querySelector('.seletor-mes-painel')!;
    painel.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    t.fixture.detectChanges();
    expect(t.titulo()).toBe('2023');
    painel.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    t.fixture.detectChanges();
    expect(t.raiz.querySelector('.seletor-mes-painel')).toBeNull();
  });

  it('a regra vale para os dois quadros: o título sobe para os anos no calendário e no seletor de mês', async () => {
    const s = seletor();
    expect(s.botaoTitulo()).not.toBeNull();
    s.clicar(s.botaoTitulo()!);
    expect(s.titulo()).toMatch(/^\d{4}–\d{4}$/);

    TestBed.resetTestingModule();
    const fixture = TestBed.createComponent(CampoHospedeiro);
    fixture.detectChanges();
    await fixture.whenStable();
    const raiz = fixture.nativeElement as HTMLElement;
    const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
    raiz.querySelector<HTMLButtonElement>('.campo-data-botao')!.click();
    await atualizar();
    const titulo = () => raiz.querySelector('.calendario-topo strong')!.textContent!.trim();
    const botao = () => raiz.querySelector<HTMLButtonElement>('.calendario-titulo-botao')!;
    botao().click();
    await atualizar();
    expect(titulo()).toBe('2023');
    botao().click();
    await atualizar();
    expect(titulo()).toMatch(/^\d{4}–\d{4}$/);
  });
});
