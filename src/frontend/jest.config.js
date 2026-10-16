// Testes do front-end com Jest + jest-preset-angular (ADR 0013).
//   npm test                   todos
//   npm run test:regressao     comportamentos que já quebraram ou que um PR garantiu (não podem voltar)
//   npm run test:usabilidade   critérios de UI/UX do CLAUDE.md (Nielsen, WCAG 2.2) verificáveis sem navegador
const { createCjsPreset } = require('jest-preset-angular/presets');

/** @type {import('jest').Config} */
module.exports = {
  ...createCjsPreset({ tsconfig: '<rootDir>/tsconfig.spec.json' }),
  roots: ['<rootDir>/testes'],
  testMatch: ['**/*.spec.ts'],
  setupFilesAfterEnv: ['<rootDir>/testes/configurar-jest.ts']
};
