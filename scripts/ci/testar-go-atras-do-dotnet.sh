#!/usr/bin/env bash
# =====================================================================
# #231 (ADR 0049): teste de ponta a ponta da convivência do .NET com o Go.
# Liga a API .NET compilada com o Go compilado ao lado (processo filho) e
# confere, como o front-end faria:
#   - /api/go/saude sem login: 401 do próprio .NET (mesmo com o Go fora);
#   - com login: 200 vindo do Go, que leu o banco (usuário e modo WAL);
#   - cabeçalhos X-Financas-* vindos de fora não chegam ao Go;
#   - POST sem o token CSRF no repasse: 400; com o token: chega ao Go;
#   - direto no Go, sem o segredo interno: 401;
#   - o .NET e o Go gravando no banco ao mesmo tempo, sem erro;
#   - o Go cai: 503 enquanto isso, e ele volta sozinho;
#   - o .NET parado (SIGTERM) ou morto (SIGKILL): nenhum Go sobra;
#   - Go:Prefixos vazio: a rota volta para o .NET (404), sem Go rodando.
#
# Banco, chaves e certificado numa pasta temporária (nunca o banco real).
# Uso (depois do dotnet build):  scripts/ci/testar-go-atras-do-dotnet.sh [Release|Debug]
# Porta: PORTA_TESTE (padrão 5081; o ligar-api.sh usa a 5080).
# =====================================================================
set -euo pipefail

configuracao="${1:-Release}"
raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
dll="$raiz/src/backend/Financas.Api/bin/$configuracao/net10.0/Financas.Api.dll"
[[ -f "$dll" ]] || { echo "API não compilada em $dll (rode o dotnet build antes)."; exit 1; }
command -v go >/dev/null || { echo "Go não encontrado no PATH."; exit 1; }
command -v sqlite3 >/dev/null || { echo "sqlite3 não encontrado no PATH."; exit 1; }

pasta="$(mktemp -d)"
log="$pasta/api.log"
porta="${PORTA_TESTE:-5081}"
url="https://127.0.0.1:$porta"
banco="$pasta/sql/FinancasLocal.db"
cookies="$pasta/cookies.txt"
go_exe="$pasta/financas-go"
pid=""

passo() { printf '\n==> %s\n' "$*"; }
ok() { printf '    OK  %s\n' "$*"; }
falha() {
    printf '    FALHOU  %s\n' "$*"
    echo "--- últimas linhas do log da API ---"; tail -n 60 "$log" || true
    exit 1
}
processos_go() { pgrep -f -- "$go_exe" || true; }
encerrar() {
    [[ -n "$pid" ]] && { kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; }
    for p in $(processos_go); do kill -9 "$p" 2>/dev/null || true; done
    [[ -n "${MANTER_PASTA:-}" ]] && echo "pasta mantida: $pasta" || rm -rf "$pasta"
}
trap encerrar EXIT

# status [opções do curl...]: o código HTTP; o corpo fica em $pasta/corpo.
status() { curl -sk -o "$pasta/corpo" -w '%{http_code}' "$@"; }
token_csrf() { awk '$6 == "__Host-XSRF-TOKEN" { print $7 }' "$cookies" | tail -n1; }

passo "Compilando o Go ao lado da API (versão do Directory.Build.props)"
versao="$(sed -n 's:.*<Version>\(.*\)</Version>.*:\1:p' "$raiz/Directory.Build.props" | head -n1)"
(cd "$raiz/src/backend-go" &&
    go build -ldflags "-X github.com/LucasDesenvGit/financas-local/src/backend-go/internal/versao.Atual=$versao" -o "$go_exe" ./cmd/financas &&
    go test -c -o "$pasta/banco.test" ./internal/banco)
ok "financas-go $versao"

