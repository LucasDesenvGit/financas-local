import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { NavigationEnd, NavigationStart, Router, RouterOutlet } from '@angular/router';
import { AvisosComponent } from './compartilhado/ui/avisos.component';
import { TemaService } from './core/tema.service';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, AvisosComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <router-outlet />
    <app-avisos />
  `
})
export class AppComponent {
  // Instancia o serviço de tema logo na inicialização (aplica claro/escuro antes da primeira tela).
  private readonly tema = inject(TemaService);

  // #820: tela ou capítulo novo começa no topo. Não rola quando só muda a query (filtro, busca, aba), quando há
  // fragmento (o anchorScrolling leva à seção) nem no voltar/avançar do navegador (ele restaura a posição).
  private readonly router = inject(Router);
  private caminhoAnterior: string | null = null;
  private porHistorico = false;
  constructor() {
    this.router.events.subscribe(e => {
      if (e instanceof NavigationStart) this.porHistorico = e.navigationTrigger === 'popstate';
      if (!(e instanceof NavigationEnd)) return;
      const [semFragmento, fragmento] = e.urlAfterRedirects.split('#');
      const caminho = semFragmento.split('?')[0];
      if (this.caminhoAnterior !== null && caminho !== this.caminhoAnterior && !fragmento && !this.porHistorico) window.scrollTo(0, 0);
      this.caminhoAnterior = caminho;
    });
  }
}
