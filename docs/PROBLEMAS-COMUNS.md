# Problemas comuns

Os mais frequentes também estão no [README](../README.md#problemas-comuns). Instalou pelos scripts? Os problemas de lá estão em [Instalar a partir do código](INSTALACAO-PELO-CODIGO.md#problemas-comuns-instalação-pelos-scripts).

## Instalação e atualização (`.msi`)

| Sintoma | O que fazer |
|---|---|
| "O Windows protegeu o computador" ao abrir o `.msi` | O instalador ainda não é assinado. Clique em **Mais informações › Executar assim mesmo** |
| A instalação falhou e o log diz que a porta 5443 está em uso | Outro programa está na porta do site, e o log do instalador diz qual e o PID. Feche esse programa (ou pare o serviço dele) e abra o `.msi` de novo. Depois de instalado, o gerenciador também avisa quando a porta está ocupada |
| A instalação parou dizendo que o banco da instalação antiga não foi encontrado | Nada foi mudado e o site antigo voltou a rodar. Confira o `Banco:Caminho` no `appsettings.Production.json` da instalação antiga (pasta `publicado\api`), ou faça uma cópia do banco e remova o serviço antigo antes de instalar |
| Uma atualização não funcionou | Siga [Atualização que falhou](OPERACAO.md#atualização-que-falhou) |
| Perdi dados ou quero voltar a uma cópia do banco | Siga [Restaurar uma cópia](OPERACAO.md#restaurar-uma-cópia) |
| O Client Secret da Pluggy não abre mais | Siga [Chaves perdidas](OPERACAO.md#chaves-perdidas) |
| O navegador alerta sobre o certificado | Abra o mesmo `.msi` de novo e escolha **Reparar**: ele recoloca o certificado nos confiáveis. No Firefox, em `about:config`, ative `security.enterprise_roots.enabled` |

## Gerenciador

| Sintoma | O que fazer |
|---|---|
| O gerenciador diz "ligado, mas não responde" | Espere alguns segundos (o site está subindo) e clique em **Reiniciar**. Se continuar, veja o Visualizador de Eventos do Windows › Aplicativo: os erros da API têm a origem `Financas.Api`, e o serviço que não sobe aparece com a origem `Service Control Manager` |
| O gerenciador não oferece a versão nova | Sem internet, ele avisa e o site continua funcionando. Você também pode baixar o `.msi` novo e abrir |

## Open Finance e o site

| Sintoma | O que fazer |
|---|---|
| A sincronização falha com 401/403 | Confira o Client ID e o Secret e se o MeuPluggy foi autorizado para aquele banco |
| O Client Secret da Pluggy não abre mais (trocou de computador ou apagou `dados\chaves`) | As chaves são de cada instalação: informe o Client Secret de novo em Configurações |
| Algo deu errado e não sei o quê | Veja **Configurações › Erros**: a mensagem, quando e quantas vezes aconteceu |
