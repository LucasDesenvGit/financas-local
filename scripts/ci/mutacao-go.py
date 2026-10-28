#!/usr/bin/env python3
"""
Teste de mutação do back-end em Go (#373, ADR 0051). Roda o gremlins nos pacotes de mutacao-go.txt, um por vez, e:
- lista as mutações que sobreviveram (LIVED), com a linha do código, tirando as equivalentes conhecidas
  (mutacao-go-equivalentes.txt: as que não mudam o resultado, e por isso nenhum teste pega);
- falha quando a eficácia de um pacote (mortas / (mortas + vivas), sem as equivalentes) fica abaixo do piso dele.

O gremlins trabalha numa cópia do módulo (não mexe nos arquivos) e só testa a mutação que algum teste do pacote
cobre. Ponto cego, de propósito no relatório: a condição de um "case" não é comando para a cobertura do Go, então
mutação dentro de "case" sai como NOT COVERED e nunca é testada. Nessas regras, valem as mutações à mão no PR.

Não roda em todo PR (é lento: ~1 min por pacote). Roda toda semana e à mão, pelo workflow "Teste de mutação (Go)".

O gerenciador em Go (src/gerenciador-go, ADR 0052) é outro módulo, com a lista e as equivalentes dele
(mutacao-gerenciador-go.txt e mutacao-gerenciador-go-equivalentes.txt). Ele roda no PR que o muda, na CI, para o teste
pesado não rodar na máquina de quem desenvolve (#433).

Uso:   python3 scripts/ci/mutacao-go.py                      # todos os pacotes de mutacao-go.txt
       python3 scripts/ci/mutacao-go.py internal/parametros  # só os pacotes dados
       python3 scripts/ci/mutacao-go.py --gerenciador        # os do gerenciador (mutacao-gerenciador-go.txt)
Saída: <módulo>/mutacao/<pacote>.json (o do gremlins) e <módulo>/mutacao/relatorio.md (também no resumo do job,
       quando roda no GitHub Actions).
"""
import json
import os
import subprocess
import sys

GREMLINS = 'github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0'
# Todos os operadores do gremlins, inclusive os desligados por padrão: os pacotes medidos são poucos e pequenos.
OPERADORES = ['--invert-logical', '--invert-loopctrl', '--invert-assignments', '--invert-bitwise', '--invert-bwassign',
              '--remove-self-assignments']
# Folga no tempo de cada teste: o padrão (5 vezes o tempo do teste) estoura com a compilação dos workers em paralelo.
EXECUCAO = ['--workers', '4', '--timeout-coefficient', '30']

AQUI = os.path.dirname(os.path.abspath(__file__))
RAIZ = os.path.dirname(os.path.dirname(AQUI))
# O módulo medido: o back-end (padrão) ou, com --gerenciador, o gerenciador; cada um com a sua lista.
GERENCIADOR = '--gerenciador' in sys.argv[1:]
GO = os.path.join(RAIZ, 'src', 'gerenciador-go' if GERENCIADOR else 'backend-go')
SAIDA = os.path.join(GO, 'mutacao')
LISTA = 'mutacao-gerenciador-go.txt' if GERENCIADOR else 'mutacao-go.txt'
EQUIVALENTES = 'mutacao-gerenciador-go-equivalentes.txt' if GERENCIADOR else 'mutacao-go-equivalentes.txt'
TITULO = '## Teste de mutação (gerenciador em Go)' if GERENCIADOR else '## Teste de mutação (Go)'


def ler_linhas(nome):
    with open(os.path.join(AQUI, nome), encoding='utf-8') as f:
        for linha in f:
            linha = linha.rstrip('\n')
            if linha.strip() and not linha.lstrip().startswith('#'):
                yield linha


def pisos():
    """pacote → piso da eficácia, em %. O piso só sobe (ADR 0051)."""
    resultado = {}
    for linha in ler_linhas(LISTA):
        pacote, piso = linha.split()[:2]
        resultado[pacote] = float(piso)
    return resultado


def equivalentes():
    """(pacote, arquivo, tipo, trecho da linha) das mutações que não mudam o resultado. Pelo texto da linha, e não
    pelo número, para não quebrar quando o arquivo muda em cima."""
    resultado = {}
    for linha in ler_linhas(EQUIVALENTES):
        # Separado por " | " (com os espaços): o "||" de uma condição no trecho não corta a linha.
        partes = [p.strip() for p in linha.split(' | ')]
        pacote, arquivo, tipo, motivo = partes[0], partes[1], partes[2], partes[-1]
        trecho = ' | '.join(partes[3:-1])
        resultado[(pacote, arquivo, tipo, trecho)] = motivo
    return resultado


