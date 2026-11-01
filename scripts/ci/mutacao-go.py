#!/usr/bin/env python3
"""
Teste de mutação do back-end em Go (#373, ADR 0051). Roda o gremlins nos pacotes de mutacao-go.txt, um por vez, e:
- lista as mutações que sobreviveram (LIVED), com a linha do código, tirando as equivalentes conhecidas
  (mutacao-go-equivalentes.txt: as que não mudam o resultado, e por isso nenhum teste pega);
- falha quando a eficácia de um pacote (mortas / (mortas + vivas), sem as equivalentes) fica abaixo do piso dele.

O gremlins trabalha numa cópia do módulo (não mexe nos arquivos) e só testa a mutação que algum teste do pacote
cobre. Ponto cego, de propósito no relatório: a condição de um "case" não é comando para a cobertura do Go, então
mutação dentro de "case" sai como NOT COVERED e nunca é testada. Nessas regras, valem as mutações à mão no PR.

No PR (#456), com --base, mede só os pacotes da lista que o PR mudou (como o testes-afetados.py, ADR 0047); toda
semana e à mão, pelo workflow "Teste de mutação (Go)", mede a lista inteira. Antes de medir, confere que todo pacote
de internal/ com código fora de teste está na lista ou em mutacao-go-fora.txt, com o motivo: pacote novo não fica de
fora sem ninguém perceber.

O gerenciador em Go (src/gerenciador-go, ADR 0052) é outro módulo, com a lista e as equivalentes dele
(mutacao-gerenciador-go.txt e mutacao-gerenciador-go-equivalentes.txt). No PR que o muda, mede a lista dele inteira
(#433), na CI, para o teste pesado não rodar na máquina de quem desenvolve.

Uso:   python3 scripts/ci/mutacao-go.py                        # todos os pacotes de mutacao-go.txt
       python3 scripts/ci/mutacao-go.py internal/parametros    # só os dados (fora da lista: mede, sem piso)
       python3 scripts/ci/mutacao-go.py --base origin/develop  # no PR: só os pacotes que mudaram desde a base
       python3 scripts/ci/mutacao-go.py --conferir-lista       # só confere a lista (rápido, sem medir)
       python3 scripts/ci/mutacao-go.py --gerenciador          # os do gerenciador (mutacao-gerenciador-go.txt)
       python3 scripts/ci/mutacao-go.py --gerenciador --base origin/develop  # no PR: se o gerenciador mudou
Saída: <módulo>/mutacao/<pacote>.json (o do gremlins) e <módulo>/mutacao/relatorio.md (também no resumo do job,
       quando roda no GitHub Actions).
"""
import argparse
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
MODULO = 'src/gerenciador-go' if GERENCIADOR else 'src/backend-go'
GO = os.path.join(RAIZ, *MODULO.split('/'))
SAIDA = os.path.join(GO, 'mutacao')
LISTA = 'mutacao-gerenciador-go.txt' if GERENCIADOR else 'mutacao-go.txt'
EQUIVALENTES = 'mutacao-gerenciador-go-equivalentes.txt' if GERENCIADOR else 'mutacao-go-equivalentes.txt'
TITULO = '## Teste de mutação (gerenciador em Go)' if GERENCIADOR else '## Teste de mutação (Go)'
# #456: os pacotes do back-end fora da mutação de propósito, com o motivo. O gerenciador não tem a conferência: a
# lista dele deixa de fora, de propósito, os pacotes que chamam o Windows (a mutação roda no Linux).
FORA = 'mutacao-go-fora.txt'
# Arquivos da própria mutação: se só eles mudaram, o PR mede um pacote pequeno, para o script novo rodar de verdade.
DA_MUTACAO = ('scripts/ci/mutacao-go', 'scripts/ci/mutacao-gerenciador-go', '.github/workflows/mutacao-go.yml')
PACOTE_DE_PROVA = 'internal/tipos'


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


def fora_de_proposito():
    """pacote → motivo de não estar na mutação."""
    resultado = {}
    for linha in ler_linhas(FORA):
        pacote, _, motivo = linha.strip().partition(' ')
        resultado[pacote] = motivo.strip()
    return resultado


def pacotes_do_modulo():
    """Os pacotes de internal/ com código fora de teste (testdata não é pacote)."""
    pacotes = set()
    for pasta, subpastas, arquivos in os.walk(os.path.join(GO, 'internal')):
        subpastas[:] = [d for d in subpastas if d != 'testdata']
        if any(a.endswith('.go') and not a.endswith('_test.go') for a in arquivos):
            pacotes.add(os.path.relpath(pasta, GO).replace(os.sep, '/'))
    return pacotes


def conferir_lista(limites, fora):
    """Todo pacote está na lista ou fora de propósito (num lugar só, com o motivo), e nenhum dos dois cita pacote
    que não existe. Devolve os erros, já com o que fazer."""
    existentes = pacotes_do_modulo()
    erros = []
    for p in sorted(existentes - set(limites) - set(fora)):
        erros.append(f'{p} não está em scripts/ci/{LISTA} nem em scripts/ci/{FORA}. Com regra de negócio: meça '
                     f'(Actions → "Teste de mutação (Go)" → Run workflow, com "{p}" em pacotes) e ponha na lista com o '
                     f'piso da medição. Sem regra (código gerado, só montagem, apoio a teste): ponha em {FORA}, com o '
                     f'motivo.')
    for p in sorted(set(limites) & set(fora)):
        erros.append(f'{p} está em scripts/ci/{LISTA} e em scripts/ci/{FORA}: deixe só num dos dois.')
    for p in sorted((set(limites) | set(fora)) - existentes):
        erros.append(f'{p} está na lista, mas não existe em {MODULO}: tire a linha.')
    for p, motivo in sorted(fora.items()):
        if not motivo:
            erros.append(f'{p} está em scripts/ci/{FORA} sem o motivo.')
    return erros


