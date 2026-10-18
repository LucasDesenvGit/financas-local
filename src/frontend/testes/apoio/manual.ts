import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { ManualComponent } from '../../src/app/paginas/manual.component';

/** Abre o manual pelo roteador de verdade (capítulo, âncora e busca na URL). */
export async function abrirManual(url = '/manual') {
  TestBed.configureTestingModule({
    providers: [provideRouter([
      { path: 'manual', component: ManualComponent },
      { path: 'manual/:capitulo', component: ManualComponent }
    ])]
  });
  const harness = await RouterTestingHarness.create();
  const tela = await harness.navigateByUrl(url, ManualComponent);
  const atualizar = async () => { await harness.fixture.whenStable(); harness.detectChanges(); };
  await atualizar();
  return { harness, tela, raiz: harness.routeNativeElement as HTMLElement, atualizar, router: TestBed.inject(Router) };
}