def linha_do_codigo(pacote, arquivo, numero):
    with open(os.path.join(GO, pacote, arquivo), encoding='utf-8') as f:
        return f.read().split('\n')[numero - 1].strip()


def medir(pacote, conhecidas):
    os.makedirs(SAIDA, exist_ok=True)
    saida_json = os.path.join(SAIDA, pacote.replace('/', '-') + '.json')
    # O gerenciador mostra o andamento do gremlins (sem --silent): ajuda a ver onde ele para na CI (#433).
    silencio = [] if GERENCIADOR else ['--silent']
    comando = ['go', 'run', GREMLINS, 'unleash', './' + pacote] + silencio + ['--output', saida_json] + OPERADORES + EXECUCAO
    print(f'== {pacote}', flush=True)
    subprocess.run(comando, cwd=GO, check=True)
    with open(saida_json, encoding='utf-8') as f:
        dados = json.load(f)
    mortas, vivas, equivalentes_vistas, sem_teste = 0, [], [], 0
    for arquivo in dados['files']:
        for m in arquivo['mutations']:
            if m['status'] == 'KILLED':
                mortas += 1
            elif m['status'] == 'NOT COVERED':
                sem_teste += 1
            elif m['status'] == 'LIVED':
                trecho = linha_do_codigo(pacote, arquivo['file_name'], m['line'])
                chave = (pacote, arquivo['file_name'], m['type'], trecho)
                item = (arquivo['file_name'], m['line'], m['column'], m['type'], trecho)
                (equivalentes_vistas if chave in conhecidas else vivas).append(item + (conhecidas.get(chave, ''),))
    total = mortas + len(vivas)
    eficacia = 100.0 * mortas / total if total else 100.0
    return {'pacote': pacote, 'mortas': mortas, 'vivas': vivas, 'equivalentes': equivalentes_vistas,
            'sem_teste': sem_teste, 'eficacia': eficacia, 'segundos': dados.get('elapsed_time', 0)}


def relatorio(resultados, limites):
    linhas = [TITULO, '',
              '| Pacote | Mortas | Vivas | Equivalentes | Sem teste | Eficácia | Piso |', '|---|---|---|---|---|---|---|']
    for r in resultados:
        situacao = '' if r['eficacia'] >= limites[r['pacote']] else ' ❌'
        linhas.append(f"| `{r['pacote']}` | {r['mortas']} | {len(r['vivas'])} | {len(r['equivalentes'])} | {r['sem_teste']} | "
                      f"{r['eficacia']:.1f}%{situacao} | {limites[r['pacote']]:.0f}% |")
    linhas += ['', 'Eficácia = mortas / (mortas + vivas), sem as equivalentes conhecidas. "Sem teste" inclui as mutações '
               'dentro de `case`, que o gremlins não testa (ponto cego da cobertura do Go).']
    for r in resultados:
        if r['vivas']:
            linhas += ['', f"### Vivas em `{r['pacote']}`", '']
            linhas += [f"- `{a}:{l}:{c}` {t}: `{trecho}`" for a, l, c, t, trecho, _ in r['vivas']]
    return '\n'.join(linhas) + '\n'


def main():
    limites = pisos()
    pedidos = [a for a in sys.argv[1:] if a != '--gerenciador'] or list(limites)
    conhecidas = equivalentes()
    resultados = [medir(p, conhecidas) for p in pedidos]
    texto = relatorio(resultados, limites)
    with open(os.path.join(SAIDA, 'relatorio.md'), 'w', encoding='utf-8') as f:
        f.write(texto)
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write(texto)
    print(texto)
    abaixo = [r for r in resultados if r['eficacia'] < limites[r['pacote']]]
    for r in abaixo:
        print(f"::error::{r['pacote']}: eficácia {r['eficacia']:.1f}% abaixo do piso de {limites[r['pacote']]:.0f}%: "
              "um teste deixou de pegar mutação (veja as vivas no relatório)")
    sys.exit(1 if abaixo else 0)


if __name__ == '__main__':
    main()
