# 0061. Acesso pelo celular na rede de casa: desligado por padrão, HTTPS pelo IP com uma autoridade local e porta só para a rede local

- **Status:** Proposta
- **Data:** 2026-11-07 (issue #328)
- **Relacionadas:** substitui em parte a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) e o "sem exposição na rede" de [SEGURANCA.md](../SEGURANCA.md) e do [modelo de ameaças](../MODELO-DE-AMEACAS.md) (o site escuta só em `127.0.0.1`); usa a [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) e a [0052](0052-gerenciador-em-go-com-bandeja-e-pagina-local.md)

## Contexto

O site responde só em `127.0.0.1:5443`: um endereço que não seja de loopback é recusado ao iniciar (`config.ConferirSoLocal`), e o host é validado contra DNS rebinding (só `localhost` e `127.0.0.1`). Essa é a base do modelo de ameaças: "o risco real não vem da rede".

A #328 pediu para usar o site no celular, na mesma rede Wi-Fi de casa (ex.: `https://192.168.0.10:5443`):
- com o certificado válido também para o IP da rede, sem o aviso de site inseguro;
- expondo a porta na rede sem expor demais;
- e com uma skill que confira cada mudança de tela no celular.

O que já protege o site e continuaria valendo:
- o login, com bloqueio depois de errar a senha e limite de tentativas por minuto;
- o cookie de sessão `__Host-` (HttpOnly, Secure, SameSite=Strict), que funciona com um IP no lugar do nome;
- o token antifalsificação em todo pedido que muda dados;
- a CSP e os cabeçalhos.

O que muda o risco:
- **Outros aparelhos da rede passam a alcançar a porta:** um roteador comprometido, um convidado no Wi-Fi, um aparelho "inteligente" com falha. O login passa a ser a única barreira para quem está na rede.
- **O certificado de hoje** é autoassinado para `localhost` e `127.0.0.1`. O celular não confia nele.

## Alternativas consideradas

- **Não abrir, e usar o túnel do cabo USB** (`adb reverse tcp:5443 tcp:5443`, só Android):
  - **Prós:** o site continua só em `127.0.0.1`, nada muda na segurança.
  - **Contras:** pede o modo de desenvolvedor e o cabo a cada vez; o iPhone fica de fora; não é "abrir no celular".
- **Túnel de VPN** (Tailscale ou WireGuard):
  - **Prós:** criptografado, funciona até fora de casa, e a porta não abre na rede local.
  - **Contras:**
    - o Tailscale é um serviço de terceiro, com conta, e uma saída de rede nova (ADR própria);
    - o WireGuard pede configurar chaves e o roteador;
    - é mais do que a issue pediu: a rede de casa.
- **Proxy reverso** (Caddy, nginx) na frente do site:
  - **Prós:** o certificado e o TLS ficam com uma ferramenta pronta.
  - **Contras:** mais um programa para instalar e atualizar nos dois sistemas, para o mesmo efeito. A ADR 0002 tirou o IIS por isso.
- **Escutar também no IP da rede, desligado por padrão, com a autoridade local e o firewall só para a rede de casa.** **Escolhida (proposta).**

## Decisão

**Desligado por padrão.** Nada muda para quem não ligar: o site continua só em `127.0.0.1`, e o `ConferirSoLocal` continua recusando o resto.

**Ligar** é uma opção "Acesso pelo celular", nos dois sistemas: no gerenciador (com o IP e o QR code do endereço) e nos scripts.
- A opção grava a escolha na configuração do serviço, e o site escuta também no IP da rede escolhido. O `127.0.0.1` continua.
- O serviço reinicia para valer.

