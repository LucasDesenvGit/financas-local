### Correções
- **Trocar de capítulo no Manual volta ao topo** (#820). As setas "anterior" e "próximo" e o "Índice do manual" no fim da página agora abrem o outro capítulo já no começo. O mesmo vale ao trocar de tela pelo menu; filtros e a busca não mexem na rolagem.
  - Técnico: o `AppComponent` rola ao topo no `NavigationEnd` quando o caminho muda, sem fragmento e fora do `popstate`.
