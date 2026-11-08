# XSDs oficiais vendorizados

Fonte do gerador de specs (`go-dfe/validate/gen`) e do teste de paridade com `xsdorder`. Integridade: `SHA256SUMS`
(testado por `validate/schemas_hash_test.go`). Baixados dos portais SVRS
(`https://dfe-portal.svrs.rs.gov.br/{NFE,CTE,MDFE}/DownloadArquivoEstatico/?sistema=..&tipoArquivo=2&nomeArquivo=..`),
sem cookies nem user agent alterado.

| Documento | Pacote (pasta) | Origem | Publicação |
|---|---|---|---|
| NF-e/NFC-e | `nfe/PL_010e_v1.02` (documento e eventos) | pacote 010e, mesmo conjunto dos XSDs do documento; os eventos vieram do mesmo pacote (idênticos byte a byte aos de `Eventos_RTC.zip` do portal, exceto 211110, que é o da NT 2025.002 v1.40 de 23/07/2026) | o 010e não está mais na listagem do portal; data não confirmada |
| CT-e | `cte/PL_CTe_400_NT2026.005_SN_1.00` | portal CTE, `PL_CTe_400_NT2026.005 RTC SN_1.00.zip` | arquivo de 15/10/2026 |
| MDF-e | `mdfe/PL_MDFe_300b_NT012025_1.05` | portal MDFE, `PL_MDFe_300b_NT012025_1.04.zip` (a pasta interna chama-se 1.05) | arquivo de 25/04/2026 |

Eventos presentes: NF-e 110001, 110110, 110111, 110112, 110140, 1115xx, 1121xx, 2102xx, 2111xx, 2121xx, 4121xx;
CT-e todos os `ev*` do pacote; MDF-e todos os `ev*` do pacote.

Não obtidos: NF-e 110130/110131 (comprovante de entrega) e 110150 (ator interessado) não estão em nenhum pacote
acessível (o portal devolve zip vazio para `Evento_AtorInteressado_PL_v1.01.zip` e não lista pacote de comprovante de
entrega, só a NT em PDF); as pastas de insucesso NF-e (110192/110193) e Econf (110750/110751) existem no portal mas o
código não emite esses eventos. `xmldsig-core-schema` não é vendorizado (o `ds:Signature` fica como `any`).