**Certificado:**
- Na primeira vez que liga, o gerenciador gera uma **autoridade certificadora (CA) local**, com a chave guardada em `dados/chaves`, ao lado das outras chaves e com a mesma proteção. A chave nunca sai do computador.
- O certificado do site é assinado por essa CA, com `localhost`, `127.0.0.1` e o IP da rede no SAN.
- Quando o IP muda, o certificado é refeito.
- A CA (só o certificado público) vai para o celular uma vez:
  - pelo QR code ou pelo arquivo que o gerenciador salva;
  - nunca servida pelo site;
  - o Manual dá o passo a passo do Android e do iOS.

  No iOS, é preciso ligar a confiança total em Ajustes › Geral › Sobre.

**Firewall só para a rede de casa:**
- **Windows:** uma regra de entrada para a porta, só no perfil **Particular**, com `remoteip=LocalSubnet`. No perfil Público (Wi-Fi de café, aeroporto), a porta fica fechada.
- **Linux:** a regra no `ufw` ou no `firewalld` para a sub-rede local. Sem firewall ativo, a tela avisa.
- **Desligar apaga as regras.** Nada de redirecionar porta no roteador.

**Segurança que muda junto:**
- a validação de host aceita o IP escolhido;
- o limite de tentativas de login vale por IP de origem (hoje todo pedido vem de `127.0.0.1`);
- a auditoria grava o IP de cada acesso, que já é gravado, e avisa "novo aparelho entrou" no primeiro login de um IP que não é o local (*inferido*, a confirmar);
- os dois workflows de instalação conferem que, desligado, a porta não escuta fora do `127.0.0.1`.

**A skill `revisar-no-celular`** (`.claude/skills/`) confere cada mudança de tela no celular:
- o layout;
- o toque;
- o alvo de 24 px;
- o teclado virtual.

Ela entra no `CLAUDE.md` ao lado da `testar-no-navegador`. Ela não depende da rede: usa o projeto "celular" do Playwright e, quando o acesso estiver ligado, um celular de verdade.

## Consequências

- **Positivas:**
  - o site no celular, em casa, com HTTPS de verdade e sem aviso;
  - **desligado, nada muda:** o risco novo é só de quem liga, e só na rede de casa;
  - nenhuma saída de rede nova e nenhum serviço de terceiro.
- **Negativas:**
  - **Quem está na rede de casa chega à porta do site.** O login, o bloqueio e o limite por IP passam a ser a proteção, e uma senha fraca vira risco real. A tela de ligar deve dizer isso.
  - **A CA local é uma chave poderosa:** quem a roubar pode assinar certificados em que o celular confia, para qualquer site. Ela fica em `dados/chaves`, com a proteção das outras chaves. Uma alternativa a avaliar na implementação é restringir a CA por *name constraints* (só o IP e `localhost`).
  - **Mais código de sistema operacional nos dois lados:** firewall, certificado, serviço, nos scripts, no `.msi` (`instalador/configurar.ps1`) e no gerenciador.
  - **IP que muda** (DHCP): o certificado é refeito, mas o endereço que a pessoa salvou no celular para de funcionar. Recomendar reservar o IP no roteador.
  - **O modelo de ameaças ganha uma fronteira nova**, a rede local, e o `SEGURANCA.md` deixa de dizer "sem exposição na rede" sem ressalva.
- **Na prática:**
  - **Em partes, um PR cada:**
    1. esta ADR;
    2. a CA local e o certificado com o IP no SAN (scripts bash e PowerShell, `.msi`);
    3. escutar no IP com a validação de host e o limite por IP (Go, com teste);
    4. a opção nos dois gerenciadores, com o firewall, o QR code e o desligar;
    5. a skill `revisar-no-celular` e a regra no `CLAUDE.md`;
    6. o Manual.
  - **Revisar no mesmo PR de cada parte:**
    - `docs/MODELO-DE-AMEACAS.md`;
    - `docs/SEGURANCA.md`;
    - os diagramas de `docs/ARQUITETURA.md` (a rede local vira uma fronteira);
    - `docs/DADOS-PESSOAIS.md`, para o IP dos aparelhos na auditoria.
  - **Ao aceitar:** a 0002 ganha "Substituída em parte por 0061" (o "só escuta em 127.0.0.1").
