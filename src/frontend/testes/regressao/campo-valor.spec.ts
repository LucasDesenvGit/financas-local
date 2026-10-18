import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { readFileSync } from 'fs';
import { join } from 'path';
import { CampoValorComponent, formatarDigitando, formatarValor, interpretarValor } from '../../src/app/compartilhado/campo-valor.component';

/**
 * Regressão #132: os campos de dinheiro eram <input type="number">. "1.500" virava 1,5; colar "R$ 1.234,56" do app
 * do banco deixava o campo vazio; a rodinha do mouse mudava o valor. O app-campo-valor entende o jeito brasileiro e
 * entrega ao ngModel o mesmo number de antes (a API não muda).
 */
describe('Regressão: campo de valor em reais (#132)', () => {
  describe('interpretarValor', () => {
    it.each([
      ['1234,56', 1234.56],
      ['1.234,56', 1234.56],
      ['R$ 1.234,56', 1234.56],
      ['R$ 1.234,56', 1234.56],      // espaço não separável, como vem do app do banco
      ['1234.56', 1234.56],              // ponto como decimal: um ponto e duas casas
      ['1.500', 1500],                   // milhar, não 1,5
      ['1.234.567', 1234567],
      ['1.234.567,8', 1234567.8],
      ['1,234.56', 1234.56],             // formato americano: o último separador é o decimal
      ['0,5', 0.5],
      ['50', 50],
      ['-50,25', -50.25],
      ['R$ -50,25', -50.25],
      ['0.1234', 0.1234],                // cotação com 4 casas
      ['US$ 12,34', 12.34]
    ])('"%s" vira %p', (texto, esperado) => {
      expect(interpretarValor(texto)).toBeCloseTo(esperado as number, 6);
    });

    it('vazio (ou só espaços) é null', () => {
      expect(interpretarValor('')).toBeNull();
      expect(interpretarValor('   ')).toBeNull();
      expect(interpretarValor(null)).toBeNull();
    });

    it.each(['abc', '12a', '1,2,3,4.5,6', 'R$', '--5'])('"%s" é inválido (NaN)', (texto) => {
      expect(interpretarValor(texto)).toBeNaN();
    });
  });

  describe('formatarDigitando (formatação em tempo real)', () => {
    it.each([
      // [texto no campo, cursor, → texto, → cursor]
      ['124214', 6, '124.214', 7],
      ['1234,5', 6, '1.234,5', 7],
      ['1.2345', 6, '12.345', 6],          // digitou o 5 no fim de "1.234"
      ['1.2934', 4, '12.934', 4],          // digitou o 9 no meio de "1.234": o cursor fica depois dele
      ['0005', 4, '5', 1],
      [',5', 2, '0,5', 3],
      ['12,345', 6, '12,34', 5],           // no máximo 2 casas
      ['12a3', 4, '123', 3],               // letra ignorada
      ['', 0, '', 0]
    ])('"%s" (cursor %i) → "%s" (cursor %i)', (texto, cursor, esperado, cursorEsperado) => {
      expect(formatarDigitando(texto as string, cursor as number)).toEqual({ texto: esperado, cursor: cursorEsperado });
    });

    it('ponto digitado vira a vírgula decimal', () => {
      expect(formatarDigitando('1234.', 5, 2, false, true)).toEqual({ texto: '1.234,', cursor: 6 });
    });

    it('sinal de menos só quando o campo aceita negativo', () => {
      expect(formatarDigitando('-1234', 5, 2, true).texto).toBe('-1.234');
      expect(formatarDigitando('-1234', 5, 2, false).texto).toBe('1.234');
    });

    it('cotação com 4 casas', () => {
      expect(formatarDigitando('12,34567', 8, 4).texto).toBe('12,3456');
    });
  });

  it('formata no jeito brasileiro, com 2 casas no mínimo', () => {
    expect(formatarValor(1234.5)).toBe('1.234,50');
    expect(formatarValor(1500)).toBe('1.500,00');
    expect(formatarValor(12.3456, 4)).toBe('12,3456');
    expect(formatarValor(12.5, 4)).toBe('12,50');
  });

  describe('com ngModel', () => {
    @Component({
      imports: [FormsModule, CampoValorComponent],
      template: `<form #f="ngForm"><label>Valor <app-campo-valor name="valor" [(ngModel)]="valor" obrigatorio [minimo]="0.01" /></label>
        <label>Saldo <app-campo-valor name="saldo" [(ngModel)]="saldo" negativo /></label></form>`
    })
    class Hospedeiro { valor: number | null = null; saldo: number | null = -10; }

    async function montar() {
      const fixture = TestBed.createComponent(Hospedeiro);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      const [valor, saldo] = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('input'));
      const digitar = async (campo: HTMLInputElement, texto: string) => {
        campo.value = texto;
        campo.dispatchEvent(new Event('input'));
        campo.dispatchEvent(new Event('blur'));
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
      };
      return { fixture, valor, saldo, digitar, hospedeiro: fixture.componentInstance };
    }

    it('colar "R$ 1.234,56" entrega o number 1234.56 e, ao sair, mostra "1.234,56"', async () => {
      const t = await montar();
      await t.digitar(t.valor, 'R$ 1.234,56');
      expect(t.hospedeiro.valor).toBe(1234.56);
      expect(t.valor.value).toBe('1.234,56');
    });

    it('"1.500" entrega 1500 (não 1,5)', async () => {
      const t = await montar();
      await t.digitar(t.valor, '1.500');
      expect(t.hospedeiro.valor).toBe(1500);
    });

    it('apagar entrega null; obrigatório deixa o formulário inválido', async () => {
      const t = await montar();
      await t.digitar(t.valor, '10');
      await t.digitar(t.valor, '');
      expect(t.hospedeiro.valor).toBeNull();
      expect(t.fixture.nativeElement.querySelector('form').classList).toContain('ng-invalid');
    });

    it('abaixo do mínimo deixa o formulário inválido, com a mensagem certa', async () => {
      const t = await montar();
      await t.digitar(t.valor, '0');
      expect(t.fixture.nativeElement.querySelector('.campo-valor-mensagem').textContent).toBe('Informe um valor maior que zero.');
      expect(t.fixture.nativeElement.querySelector('form').classList).toContain('ng-invalid');
    });

    it('negativo sem permissão: digitando, o "-" é ignorado; colado, o erro avisa', async () => {
      const t = await montar();
      await tecla(t, t.valor, '-', 1, 'insertText', '-');
      expect(t.valor.value).toBe('');
      await tecla(t, t.valor, '-5', 2, 'insertFromPaste', null);
      t.valor.dispatchEvent(new Event('blur'));
      t.fixture.detectChanges();
      expect(t.fixture.nativeElement.querySelector('.campo-valor-mensagem').textContent).toBe('Informe um valor positivo.');
      expect(t.fixture.nativeElement.querySelector('form').classList).toContain('ng-invalid');
    });

    it('saldo inicial aceita negativo e mostra o valor que veio do modelo já formatado', async () => {
      const t = await montar();
      expect(t.saldo.value).toBe('-10,00');
      await t.digitar(t.saldo, '-1.234,5');
      expect(t.hospedeiro.saldo).toBe(-1234.5);
    });

    /** Uma tecla de verdade: o navegador já pôs o caractere no campo; o componente formata. */
    async function tecla(t: Awaited<ReturnType<typeof montar>>, campo: HTMLInputElement, novoTexto: string, cursor: number,
      inputType: string, data: string | null) {
      campo.value = novoTexto;
      campo.setSelectionRange(cursor, cursor);
      campo.dispatchEvent(new InputEvent('input', { inputType, data }));
      t.fixture.detectChanges();
      await t.fixture.whenStable();
    }

    it('digitando "124214" tecla a tecla, o campo já mostra "124.214" (sem esperar sair do campo)', async () => {
      const t = await montar();
      let texto = '';
      for (const d of '124214') {
        texto = t.valor.value + d;
        await tecla(t, t.valor, texto, texto.length, 'insertText', d);
      }
      expect(t.valor.value).toBe('124.214');
      expect(t.valor.selectionStart).toBe(7);
      expect(t.hospedeiro.valor).toBe(124214);
    });

    it('Backspace logo depois de um ponto de milhar apaga o dígito antes dele', async () => {
      const t = await montar();
      await tecla(t, t.valor, '1234', 4, 'insertFromPaste', null);          // colado: vira "1.234,00"
      t.valor.value = '1.234';
      (t.fixture.debugElement.children[0].query((d) => d.name === 'app-campo-valor').componentInstance).texto.set('1.234');
      await tecla(t, t.valor, '1234', 1, 'deleteContentBackward', null);   // apagou o "." (cursor estava em 2)
      expect(t.valor.value).toBe('234');
      expect(t.hospedeiro.valor).toBe(234);
    });

    it('texto inválido entrega null e mostra o erro', async () => {
      const t = await montar();
      await t.digitar(t.valor, '12a');
      expect(t.hospedeiro.valor).toBeNull();
      expect(t.fixture.nativeElement.querySelector('.campo-valor-mensagem').textContent).toContain('Valor inválido');
    });
  });

  it('nenhum campo de dinheiro das telas voltou a ser <input type="number">', () => {
    const telas = ['transacoes', 'contas', 'contas-agendadas', 'metas', 'orcamentos', 'investimentos', 'conferir-investimentos'];
    const dinheiro = /type="number"[^>]*\[\(ngModel\)\]="[^"]*\b(valor|valorAlvo|valorAtual|valorAporte|valorEditado|saldo|saldoInicial|limiteCredito|precoUnitario|precoAtual|preco|taxas)"/;
    for (const tela of telas) {
      const fonte = readFileSync(join(__dirname, `../../src/app/paginas/${tela}.component.ts`), 'utf8');
      expect({ tela, achou: dinheiro.test(fonte) }).toEqual({ tela, achou: false });
    }
  });
});
