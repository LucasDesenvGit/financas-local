# 0061. Acesso pelo celular na rede de casa: desligado por padrão, HTTPS pelo IP com uma autoridade local e porta só para a rede local

- **Status:** Aceita (2026-11-07, pelo usuário no PR #672, sabendo que contraria o "só escuta em `127.0.0.1`" da 0002)
- **Data:** 2026-11-07 (issue #328)
- **Relacionadas:** substitui em parte a [0002](0002-sqlite-e-kestrel-no-lugar-de-sql-server-e-iis.md) e o "sem exposição na rede" de [SEGURANCA.md](../SEGURANCA.md) e do [modelo de ameaças](../MODELO-DE-AMEACAS.md) (o site escuta só em `127.0.0.1`); usa a [0015](0015-parametros-do-sistema-no-banco-editaveis-pelo-site.md) e a [0049](0049-back-end-em-go-trocado-aos-poucos-atras-do-dotnet.md)

## Contexto

O site respondia só em `127.0.0.1:5443`: um endereço que não seja de loopback é recusado ao iniciar (`config.ConferirSoLocal`), e o host é validado contra DNS rebinding (só `localhost` e `127.0.0.1`). Essa é a base do modelo de ameaças: "o risco real não vem da rede".

A #328 pediu para usar o site no celular, na mesma rede Wi-Fi de casa (ex.: `https://192.168.0.10:5443`):
- com o certificado válido também para o IP da rede, sem o aviso de site inseguro;
- expondo a porta na rede sem expor demais;
- e com uma skill que confira cada mudança de tela no celular.

O que já protege o site e continua valendo:
- o login, com bloqueio depois de errar a senha e limite de tentativas por minuto, que já era **por IP** (`autenticacao.limitador`): cada aparelho tem o seu;
- o cookie de sessão `__Host-` (HttpOnly, Secure, SameSite=Strict), que funciona com um IP no lugar do nome;
- o token antifalsificação em todo pedido que muda dados (não depende do nome do site);
- a CSP e os cabeçalhos.

O que muda o risco:
- **Outros aparelhos da rede passam a alcançar a porta:** um roteador comprometido, um convidado no Wi-Fi, um aparelho "inteligente" com falha. O login passa a ser a única barreira para quem está na rede.
- **O certificado do computador** é autoassinado para `localhost` e `127.0.0.1`. O celular não confia nele.

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
- **Ligar pelo gerenciador** (a proposta, antes de aceitar), com o UAC no Windows:
  - **Contras:** o mesmo código em dois lugares (o gerenciador e o site), o UAC a cada mudança, e a configuração do serviço que o `.msi` e os scripts regravam a cada atualização (o acesso desligaria sozinho). Ficou para o site, abaixo.
- **Escutar também no IP da rede, desligado por padrão, ligado pelo próprio site, com a autoridade local e o firewall só para a rede de casa.** **Escolhida.**

## Decisão

**Desligado por padrão.** Nada muda para quem não ligar: o site continua só em `127.0.0.1`, e o `ConferirSoLocal` continua valendo para o endereço do site. O certificado do computador (o `.pfx` da instalação) não muda, ligado ou não.

**Ligar** é o quadro "Acesso pelo celular", em Configurações › Segurança (só com login), nos dois sistemas:
- a pessoa escolhe um dos IPs deste computador na rede de casa (a API lista só IPv4 privado, RFC 1918, das interfaces ligadas, com o da rota padrão primeiro) e confirma, numa janela que diz que a senha passa a ser a proteção;
- o site abre **um segundo servidor**, só nesse IP e na mesma porta (5443), com o mesmo pipeline (login, CSRF, cabeçalhos), mas só com esse IP nos hosts permitidos (contra DNS rebinding) e o certificado abaixo. O `127.0.0.1` continua com o servidor de sempre;
- a escolha fica na tabela `Configuracao` (`AcessoCelular.Ip`), e ao subir o site liga de novo. O IP que saiu do computador (o roteador deu outro) não liga: a tela mostra e a pessoa escolhe de novo;
- ligar e desligar entram na auditoria de acesso (`AcessoCelularLigado`, `AcessoCelularDesligado`), que também já grava o IP de cada login: o celular aparece pelo IP dele em "Acessos recentes";
- desligar fecha o servidor da rede na hora (as conexões dos celulares caem), apaga a regra do firewall e a escolha.

O código é o pacote `internal/plataforma/celular` do back-end em Go.

**Certificado do celular:**
- Na primeira vez que liga, o site cria uma **autoridade certificadora (CA) local** (ECDSA P-256, 10 anos), com a chave em `dados/chaves/celular-autoridade.chave`, ao lado da chave da sessão e com a mesma proteção (só a conta do serviço lê). A chave nunca sai do computador.
- A autoridade tem **restrição de nomes** (*name constraints*): só assina IPs de `10/8`, `172.16/12` e `192.168/16`, e nenhum nome de DNS (só `invalid`, que nunca existe). Quem roubar a chave não consegue um certificado de um site da internet em que o celular confie. A extensão é **não crítica**, de propósito (*julgamento*): quem a entende (Chrome, Android, o Go) a cumpre, e um celular que não a entendesse continua aceitando a autoridade, em vez de recusá-la inteira.
- O certificado do site para o IP é assinado por ela a cada vez que o site liga o acesso (ao subir também), com uma chave nova que só vive na memória. Vale 397 dias, dentro do limite do iOS e do Chrome (398), e o IP no SAN.
- O celular instala a autoridade uma vez. O site a entrega em `GET /api/celular/autoridade`, **sem login**, só com o acesso ligado: é só o certificado público, e o celular precisa dela antes de confiar no site e de entrar. Na primeira vez, o navegador do celular avisa que a conexão não é particular (é esperado). Para conferir, a tela mostra a impressão digital (SHA-256), que o iPhone mostra ao instalar. Dá também para baixar no computador e mandar de outro jeito.
- No iOS, é preciso ligar a confiança total em Ajustes › Geral › Sobre › Ajustes de Confiança de Certificados. O Manual dá o passo a passo do Android e do iPhone.
- O IP mudou: desligar e ligar no IP novo. A autoridade é a mesma, e o celular não instala de novo.

**Firewall só para a rede de casa:**
- **Windows:** o serviço roda como LocalSystem e cria, pelo `netsh`, uma regra de entrada só para a porta e o IP do site, só no perfil **Particular**, com `remoteip=localsubnet`. No perfil Público (Wi-Fi de café, aeroporto) a porta fica fechada; se a rede de casa estiver como Pública, a tela avisa como mudar (`Get-NetConnectionProfile`). Desligar apaga a regra.
- **Linux:** o serviço roda na conta da pessoa (`systemctl --user`), sem root, então **não mexe** no firewall. Com o `ufw` ou o `firewalld` ligados, a tela mostra o comando, com `sudo`, para a sub-rede do IP. Sem eles, a porta já está aberta para a rede (o padrão das distribuições de computador).
- Nada de redirecionar porta no roteador.

**A skill `revisar-no-celular`** (`.claude/skills/`) confere cada mudança de tela no celular: o layout, o toque, o alvo de 24 px e o teclado virtual. Ela entra no `CLAUDE.md` ao lado da `testar-no-navegador`. Ela não depende da rede: usa o projeto "celular" do Playwright e, quando o acesso estiver ligado, um celular de verdade.

**Ficou de fora desta vez** (issues à parte, se fizer falta):
- o **QR code** do endereço: pede um gerador de QR (dependência nova no front-end ou no Go, que pediria ADR); a tela mostra o endereço por escrito;
- o aviso de **"novo aparelho entrou"**: a auditoria já mostra o IP de cada login;
- a conferência, nos workflows de instalação, de que desligado a porta não escuta fora do `127.0.0.1`: é mudança só de CI, que vai num PR `ci:` (ADR 0039). O teste do Go já confere que, desligado, o site não abre nada na rede.

## Consequências

- **Positivas:**
  - o site no celular, em casa, com HTTPS de verdade e sem aviso (depois de instalar a autoridade);
  - **desligado, nada muda:** o risco novo é só de quem liga, e só na rede de casa;
  - o certificado e a confiança do computador não mudam;
  - nenhuma saída de rede nova e nenhum serviço de terceiro; nenhum script de instalação muda.
- **Negativas:**
  - **Quem está na rede de casa chega à porta do site.** O login, o bloqueio e o limite por IP passam a ser a proteção, e uma senha fraca vira risco real. A tela de ligar diz isso.
  - **A autoridade local é uma chave a proteger:** quem a roubar consegue um certificado em que o celular confia para os IPs da rede de casa (o roteador, outro aparelho), mas não para um site da internet (restrição de nomes). Ela fica em `dados/chaves`, com a proteção das outras chaves; a cópia do banco não a leva.
  - **A restrição de nomes não foi conferida num celular de verdade** (*não validado com uso real*): conferida no Go, que a cumpre como o Chrome. Se um celular recusar a autoridade, a causa provável é ela.
  - **Mais código de sistema operacional:** o `netsh` e o perfil de rede no Windows, os avisos do Linux.
  - **IP que muda** (DHCP): o endereço salvo no celular para de funcionar, e o site não liga ao subir até a pessoa escolher o IP novo. A tela recomenda reservar o IP no roteador.
  - **O modelo de ameaças ganha uma fronteira nova**, a rede de casa, e o `SEGURANCA.md` deixa de dizer "sem exposição na rede" sem ressalva.
- **Na prática:**
  - Tudo num PR só (#672), a pedido do usuário, no lugar das 6 partes da proposta.
  - Mudou alguma coisa aqui (a porta, o certificado, o firewall)? Revise no mesmo PR o `docs/MODELO-DE-AMEACAS.md`, o `docs/SEGURANCA.md`, os diagramas de `docs/ARQUITETURA.md` e o `docs/DADOS-PESSOAIS.md`.
  - A 0002 ganhou "substituída em parte por 0061" (o "só escuta em 127.0.0.1").