export ASPNETCORE_ENVIRONMENT=Production
export Kestrel__Endpoints__Https__Url="$url"
senha="$(head -c 18 /dev/urandom | base64 | tr -d '/+=')"
dotnet dev-certs https -ep "$pasta/teste.pfx" -p "$senha" >/dev/null
export Kestrel__Endpoints__Https__Certificate__Path="$pasta/teste.pfx"
export Kestrel__Endpoints__Https__Certificate__Password="$senha"
export Banco__Caminho="$banco"
export Seguranca__PastaChaves="$pasta/chaves"
export Go__Executavel="$go_exe"
mkdir -p "$pasta/sql" "$pasta/chaves"

# ligar: sobe a API e espera /api/auth/estado; com Go:Prefixos, espera também o Go avisar a porta.
ligar() {
    local esperar_go="$1"
    : >"$log"
    # De dentro da pasta da API, como o serviço (WorkingDirectory): é de lá que o .NET lê o appsettings.json,
    # com o Go:Prefixos padrão.
    (cd "$(dirname "$dll")" && exec dotnet "$dll") >>"$log" 2>&1 &
    pid=$!
    for _ in $(seq 1 60); do
        kill -0 "$pid" 2>/dev/null || falha "a API parou ao iniciar"
        if curl -fsSk "$url/api/auth/estado" >/dev/null 2>&1; then
            if [[ "$esperar_go" == sim ]]; then
                grep -q 'Go no ar em' "$log" && return 0
            else
                return 0
            fi
        fi
        sleep 1
    done
    falha "a API (ou o Go) não subiu em 60 s"
}
porta_do_go() { grep -oE 'Go no ar em http://127\.0\.0\.1:[0-9]+' "$log" | tail -n1 | grep -oE '[0-9]+$'; }
# esperar_sem_go SEGUNDOS: espera não haver nenhum processo do Go.
esperar_sem_go() {
    for _ in $(seq 1 "$1"); do
        [[ -z "$(processos_go)" ]] && return 0
        sleep 1
    done
    return 1
}

# ------------------------------------------------------------------
passo "1. Liga a API com Go:Prefixos=/api/go (o padrão do appsettings.json)"
ligar sim
pid_go="$(processos_go)"
[[ -n "$pid_go" ]] || falha "o Go não está rodando"
filhos="$(pgrep -P "$pid" -f -- "$go_exe" || true)"
[[ "$filhos" == "$pid_go" ]] || falha "o Go ($pid_go) não é processo filho do .NET ($pid)"
ok "o Go (processo $pid_go) é filho do .NET (processo $pid), na porta $(porta_do_go)"
modo="$(sqlite3 "$banco" 'PRAGMA journal_mode;')"
[[ "$modo" == wal ]] || falha "o banco está em $modo, e não em WAL"
ok "banco em WAL"

passo "2. Sem login: 401 do .NET"
s="$(status "$url/api/go/saude")"
[[ "$s" == 401 ]] || falha "esperado 401, veio $s"
ok "401"

passo "3. Cria o usuário e entra (com o token CSRF, como o front-end)"
curl -fsSk -c "$cookies" -b "$cookies" "$url/api/auth/estado" >/dev/null
s="$(status -c "$cookies" -b "$cookies" -H "X-XSRF-TOKEN: $(token_csrf)" -H 'Content-Type: application/json' \
    -d '{"login":"verificador","senha":"Teste#Go2026"}' "$url/api/auth/configurar")"
[[ "$s" == 200 ]] || falha "configurar: $s $(cat "$pasta/corpo")"
ok "logado como verificador"

passo "4. Com login: 200 vindo do Go, que leu o banco"
s="$(status -b "$cookies" "$url/api/go/saude")"
corpo="$(cat "$pasta/corpo")"
[[ "$s" == 200 ]] || falha "esperado 200, veio $s: $corpo"
for esperado in '"situacao":"ok"' "\"versao\":\"$versao\"" '"usuario":"verificador"' '"diarioDoBanco":"wal"'; do
    [[ "$corpo" == *"$esperado"* ]] || falha "faltou $esperado em $corpo"
