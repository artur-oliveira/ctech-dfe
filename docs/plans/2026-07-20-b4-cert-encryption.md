# B4 — Plano de implementação (Opção A)

**Data:** 2026-07-20 · **Entregue nesta passada:** spec + script de migração. **Pendente:** runtime (Fases 2.x).

## Fase 0 — Chave (tooling pronto)

- [x] `tools/migrate-cert-passwords genkey` gera key 32 bytes, imprime base64 + comando SSM.
- [ ] (operador)
  `aws ssm put-parameter --name /ctech-dfe/$ENV/cert-encryption-key --type SecureString --value <b64> --overwrite`
- verify: param existe, tipo `SecureString`, policy restrita à role dfe (`ssm:GetParameter` + `kms:Decrypt` na CMK).

## Fase 1 — Migração do repouso (tooling pronto)

- [x] `tools/migrate-cert-passwords migrate --table <env>organization_certificates` criptografa `password` plaintext
  in-place. Idempotente.
- verify: scan mostra `password` começando com `enc:v1:`; itens já criptografados/legados pulados; log de contagem no
  fim.
- ⚠️ Rodar **após** Fase 2.4 (worker com detecção) — ver spec.

## Fase 2 — Runtime (FUTURO, não nesta entrega)

1. api: helper `crypto/aes.go` (encrypt/decrypt, lê key do SSM no boot, cache em memória, deriva chave por cert via
   HKDF+salt).
    - verify: unit test encrypt→decrypt roundtrip; test detecção de prefixo `enc:v1:`.
2. api: criptografar `password` no Create/Upload (service `certificates.go`).
    - verify: novo cert gravado como `enc:v1:`; legado continua legível.
3. dfe: `GET /certificates/{id}/decrypt` (scope `internal:dfe:certificate:decrypt`), lê DynamoDB, descriptografa,
   retorna plaintext. M2M.
    - verify: integration test com token M2M válido retorna plaintext; sem scope → 403.
4. worker: detectar prefixo em `cert_password`; se `enc:v1:` → chamar `/decrypt`; senão `WARN` + usar como está.
    - verify: test com plaintext (warn, usa) e com `enc:v1:` (chama `/decrypt`).
5. SSE-KMS em SNS topic + SQS queue (cdk).
    - verify: fila/topic com `SseKms`; mensagens criptografadas em repouso.

## Ordem independente de deploy (Fases 2.1–2.4 vs Fase 1)

- Não rodar Fase 1 → tudo plaintext → funciona (requisito do operador atendido).
- Rodar Fase 1 **após** 2.4 → worker descriptografa, funciona.
- Rodar Fase 1 **antes** de 2.4 → worker quebra (recebe `enc:v1:` como senha). Evitar.
