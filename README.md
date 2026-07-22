# CTech DFe

Plataforma SaaS para emissão e gerenciamento de Documentos Fiscais Eletrônicos (NF-e, NFC-e, CT-e, MDF-e) com
comunicação direta à SEFAZ.

## Documentação

| Documento                                | Descrição                              |
|------------------------------------------|----------------------------------------|
| [`OVERVIEW.md`](OVERVIEW.md)             | Visão geral do sistema — leia primeiro |
| [`DOCS.md`](DOCS.md)                     | Documentação técnica completa          |
| [`CONDUCT.md`](CONDUCT.md)               | Código de conduta de desenvolvimento   |
| [`CLAUDE.md`](CLAUDE.md)                 | Instruções para Claude Code            |
| [`AGENTS.md`](AGENTS.md)                 | Contexto para agentes de IA            |
| [`DEPLOYMENT.md`](DEPLOYMENT.md) | Deploy, bootstrap, troubleshooting CDK |
| [`THEME.md`](THEME.md)                   | Design system e paleta de cores        |

## Subprojetos

| Diretório | Papel | README |
|-----------|-------|--------|
| `py-dfe/` | Biblioteca Python core — Lambda + SEFAZ SOAP (mTLS) | [py-dfe/README.md](py-dfe/README.md) |
| `go-dfe/` | Cliente Go SEFAZ (mTLS, in-process) | [go-dfe/README.md](go-dfe/README.md) |
| `api/`    | Backend REST — Go (Fiber v3), multi-tenant | [api/README.md](api/README.md) |
| `ui/`     | Frontend SaaS — Next.js + TypeScript + ShadCN | [ui/README.md](ui/README.md) |
| `worker/` | Workers assíncronos — Go Lambda (SQS consumers) | [worker/README.md](worker/README.md) |
| `cdk/`    | Infraestrutura AWS — CDK TypeScript | [cdk/README.md](cdk/README.md) |

```
py-dfe/     # Biblioteca Python core — Lambda + SEFAZ SOAP (mTLS)
go-dfe/     # Cliente Go SEFAZ (mTLS, in-process) — primário; py-dfe é fallback
api/        # Backend REST — Go (Fiber v3), multi-tenant
ui/         # Frontend SaaS — Next.js + TypeScript + ShadCN
worker/     # Workers assíncronos — Go Lambda (SQS consumers)
cdk/        # Infraestrutura AWS — CDK TypeScript
```

## Início Rápido

```bash
# Biblioteca SEFAZ
cd py-dfe && pip install -e ".[dev]" && pytest tests/unit/

# Backend
cd api && go run ./cmd/server

# Frontend
cd ui && npm install && npm run dev

# Infraestrutura
cd cdk && npm install && cdk synth
```

## Licença

[Elastic License 2.0 (ELv2)](LICENSE.md) — código fonte disponível, uso como serviço gerenciado por terceiros não permitido.