done
cabecalhos="$(curl -sk -D - -o /dev/null -b "$cookies" "$url/api/go/saude")"
grep -qi '^cache-control: no-store' <<<"$cabecalhos" || falha "sem Cache-Control: no-store na resposta repassada"
grep -qi '^content-security-policy:' <<<"$cabecalhos" || falha "sem CSP na resposta repassada"
ok "$corpo"

passo "5. X-Financas-* vindos de fora não chegam ao Go"
s="$(status -b "$cookies" -H 'X-Financas-Usuario: 999' -H 'X-Financas-Segredo: inventado' "$url/api/go/saude")"
[[ "$s" == 200 && "$(cat "$pasta/corpo")" == *'"usuario":"verificador"'* ]] || falha "$s $(cat "$pasta/corpo")"
ok "o Go viu o usuário da sessão, e não o do cabeçalho"

passo "6. CSRF no repasse"
s="$(status -b "$cookies" -X POST -H 'Content-Type: application/json' -d '{}' "$url/api/go/saude")"
[[ "$s" == 400 ]] || falha "POST sem token: esperado 400, veio $s"
ok "POST sem token: 400"
s="$(status -b "$cookies" -X POST -H "X-XSRF-TOKEN: $(token_csrf)" -H 'Content-Type: application/json' -d '{}' "$url/api/go/saude")"
[[ "$s" == 404 ]] || falha "POST com token: esperado o 404 do Go (rota só tem GET), veio $s"
ok "POST com token: passou do CSRF e chegou ao Go (404: a rota só tem GET)"

passo "7. Direto no Go, sem o .NET"
porta_go="$(porta_do_go)"
s="$(status "http://127.0.0.1:$porta_go/api/go/saude")"
[[ "$s" == 401 ]] || falha "sem segredo: esperado 401, veio $s"
s="$(status -H 'X-Financas-Segredo: 0000000000000000000000000000000000000000000000000000000000000000' \
    -H 'X-Financas-Usuario: 1' "http://127.0.0.1:$porta_go/api/go/saude")"
[[ "$s" == 401 ]] || falha "segredo errado: esperado 401, veio $s"
ok "401 sem o segredo e com o segredo errado"

passo "8. O .NET e o Go gravando no banco ao mesmo tempo (6 s)"
# O Go segura a trava de gravação 5 ms por transação: o .NET encontra o banco ocupado e tem de esperar.
FINANCAS_GRAVAR_JUNTO="$banco" FINANCAS_GRAVAR_JUNTO_SEGUNDOS=6 FINANCAS_GRAVAR_JUNTO_QUEM=e2e \
    "$pasta/banco.test" -test.run '^TestGravarJuntoComOutroProcesso$' -test.count=1 -test.v >"$pasta/gravador.log" 2>&1 &
pid_gravador=$!
criadas=0
while kill -0 "$pid_gravador" 2>/dev/null; do
    criadas=$((criadas + 1))
    s="$(status -b "$cookies" -H "X-XSRF-TOKEN: $(token_csrf)" -H 'Content-Type: application/json' \
        -d "{\"nome\":\"Teste WAL $criadas\",\"tipo\":1,\"cor\":\"#123456\"}" "$url/api/categorias")"
    [[ "$s" == 200 ]] || falha "POST /api/categorias $criadas: $s $(cat "$pasta/corpo")"
    s="$(status -b "$cookies" "$url/api/go/saude")"
    [[ "$s" == 200 ]] || falha "GET /api/go/saude durante as gravações: $s"
done
wait "$pid_gravador" || { cat "$pasta/gravador.log"; falha "o Go teve erro gravando junto com o .NET"; }
feitas_go="$(grep -oE 'transacoes=[0-9]+' "$pasta/gravador.log" | cut -d= -f2)"
n_go="$(sqlite3 "$banco" "SELECT COUNT(*) FROM Configuracao WHERE Chave LIKE 'teste-wal-e2e-%';")"
n_net="$(sqlite3 "$banco" "SELECT COUNT(*) FROM Categoria WHERE Nome LIKE 'Teste WAL %';")"
[[ -n "$feitas_go" && "$n_go" == "$feitas_go" && "$n_net" == "$criadas" ]] ||
    falha "gravados: Go $n_go de ${feitas_go:-?}, .NET $n_net de $criadas"
