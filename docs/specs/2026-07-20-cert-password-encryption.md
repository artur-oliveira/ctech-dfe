# B4 — Criptografia da senha de certificado (Opção A)

**Data:** 2026-07-20 · **Status:** Spec (runtime não implementado nesta entrega) · **Backlog:** `CTECH_BACKLOG.md` B4
**Decisão:** Opção A — chave AES-256 estática em SSM SecureString + AES-256-GCM no app. **Entregue nesta passada:**
spec + plan + script de migração (`tools/migrate-cert-passwords`).

## Contexto / ameaça

Hoje a senha do certificado PFX está em plaintext em dois lugares:

1. DynamoDB `organization_certificates.password` (repouso).
2. Corpo SNS/SQS `cert_password` (trânsito/repouso na fila). SSE-KMS ausente.

Sem KMS por operação (cenário 1M ops/mês): custo ~$0–1/mês, latência 0 por op (AES local <1ms).

## Decisão

Chave AES-256 (32 bytes) gerada uma vez, guardada em SSM Parameter Store **SecureString** em
`/ctech-dfe/{env}/cert-encryption-key`. O SecureString usa uma KMS CMK por baixo — KMS é tocado só no boot (leitura do
parâmetro), **não por op**. O app criptografa/descriptografa em memória.

IAM: só a role do dfe (api + worker) recebe `ssm:GetParameter` nesse parâmetro + `kms:Decrypt` nessa CMK. Nada de
`AdministratorAccess`.

## Formato do valor criptografado

```
enc:v1: + base64( salt[16] || iv[12] || ciphertext+tag )
```

- **AES-256-GCM** (AEAD): detecta adulteração.
- `salt` aleatório por certificado: usado como *info* do HKDF para derivar uma **chave por certificado** a partir da
  master (isolamento por cert a custo ~0, sem KMS por op).
- `iv` aleatório por operação (nonce GCM 12 bytes).
- Prefixo `enc:v1:` é o **marcador de retrocompatibilidade**: permite ao leitor distinguir plaintext de criptografado.

## Fluxo (pós-migração — implementação futura, NÃO nesta entrega)

1. Upload de cert: api lê senha plaintext do request → criptografa com a master → grava
   `password = enc:v1:...`.
2. Publisher (já faz `CertPassword: strAttr(cert, "password")` em `nfes/emit.go:340`,
   `mdfes/emit.go:303`, `nfce_service.go:252`): repassa o valor bruto como `cert_password` na mensagem SQS. **Sem
   alteração.**
3. Worker (`worker/internal/service/dfe.go`): recebe `cert_password`.
    - tem prefixo `enc:v1:` → chama `GET /certificates/{id}/decrypt` (M2M, scope
      `internal:dfe:certificate:decrypt`) → dfe lê `password` do DynamoDB, descriptografa, retorna plaintext.
    - **não tem prefixo** → é plaintext → loga `WARN "cert password em plaintext (migração não
     rodada ou legado); usando como está"` e segue. **Não tenta descriptografar.**

## Retrocompatibilidade (requisito do operador)

- Rodar o script de migração é **opcional por ambiente**. Não rodar NÃO quebra nada: dados legados em plaintext
  continuam funcionando (worker avisa e usa como está).
- O código de descriptografia pode subir antes ou depois do script. Antes: tudo plaintext → sem prefixo → usa como está.
  Depois: criptografado → descriptografa. Nenhum dos lados quebra.
- Script é **idempotente**: pula linhas já criptografadas (`enc:v1:`); nunca erra em plaintext (só criptografa).

> ⚠️ **Ordem de deploy:** a Fase 1 (migrar o repouso) só deve rodar **depois** do worker ter a
> lógica de detecção (Fase 2.4). Se o script rodar antes do runtime descriptografar, o worker
> receberá `enc:v1:...` como se fosse a senha e a assinatura falha. Ou seja: "não rodar o script
> não quebra" (OK), mas "rodar o script sem o runtime" quebra — responsabilidade do operador
> (rodar após o deploy do runtime).

## Rollback

Como o caminho plaintext sempre funciona: reverter = rodar script inverso (descriptografar tudo
`enc:v1:` → plaintext) ou apagar o parâmetro SSM e voltar código antigo. Seguro.

## Controles de segurança

- Parâmetro SSM com least-privilege (só role dfe).
- Chave nunca logada.
- (futuro) SSE-KMS no tópico SNS + fila SQS.
- (futuro) worker zera a senha em memória após uso (`defer` wipe); não loga resposta de `/decrypt`.

## Fora desta entrega (runtime — NÃO implementar agora)

- Helper crypto no app (api + dfe).
- Criptografar no Create/Upload (api).
- Endpoint `GET /certificates/{id}/decrypt` (dfe, M2M).
- Lógica de detecção no worker.
- SSE-KMS SNS/SQS.
- Script de migração (`tools/migrate-cert-passwords`) — **ENTREGUE**.
