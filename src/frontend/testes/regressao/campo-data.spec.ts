import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { readdirSync, readFileSync } from 'fs';
import { join } from 'path';
import {
  CampoDataComponent, DATA_MAXIMA, formatarDataDigitando, isoParaTexto, textoParaIso
} from '../../src/app/compartilhado/campo-data.component';

/**
 * Regressão do campo de data (app-campo-data): o <input type="date"> aceitava ano com 6 dígitos ("13/02/275760") e
 * o calendário era o do navegador. Agora se digita dd/mm/aaaa (ano sempre com 4 dígitos) ou se escolhe no calendário
 * do site; o ngModel recebe o mesmo 'AAAA-MM-DD' de antes.
 */
describe('Regressão: campo de data', () => {
  describe('funções', () => {
    it.each([
      ['1', 1, '1', 1], ['18', 2, '18', 2], ['181', 3, '18/1', 4], ['18102026', 8, '18/10/2026', 10],
      ['181020269999', 12, '18/10/2026', 10],     // ano nunca passa de 4 dígitos
      ['18/1a0', 6, '18/10', 5]                   // letra ignorada
    ])('máscara: "%s" → "%s"', (texto, cursor, esperado, cursorEsperado) => {
      expect(formatarDataDigitando(texto as string, cursor as number)).toEqual({ texto: esperado, cursor: cursorEsperado });
    });

    it('lê dd/mm/aaaa, aponta incompleta e inválida', () => {
      expect(textoParaIso('18/10/2026')).toEqual({ estado: 'ok', iso: '2026-10-18' });
      expect(textoParaIso('18/10/20')).toEqual({ estado: 'incompleta', iso: null });
      expect(textoParaIso('31/02/2026')).toEqual({ estado: 'invalida', iso: null });
      expect(textoParaIso('')).toEqual({ estado: 'vazia', iso: null });
      expect(isoParaTexto('2026-10-18')).toBe('18/10/2026');
    });
  });

  describe('com ngModel', () => {
    @Component({
      imports: [FormsModule, CampoDataComponent],
      template: `<form #f="ngForm">
        <label>Data <app-campo-data name="data" [(ngModel)]="data" obrigatorio (alterado)="alterados.push($event)" /></label>
        <label>Até hoje <app-campo-data name="ate" [(ngModel)]="ate" [maximo]="hoje" /></label>
      </form>`
    })
    class Hospedeiro { data = '2026-10-18'; ate = ''; hoje = '2026-10-18'; alterados: string[] = []; }

    async function montar() {
      const fixture = TestBed.createComponent(Hospedeiro);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      const raiz = fixture.nativeElement as HTMLElement;
      const [data, ate] = Array.from(raiz.querySelectorAll<HTMLInputElement>('app-campo-data input'));
      const atualizar = async () => { fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges(); };
      const digitar = async (campo: HTMLInputElement, texto: string) => {
        campo.value = texto;
        campo.dispatchEvent(new Event('input'));
        campo.dispatchEvent(new Event('blur'));
        await atualizar();
      };
      return { fixture, raiz, data, ate, digitar, atualizar, form: () => raiz.querySelector('form')!, h: fixture.componentInstance };
    }

    it('mostra a data do modelo como dd/mm/aaaa e entrega AAAA-MM-DD ao digitar', async () => {
      const t = await montar();
      expect(t.data.value).toBe('18/10/2026');
      await t.digitar(t.data, '01112026');
      expect(t.data.value).toBe('01/11/2026');
      expect(t.h.data).toBe('2026-11-01');
      expect(t.h.alterados).toEqual(['2026-11-01']);
    });

    it('colar AAAA-MM-DD (de uma planilha) também funciona', async () => {
      const t = await montar();
      await t.digitar(t.data, '2026-12-25');
      expect(t.data.value).toBe('25/12/2026');
      expect(t.h.data).toBe('2026-12-25');
    });

    it('data incompleta ou inválida deixa o formulário inválido, com a mensagem em texto', async () => {
      const t = await montar();
      await t.digitar(t.data, '31/02/2026');
      expect(t.form().classList).toContain('ng-invalid');
      expect(t.raiz.querySelector('.campo-valor-mensagem')!.textContent).toBe('Data inválida.');
      await t.digitar(t.data, '18/10');
      expect(t.raiz.querySelector('.campo-valor-mensagem')!.textContent).toBe('Data incompleta. Use dd/mm/aaaa.');
      expect(t.h.alterados).toEqual([]);          // nada é confirmado enquanto a data não está completa
    });

    it('respeita o limite da tela (ex.: [maximo]="hoje")', async () => {
      const t = await montar();
      await t.digitar(t.ate, '19/10/2026');
      expect(t.form().classList).toContain('ng-invalid');
      expect(t.raiz.querySelectorAll('.campo-valor-mensagem')[0].textContent).toBe('Use uma data até 18/10/2026.');
    });

    it('limite padrão de 2099: ano grande não passa', async () => {
      const t = await montar();
      expect(DATA_MAXIMA).toBe('2099-12-31');
      await t.digitar(t.data, '13/02/2757');
      expect(t.form().classList).toContain('ng-invalid');
    });
  });

  it('nenhuma tela usa mais <input type="date"> (todas usam app-campo-data)', () => {
    const pasta = join(__dirname, '../../src/app/paginas');
    const comDate = readdirSync(pasta).filter((f) => f.endsWith('.ts'))
      .filter((f) => readFileSync(join(pasta, f), 'utf8').includes('type="date"'));
    expect(comDate).toEqual([]);
  });
});
