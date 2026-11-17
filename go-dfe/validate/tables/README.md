# tables

`ibscbs.json`: tabelas de CST e cClassTrib do IBS/CBS usadas pelas regras F76 a F140 do BP-e TA (`../rules_bpe_ibscbs.go`) e
pela API (`validate.IBSCBSClassByCode`).

- **Fonte:** `Tabela IT 2025.002 v.1.71 - cClassTrib.xlsx` (aba `cClass` de 2026-09-01 e aba `CST`), portal nacional,
  `https://dfe-portal.svrs.rs.gov.br/DFE/DownloadArquivoEstatico/?sistema=DFE&tipoArquivo=12&nomeArquivo=...`, publicada em 30/10/2026.
  SHA-256 do arquivo: `d104ce4df4d2fd8ea6329ce794d8d8ce03855f3adc1692b99f5446d87c4c71f8`.
- **Colunas usadas** (aba `cClass`): `CST-IBS/CBS`, `cClassTrib`, `Nome cClassTrib`, `Tipo de Alíquota`, `pRedIBS`, `pRedCBS`,
  `ind_gTribRegular`, `ind_gEstornoCred`, `tpDoacao`, `indBPe`, `indBPeTA`, `indBPeTM` (aba `CST`: `ind_gIBSCBS`, `ind_gRed`, `ind_gDif`).
- **Como atualizar:** baixar a versão nova, converter as duas abas para o mesmo formato (um objeto por linha, campos como texto) e
  rodar `go test ./validate`. O teste `TestIBSCBSTablesLoad` fixa em oito as classificações do TA; mudar esse número é decisão
  consciente, e `BPE_TA_CLASSES` em `ui/src/lib/constants/bpe.ts` precisa seguir a tabela.
