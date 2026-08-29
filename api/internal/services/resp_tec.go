package services

// resp_tec.go implementa infRespTec e o Código de Segurança do Responsável
// Técnico (NT 2018.005). Mora aqui, e não em nfes/ ou mdfes/, porque o grupo é
// literalmente o mesmo nó nos três leiautes (NF-e, CT-e e MDF-e) — ver
// "infRespTec — compartilhado" nas tabelas de ordem XSD.
//
// O CSRT em si é segredo: entra por configuração, nunca sai por API, nunca vai
// para log. O que viaja no XML é só o hash, que não é reversível.

import (
	"crypto/sha1"
	"encoding/base64"
)

// HashCSRT devolve Base64(SHA1(CSRT + chave de acesso)), conforme a NT 2018.005.
// A concatenação não tem separador.
func HashCSRT(csrt, accessKey string) string {
	sum := sha1.Sum([]byte(csrt + accessKey))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// BuildRespTec monta infRespTec. idCSRT/hashCSRT só aparecem quando a
// organização configurou um CSRT — o par é tudo-ou-nada no XSD.
func BuildRespTec(cnpj, contact, email, phone, idCSRT, csrt, accessKey string) map[string]any {
	node := map[string]any{
		"CNPJ":     cnpj,
		"xContato": contact,
		"email":    email,
		"fone":     phone,
	}
	if idCSRT != "" && csrt != "" {
		node["idCSRT"] = idCSRT
		node["hashCSRT"] = HashCSRT(csrt, accessKey)
	}
	return node
}
