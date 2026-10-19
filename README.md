# CTech DFe

Plataforma SaaS para emissão e gerenciamento de Documentos Fiscais Eletrônicos (NF-e, NFC-e, CT-e, MDF-e, NFS-e) com
comunicação direta à SEFAZ (sempre pela Lambda `go-dfe-egress` em sa-east-1).

## Documentação

**Documentação jurídica vigente:** o aditivo DF-e publicado pela Central Jurídica
do CTech está na versão **2.1**. A API reexige o aceite quando
`CurrentTermsAddendumVersion` muda; a fonte pública de verdade é
`https://accounts.aoctech.app/products/dfe`.

| Documento                        | Descrição                              |
|----------------------------------|----------------------------------------|
| [`OVERVIEW.md`](OVERVIEW.md)     | Visão geral do sistema — leia primeiro |
| [`DOCS.md`](DOCS.md)             | Documentação técnica completa          |
| [`CONDUCT.md`](CONDUCT.md)       | Código de conduta de desenvolvimento   |
| [`CLAUDE.md`](CLAUDE.md)         | Instruções para Claude Code            |
| [`AGENTS.md`](AGENTS.md)         | Contexto para agentes de IA            |
| [`DEPLOYMENT.md`](DEPLOYMENT.md) | Deploy, bootstrap, troubleshooting CDK |
| [`THEME.md`](THEME.md)           | Design system e paleta de cores        |

## Subprojetos

| Diretório | Papel                                               | README                               |
|-----------|-----------------------------------------------------|--------------------------------------|
| `py-dfe/` | **Aposentado** (nenhum chamador; Lambda ainda implantada) | [py-dfe/README.md](py-dfe/README.md) |
| `go-dfe/` | Biblioteca Go SEFAZ/NFS-e (mTLS), executada pela `go-dfe-egress` | [go-dfe/README.md](go-dfe/README.md) |
| `go-dfe-egress/` | Lambda Go em sa-east-1 que executa o go-dfe | [go-dfe-egress/CLAUDE.md](go-dfe-egress/CLAUDE.md) |
| `api/`    | Backend REST — Go (Fiber v3), multi-tenant          | [api/README.md](api/README.md)       |
| `ui/`     | Frontend SaaS — Next.js (export estático, Cloudflare) + TypeScript | [ui/README.md](ui/README.md)         |
| `worker/` | Workers assíncronos — Go Lambda (SQS consumers)     | [worker/README.md](worker/README.md) |
| `cdk/`    | Infraestrutura AWS — CDK TypeScript                 | [cdk/README.md](cdk/README.md)       |

```
py-dfe/     # Lambda Python aposentada (remoção pendente)
go-dfe/     # Biblioteca Go SEFAZ/NFS-e (mTLS)
go-dfe-egress/ # Lambda Go (sa-east-1) que executa o go-dfe
api/        # Backend REST — Go (Fiber v3), multi-tenant
ui/         # Frontend SaaS — Next.js + TypeScript
worker/     # Workers assíncronos — Go Lambda (SQS consumers)
cdk/        # Infraestrutura AWS — CDK TypeScript
```

## Início Rápido

```bash
# Biblioteca SEFAZ
cd go-dfe && go test ./... -race

# Backend
cd api && go run ./cmd/server

# Frontend
cd ui && npm install && npm run dev

# Infraestrutura
cd cdk && npm install && cdk synth
```

## Exclusão de conta: toda feature nova entrega seu fluxo de exclusão

A ctech-account orquestra um saga de exclusão (LGPD) em toda a família CTech (specs e revisão em
`ctech-account/docs/specs/2026-10-06-account-deletion-*.md` e `docs/2026-10-13-account-deletion-plan-review.md`).
Cada serviço é participante. Os planos de participante envelheceram conforme features novas entraram, então vale esta regra:

**Sair de um único produto (`scope: service`).** O ctech-account deixa a pessoa sair de um produto CTech sem encerrar a conta.
A mensagem chega sem workspaces, então o purge apaga só os dados da própria pessoa e deixa os dados dos workspaces; o mesmo
consumer limpa o tombstone com `user.relinked` quando ela volta (`api-commons` ≥ v1.28.0, `Store.Relink`). Nada neste repo muda
por isso. Spec: ctech-account `docs/specs/2026-10-17-service-unlink.md`.

**Uma feature que guarda ou deriva dado ligado a uma pessoa ou organização (tabela, índice, prefixo de chave ou de bucket,
chave de cache, linha de fila ou job, registro em terceiro, campo de auditoria) só está pronta quando o fluxo de exclusão
dela existe e foi testado.** Na mesma alteração:

1. Inclua a linha dela no inventário de dados do serviço (apagar, anonimizar ou reter, com base legal e prazo; reter exige motivo).
2. Implemente o passo de apagar ou anonimizar no purge do serviço, idempotente e retomável (reexecutável do zero).
3. Crie ou atualize os bloqueios de elegibilidade se ela puder guardar dinheiro, dívida, trabalho em aberto ou titularidade.
4. Todo caminho assíncrono (webhook, job, varredura, consumidor de fila) deve descartar trabalho de `sub` ou organização travada ou apagada.
5. Teste: crie os dados, rode o purge duas vezes, confirme que sumiram ou foram anonimizados e que uma escrita tardia para o titular
   apagado é rejeitada. O teste de classificação de tabelas do purge deve falhar para tabela não classificada; mantenha-o exaustivo.
6. Revogue ou apague o que vive em terceiros (por exemplo webhooks ou credenciais registradas em banco).
7. Atualize o plano ou spec do participante e registre o impacto entre serviços (quem mais precisa agir quando isto for apagado).

Um pull request que adiciona dado ligado a pessoa sem isso está incompleto, independentemente do resto passar.

## Licença

[Elastic License 2.0 (ELv2)](LICENSE.md) — código fonte disponível, uso como serviço gerenciado por terceiros não
permitido.
