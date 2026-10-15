import { Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { AvisosComponent } from './compartilhado/avisos.component';
import { TemaService } from './core/tema.service';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, AvisosComponent],
  template: `
    <router-outlet />
    <app-avisos />
  `
})
export class AppComponent {
  // Instancia o serviço de tema logo na inicialização (aplica claro/escuro antes da primeira tela).
  private readonly tema = inject(TemaService);
}
