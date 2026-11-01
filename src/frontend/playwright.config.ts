import { defineConfig, devices } from '@playwright/test';

/**
 * Testes no navegador (#185, ADR 0041): o build de produção, servido com a CSP da API, num navegador de verdade e com
 * a API falsa (testes-navegador/api-falsa.ts). Complementa o Jest (ADR 0013), que não desenha a tela.
 *   npm run test:navegador          compila e roda tudo (computador e celular)
 *   npx playwright test --ui        para ver passo a passo
 * Neste computador usa o Edge instalado (sem baixar navegador); na CI, o Chromium do Playwright.
 * NAVEGADOR=chromium usa aqui o mesmo Chromium da CI (`npx playwright install chromium` uma vez): rode assim antes de
 * subir (push). O Edge deixou passar um teste que falhou na CI (#264).
 */
// #540: o teste e a API falsa rodam no Node, e o site no navegador (timezoneId, abaixo). Os dois no fuso de Brasília:
// com o Node em UTC (o runner da CI), das 21h à meia-noite do último dia do mês, o teste via um mês e o site, o anterior.
// Vale também para os workers, que herdam o ambiente.
process.env['TZ'] = 'America/Sao_Paulo';

const naCi = !!process.env['CI'];
const canal = naCi || process.env['NAVEGADOR'] === 'chromium' ? undefined : process.env['NAVEGADOR'] ?? 'msedge';
const porta = Number(process.env['PORTA'] ?? 4300);

export default defineConfig({
  testDir: 'testes-navegador',
  outputDir: 'testes-navegador/resultados',
  fullyParallel: true,
  forbidOnly: naCi,
  retries: naCi ? 1 : 0,
  reporter: naCi ? [['list'], ['html', { open: 'never', outputFolder: 'testes-navegador/relatorio' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${porta}`,
    locale: 'pt-BR',
    timezoneId: 'America/Sao_Paulo',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  projects: [
    { name: 'computador', use: { ...devices['Desktop Chrome'], channel: canal, viewport: { width: 1366, height: 860 } } },
    { name: 'celular', use: { ...devices['Pixel 7'], channel: canal } }
  ],
  webServer: {
    command: `node testes-navegador/servidor.mjs ${porta}`,
    url: `http://127.0.0.1:${porta}`,
    reuseExistingServer: !naCi,
    timeout: 30_000
  }
});
