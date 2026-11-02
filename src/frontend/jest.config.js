// Testes do front-end com Jest + jest-preset-angular (ADR 0013).
//   npm test                   todos
//   npm run test:regressao     comportamentos que já quebraram ou que um PR garantiu (não podem voltar)
//   npm run test:usabilidade   critérios de UI/UX de docs/UI-UX.md (Nielsen, WCAG 2.2) verificáveis sem navegador
const { createCjsPreset } = require('jest-preset-angular/presets');

/** @type {import('jest').Config} */
module.exports = {
  ...createCjsPreset({ tsconfig: '<rootDir>/tsconfig.spec.json' }),
  roots: ['<rootDir>/testes'],
  testMatch: ['**/*.spec.ts'],
  setupFilesAfterEnv: ['<rootDir>/testes/configurar-jest.ts'],
  // #204: o @lucide/angular de verdade (10,6 MB, ~1.900 componentes compilados em JIT a cada arquivo) levava ~15 s por
  // arquivo de teste. Nos testes do Jest vale um substituto leve; o build e os testes no navegador usam o de verdade.
  moduleNameMapper: { '^@lucide/angular$': '<rootDir>/testes/apoio/lucide-falso.js' }
};