def mudados(base):
    """Arquivos que o PR mudou em relação à base (o que o merge leva: base...HEAD)."""
    saida = subprocess.run(['git', 'diff', '--name-only', f'{base}...HEAD'], cwd=RAIZ, check=True,
                           capture_output=True, text=True).stdout
    return [a for a in saida.splitlines() if a]


def pacotes_mudados(arquivos, limites):
    """Os pacotes da lista com arquivo .go mudado, de código ou de teste (teste mais fraco também baixa a eficácia), ou
    com arquivo mudado no testdata/ deles (um gabarito muda o que os testes conferem)."""
    prefixo = MODULO + '/'
    pacotes = set()
    for a in arquivos:
        if not a.startswith(prefixo):
            continue
        caminho = a[len(prefixo):]
        if '/testdata/' in caminho:
            pasta = caminho.split('/testdata/')[0]
        elif caminho.endswith('.go'):
            pasta = os.path.dirname(caminho)
        else:
            continue
        if pasta in limites:
            pacotes.add(pasta)
    return sorted(pacotes)


def escolher_no_pr(arquivos, limites):
    """O que medir no PR: no back-end, os pacotes mudados; no gerenciador, a lista dele inteira, se ele mudou. Se só a
    mutação mudou (script, listas, workflow), um pacote pequeno, para provar que ela ainda roda."""
    da_mutacao = any(a.startswith(DA_MUTACAO) for a in arquivos)
    if GERENCIADOR:
        mudou = any(a.startswith(MODULO + '/') for a in arquivos)
        return list(limites) if mudou or da_mutacao else []
    pacotes = pacotes_mudados(arquivos, limites)
    if not pacotes and da_mutacao:
        pacotes = [PACOTE_DE_PROVA]
    return pacotes


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
    # O andamento do gremlins (sem --silent) fica no log: mostra o último mutante quando um deles faz o teste rodar sem
    # parar e o job cai (#433 no gerenciador; #455 no back-end, com o internal/carteira).
    comando = ['go', 'run', GREMLINS, 'unleash', './' + pacote, '--output', saida_json] + OPERADORES + EXECUCAO
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
        piso = limites.get(r['pacote'])
        situacao = ' ❌' if piso is not None and r['eficacia'] < piso else ''
        texto_piso = f'{piso:.0f}%' if piso is not None else '— (fora da lista)'
        linhas.append(f"| `{r['pacote']}` | {r['mortas']} | {len(r['vivas'])} | {len(r['equivalentes'])} | "
                      f"{r['sem_teste']} | {r['eficacia']:.1f}%{situacao} | {texto_piso} |")
    linhas += ['', 'Eficácia = mortas / (mortas + vivas), sem as equivalentes conhecidas. "Sem teste" inclui as mutações '
               'dentro de `case`, que o gremlins não testa (ponto cego da cobertura do Go).']
    for r in resultados:
        if r['vivas']:
            linhas += ['', f"### Vivas em `{r['pacote']}`", '']
            linhas += [f"- `{a}:{l}:{c}` {t}: `{trecho}`" for a, l, c, t, trecho, _ in r['vivas']]
    return '\n'.join(linhas) + '\n'


def escrever(texto):
    """O relatório no arquivo, no resumo do job (no GitHub Actions) e na saída."""
    os.makedirs(SAIDA, exist_ok=True)
    with open(os.path.join(SAIDA, 'relatorio.md'), 'w', encoding='utf-8') as f:
        f.write(texto)
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as f:
            f.write(texto)
    print(texto)


def main():
    argumentos = argparse.ArgumentParser(description='Teste de mutação do Go (#373, #456).')
    argumentos.add_argument('pacotes', nargs='*', help='pacotes a medir (vazio: a lista inteira)')
    argumentos.add_argument('--gerenciador', action='store_true', help='o módulo do gerenciador, e não o back-end')
    argumentos.add_argument('--base', help='no PR: medir só o que mudou desde esta base (ex.: origin/develop)')
    argumentos.add_argument('--conferir-lista', action='store_true', help='só conferir a lista, sem medir')
    opcoes = argumentos.parse_args()

    limites = pisos()
    if not GERENCIADOR:
        erros = conferir_lista(limites, fora_de_proposito())
        for e in erros:
            print(f'::error::{e}')
        if erros:
            sys.exit(1)
    if opcoes.conferir_lista:
        print(f'Lista em dia: todo pacote de {MODULO}/internal está em {LISTA} ou em {FORA}.')
        return

    if opcoes.pacotes:
        pedidos = opcoes.pacotes
    elif opcoes.base:
        pedidos = escolher_no_pr(mudados(opcoes.base), limites)
        if not pedidos:
            escrever(f'{TITULO}\n\nNada a medir: o PR não mexe em pacote da lista ({MODULO}).\n')
            return
    else:
        pedidos = list(limites)

    conhecidas = equivalentes()
    resultados = [medir(p, conhecidas) for p in pedidos]
    escrever(relatorio(resultados, limites))
    abaixo = [r for r in resultados if r['pacote'] in limites and r['eficacia'] < limites[r['pacote']]]
    for r in abaixo:
        print(f"::error::{r['pacote']}: eficácia {r['eficacia']:.1f}% abaixo do piso de {limites[r['pacote']]:.0f}%: "
              "um teste deixou de pegar mutação (veja as vivas no relatório)")
    sys.exit(1 if abaixo else 0)


if __name__ == '__main__':
    main()
