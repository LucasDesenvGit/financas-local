import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { PALETAS, PreferenciaTema, TemaService } from '../core/tema.service';
import { IconeComponent } from '../compartilhado/icone.component';
import { Icones } from '../compartilhado/icones';

/** Aba "Aparência" de Configurações: paleta de cores, cor personalizada e modo claro/escuro. */
@Component({
  selector: 'app-aparencia',
  imports: [FormsModule, IconeComponent],
  template: `
    <section class="painel-bloco">
      <h2>Cores</h2>
      <p class="texto-suave">A paleta muda a cor principal, os destaques e o tom dos fundos. Verde (positivo), vermelho
        (negativo), laranja (emergência) e as cores de financiamento e empréstimo continuam as mesmas em todas.</p>

      <div class="paletas" role="radiogroup" aria-label="Paleta de cores">
        @for (p of paletas; track p.id) {
          <button type="button" class="paleta" role="radio" [attr.aria-checked]="tema.paleta() === p.id"
            [class.ativa]="tema.paleta() === p.id" (click)="tema.definirPaleta(p.id)">
            <span class="amostras">
              @for (c of p.amostras; track $index) { <i [style.background]="c"></i> }
            </span>
            <span class="paleta-nome">{{ p.nome }}</span>
            @if (tema.paleta() === p.id) { <app-icone class="paleta-check" [icone]="Icones.marcado" [tamanho]="16" /> }
          </button>
        }
        <label class="paleta" [class.ativa]="tema.paleta() === 'personalizada'">
          <span class="amostras"><input type="color" [ngModel]="tema.corPersonalizada()"
            (ngModelChange)="tema.definirCorPersonalizada($event)" aria-label="Escolher cor personalizada"></span>
          <span class="paleta-nome">Sua cor</span>
          @if (tema.paleta() === 'personalizada') { <app-icone class="paleta-check" [icone]="Icones.marcado" [tamanho]="16" /> }
        </label>
      </div>
      <p class="texto-suave pequeno">Na cor personalizada, os tons são ajustados automaticamente para o texto continuar legível
        (contraste mínimo de 4,5:1) no claro e no escuro.</p>
    </section>

    <section class="painel-bloco">
      <h2>Modo</h2>
      <div class="segmentado" role="radiogroup" aria-label="Modo claro ou escuro">
        @for (m of modos; track m.valor) {
          <button type="button" role="radio" [attr.aria-checked]="tema.preferencia() === m.valor"
            [class.ativo]="tema.preferencia() === m.valor" (click)="tema.definir(m.valor)">{{ m.rotulo }}</button>
        }
      </div>
    </section>

    <section class="painel-bloco">
      <h2>Prévia</h2>
      <div class="previa">
        <article class="kpi kpi-destaque">
          <span>Patrimônio líquido</span>
          <strong>R$ 48.320,15</strong>
          <small>contas + investimentos − cartões</small>
        </article>
        <div class="previa-itens">
          <div class="linha-flex">
            <button type="button" class="botao botao-primario">Botão principal</button>
            <button type="button" class="botao">Secundário</button>
          </div>
          <div class="linha-flex">
            <span class="etiqueta">Etiqueta</span>
            <span class="etiqueta etiqueta-sucesso">Pago</span>
            <span class="etiqueta etiqueta-alerta">Vencendo</span>
            <span class="positivo">+ R$ 1.200,00</span>
            <span class="negativo">− R$ 350,00</span>
          </div>
          <div class="trilho"><div class="progresso" style="width: 64%"></div></div>
        </div>
      </div>
    </section>
    <p class="texto-suave pequeno">As preferências de aparência ficam salvas neste navegador.</p>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [`
    .paletas { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; margin: 14px 0 10px; }
    .paleta {
      position: relative; display: grid; gap: 10px; justify-items: start; padding: 14px; cursor: pointer;
      border: 2px solid var(--linha); border-radius: var(--raio-medio); background: var(--superficie); font: inherit; color: var(--texto);
      text-align: left; transition: border-color var(--transicao), transform var(--transicao);
    }
    .paleta:hover { border-color: var(--primaria-suave); transform: translateY(-1px); }
    .paleta.ativa { border-color: var(--primaria); }
    .amostras { display: flex; gap: 6px; height: 28px; align-items: center; }
    .amostras i { width: 28px; height: 28px; border-radius: 8px; display: block; }
    .amostras input[type=color] { width: 64px; height: 30px; padding: 0; border: 0; background: none; cursor: pointer; }
    .paleta-nome { font-weight: 600; font-size: 0.92rem; }
    .paleta-check { position: absolute; top: 10px; right: 10px; color: var(--primaria); }
    .pequeno { font-size: 0.84rem; }
    .previa { display: grid; grid-template-columns: minmax(220px, 300px) 1fr; gap: 18px; align-items: center; }
    .previa-itens { display: grid; gap: 14px; }
    .previa-itens .linha-flex { flex-wrap: wrap; gap: 10px; align-items: center; }
    @media (max-width: 720px) { .previa { grid-template-columns: 1fr; } }
  `]
})
export class AparenciaComponent {
  readonly tema = inject(TemaService);
  readonly Icones = Icones;
  readonly paletas = PALETAS;
  readonly modos: { valor: PreferenciaTema; rotulo: string }[] = [
    { valor: 'claro', rotulo: 'Claro' },
    { valor: 'escuro', rotulo: 'Escuro' },
    { valor: 'sistema', rotulo: 'Automático' }
  ];
}