[[ "$criadas" -ge 10 ]] || falha "o .NET gravou só $criadas vezes em 6 s: o teste não mostrou nada"
if grep -i 'database is locked' "$log"; then falha "a API registrou banco travado"; fi
ok "Go: $feitas_go transações; .NET: $criadas categorias e $criadas leituras pelo repasse; nenhuma falha"

passo "9. O Go cai: 503 enquanto isso, e ele volta sozinho"
kill -9 "$pid_go"
s="$(status -b "$cookies" "$url/api/go/saude")"
[[ "$s" == 503 && "$(cat "$pasta/corpo")" == *'indispon'* ]] || falha "com o Go fora: esperado 503, veio $s $(cat "$pasta/corpo")"
s="$(status "$url/api/go/saude")"
[[ "$s" == 401 ]] || falha "sem login, com o Go fora: esperado 401 do .NET, veio $s"
ok "503 com login; 401 sem login (o login é conferido antes do repasse)"
for _ in $(seq 1 20); do
    s="$(status -b "$cookies" "$url/api/go/saude")"
    [[ "$s" == 200 ]] && break
    sleep 1
done
[[ "$s" == 200 ]] || falha "o Go não voltou em 20 s"
novo_go="$(processos_go)"
[[ -n "$novo_go" && "$novo_go" != "$pid_go" ]] || falha "o Go não foi iniciado de novo"
grep -q 'O Go parou sozinho' "$log" || falha "a queda do Go não foi registrada"
ok "o Go voltou (processo $novo_go)"

# ADR 0022: o aviso do próprio Go (passo 7) e a queda vão para Configurações › Erros. O gravador do .NET grava
# a cada poucos segundos.
for _ in $(seq 1 15); do
    erros="$(sqlite3 -separator '|' "$banco" "SELECT Nivel, Origem, Categoria, Mensagem FROM ErroSistema ORDER BY Id;")"
    [[ "$erros" == *'O Go parou sozinho'* && "$erros" == *'Financas.Go.API'* ]] && break
    sleep 1
done
grep -qF '3|API|Financas.Go.API|requisição recusada: sem o segredo interno' <<<"$erros" ||
    falha "o aviso do Go não chegou à tabela de erros com a origem API: $erros"
grep -qE '^4\|Sistema\|Financas\.Api\.Servicos\.Go\.ProcessoGo\|O Go parou sozinho' <<<"$erros" ||
    falha "a queda do Go não chegou à tabela de erros: $erros"
ok "na tabela de erros: o aviso do Go (Financas.Go.API) e a queda (ProcessoGo)"

passo "10. O .NET para (SIGTERM): o Go para junto"
kill -TERM "$pid"
wait "$pid" 2>/dev/null || true
pid=""
esperar_sem_go 15 || falha "sobrou Go rodando: $(processos_go)"
grep -q 'Go parado' "$log" || falha "o .NET não registrou a parada do Go"
ok "nenhum Go sobrando"

passo "11. O .NET morre (SIGKILL): o Go para sozinho (a entrada padrão fechou)"
ligar sim
pid_go="$(processos_go)"
kill -9 "$pid"
wait "$pid" 2>/dev/null || true
pid=""
esperar_sem_go 15 || falha "sobrou Go órfão: $(processos_go)"
ok "o Go $pid_go parou sozinho"

passo "12. Go:Prefixos vazio: a rota volta para o .NET, sem reinstalar"
Go__Prefixos="" ligar nao
sleep 2
[[ -z "$(processos_go)" ]] || falha "o Go foi iniciado sem nenhum prefixo"
s="$(status -b "$cookies" "$url/api/go/saude")"
[[ "$s" == 404 ]] || falha "esperado 404 do .NET, veio $s"
ok "404 do .NET (MapFallback de /api) e nenhum processo do Go"

echo
echo "OK: o .NET e o Go convivem (#231)."
