package nfes

// nfce_qrcode.go builds the NFC-e infNFeSupl node (qrCode + urlChave).
//
// QR Code version 2.00, online emission. The signed XML is produced by the
// py-dfe Lambda, but the QR Code does NOT depend on the signature in online mode
// — it is derived solely from the access key, environment and the CSC (Código de
// Segurança do Contribuinte). So we build it here in the API before enqueuing.
//
// p = chNFe|nVersao|tpAmb|cIdToken|cHashQRCode
// cHashQRCode = SHA1( chNFe|nVersao|tpAmb|cIdToken + CSC ) in upper-case hex
//
// URL maps transcribed from nfce_chave.txt (urlChave / consulta) and
// nfce_qr_code.txt (QR Code base URL) — these MUST be validated against SEFAZ
// homologação before production use.

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
)

const qrCodeVersion = "2"

// nfceQRBase holds the QR Code base URL per environment.
type nfceQRBase struct {
	prod string
	hom  string
}

// nfceQRBaseURL maps emitter UF → QR Code base URL (from nfce_qr_code.txt).
var nfceQRBaseURL = map[string]nfceQRBase{
	"AC": {"http://www.sefaznet.ac.gov.br/nfce/qrcode?", "http://www.hml.sefaznet.ac.gov.br/nfce/qrcode?"},
	"AL": {"http://nfce.sefaz.al.gov.br/QRCode/consultarNFCe.jsp", "http://nfce.sefaz.al.gov.br/QRCode/consultarNFCe.jsp"},
	"AP": {"https://www.sefaz.ap.gov.br/nfce/nfcep.php", "https://www.sefaz.ap.gov.br/nfcehml/nfce.php"},
	"AM": {"https://sistemas.sefaz.am.gov.br/nfceweb/consultarNFCe.jsp", "https://homnfce.sefaz.am.gov.br/nfceweb/consultarNFCe.jsp"},
	"BA": {"http://nfe.sefaz.ba.gov.br/servicos/nfce/qrcode.aspx", "http://hnfe.sefaz.ba.gov.br/servicos/nfce/qrcode.aspx"},
	"CE": {"http://nfce.sefaz.ce.gov.br/pages/ShowNFCe.html?", "http://nfceh.sefaz.ce.gov.br/pages/ShowNFCe.html?"},
	"DF": {"http://www.fazenda.df.gov.br/nfce/qrcode?", "http://www.fazenda.df.gov.br/nfce/qrcode?"},
	"ES": {"http://app.sefaz.es.gov.br/ConsultaNFCe/qrcode.aspx", "http://homologacao.sefaz.es.gov.br/ConsultaNFCe/qrcode.aspx"},
	"GO": {"https://nfeweb.sefaz.go.gov.br/nfeweb/sites/nfce/danfeNFCe", "https://nfewebhomolog.sefaz.go.gov.br/nfeweb/sites/nfce/danfeNFCe"},
	"MA": {"http://nfce.sefaz.ma.gov.br/portal/consultarNFCe.jsp", "http://homologacao.sefaz.ma.gov.br/portal/consultarNFCe.jsp"},
	"MT": {"http://www.sefaz.mt.gov.br/nfce/consultanfce", "http://homologacao.sefaz.mt.gov.br/nfce/consultanfce"},
	"MS": {"http://www.dfe.ms.gov.br/nfce/qrcode?", "http://www.dfe.ms.gov.br/nfce/qrcode?"},
	"MG": {"https://portalsped.fazenda.mg.gov.br/portalnfce/sistema/qrcode.xhtml", "https://portalsped.fazenda.mg.gov.br/portalnfce/sistema/qrcode.xhtml"},
	"PA": {"https://appnfc.sefa.pa.gov.br/portal/view/consultas/nfce/nfceForm.seam", "https://appnfc.sefa.pa.gov.br/portal-homologacao/view/consultas/nfce/nfceForm.seam"},
	"PB": {"http://www.sefaz.pb.gov.br/nfce", "http://www.sefaz.pb.gov.br/nfcehom"},
	"PR": {"http://www.fazenda.pr.gov.br/nfce/qrcode?", "http://www.fazenda.pr.gov.br/nfce/qrcode?"},
	"PE": {"http://nfce.sefaz.pe.gov.br/nfce/consulta", "http://nfcehomolog.sefaz.pe.gov.br/nfce/consulta"},
	"PI": {"http://www.sefaz.pi.gov.br/nfce/qrcode", "http://www.sefaz.pi.gov.br/nfce/qrcode"},
	"RJ": {"https://consultadfe.fazenda.rj.gov.br/consultaNFCe/QRCode", "http://www4.fazenda.rj.gov.br/consultaNFCe/QRCode?"},
	"RN": {"https://nfce.sefaz.rn.gov.br/consultarNFCe.aspx", "https://hom.nfce.sefaz.rn.gov.br/consultarNFCe.aspx"},
	"RS": {"https://www.sefaz.rs.gov.br/NFCE/NFCE-COM.aspx", "https://www.sefaz.rs.gov.br/NFCE/NFCE-COM.aspx"},
	"RO": {"http://www.nfce.sefin.ro.gov.br/consultanfce/consulta.jsp", "http://www.nfce.sefin.ro.gov.br/consultanfce/consulta.jsp"},
	"RR": {"https://www.sefaz.rr.gov.br/nfce/servlet/qrcode", "http://200.174.88.103:8080/nfce/servlet/qrcode"},
	"SC": {"https://sat.sef.sc.gov.br/nfce/consulta?", "https://hom.sat.sef.sc.gov.br/nfce/consulta?"},
	"SP": {"https://www.nfce.fazenda.sp.gov.br/qrcode", "https://www.homologacao.nfce.fazenda.sp.gov.br/qrcode"},
	"SE": {"http://www.nfce.se.gov.br/nfce/qrcode?", "http://www.hom.nfe.se.gov.br/nfce/qrcode?"},
	"TO": {"http://www.sefaz.to.gov.br/nfce/qrcode", "http://homologacao.sefaz.to.gov.br/nfce/qrcode"},
}

// nfceConsultURL maps emitter UF → consumer consultation URL (urlChave), per
// environment, from nfce_chave.txt. For most UFs prod and hom share the same
// endpoint; BA/MG/MT/PB/SC/SP/SE/TO differ.
var nfceConsultURL = map[string]nfceQRBase{
	"AC": {"https://www.sefaznet.ac.gov.br/nfce/consulta", "https://www.sefaznet.ac.gov.br/nfce/consulta"},
	"AL": {"https://www.sefaz.al.gov.br/nfce/consulta", "https://www.sefaz.al.gov.br/nfce/consulta"},
	"AP": {"https://www.sefaz.ap.gov.br/nfce/consulta", "https://www.sefaz.ap.gov.br/nfce/consulta"},
	"AM": {"https://www.sefaz.am.gov.br/nfce/consulta", "https://www.sefaz.am.gov.br/nfce/consulta"},
	"BA": {"https://www.sefaz.ba.gov.br/nfce/consulta", "http://hinternet.sefaz.ba.gov.br/nfce/consulta"},
	"CE": {"https://www.sefaz.ce.gov.br/nfce/consulta", "https://www.sefaz.ce.gov.br/nfce/consulta"},
	"DF": {"https://www.fazenda.df.gov.br/nfce/consulta", "https://www.fazenda.df.gov.br/nfce/consulta"},
	"ES": {"https://www.sefaz.es.gov.br/nfce/consulta", "https://www.sefaz.es.gov.br/nfce/consulta"},
	"GO": {"https://www.sefaz.go.gov.br/nfce/consulta", "https://www.sefaz.go.gov.br/nfce/consulta"},
	"MA": {"https://www.sefaz.ma.gov.br/nfce/consulta", "https://www.sefaz.ma.gov.br/nfce/consulta"},
	"MT": {"http://www.sefaz.mt.gov.br/nfce/consultanfce", "http://homologacao.sefaz.mt.gov.br/nfce/consultanfce"},
	"MS": {"https://www.dfe.ms.gov.br/nfce/consulta", "https://www.dfe.ms.gov.br/nfce/consulta"},
	"MG": {"http://nfce.fazenda.mg.gov.br/portalnfce", "http://hnfce.fazenda.mg.gov.br/portalnfce"},
	"PA": {"https://www.sefa.pa.gov.br/nfce/consulta", "https://www.sefa.pa.gov.br/nfce/consulta"},
	"PB": {"https://www.receita.pb.gov.br/nfce/consulta", "https://www.receita.pb.gov.br/nfcehom"},
	"PR": {"http://www.fazenda.pr.gov.br/nfce/consulta", "http://www.fazenda.pr.gov.br/nfce/consulta"},
	"PE": {"https://nfce.sefaz.pe.gov.br/nfce/consulta", "https://nfce.sefaz.pe.gov.br/nfce/consulta"},
	"PI": {"https://www.sefaz.pi.gov.br/nfce/consulta", "https://www.sefaz.pi.gov.br/nfce/consulta"},
	"RJ": {"https://www.fazenda.rj.gov.br/nfce/consulta", "https://www.fazenda.rj.gov.br/nfce/consulta"},
	"RN": {"https://www.set.rn.gov.br/nfce/consulta", "https://www.set.rn.gov.br/nfce/consulta"},
	"RS": {"https://www.sefaz.rs.gov.br/nfce/consulta", "https://www.sefaz.rs.gov.br/nfce/consulta"},
	"RO": {"https://www.sefin.ro.gov.br/nfce/consulta", "https://www.sefin.ro.gov.br/nfce/consulta"},
	"RR": {"https://www.sefaz.rr.gov.br/nfce/consulta", "https://www.sefaz.rr.gov.br/nfce/consulta"},
	"SC": {"https://sat.sef.sc.gov.br/nfce/consulta", "https://hom.sat.sef.sc.gov.br/nfce/consulta"},
	"SP": {"https://www.nfce.fazenda.sp.gov.br/consulta", "https://www.homologacao.nfce.fazenda.sp.gov.br/consulta"},
	"SE": {"http://www.nfce.se.gov.br/nfce/consulta", "http://www.hom.nfe.se.gov.br/nfce/consulta"},
	"TO": {"https://www.sefaz.to.gov.br/nfce/consulta", "http://homologacao.sefaz.to.gov.br/nfce/consulta.jsf"},
}

// buildNFCeSupl returns the infNFeSupl node (qrCode + urlChave) for an NFC-e.
// environment: 1 = produção, 2 = homologação.
func buildNFCeSupl(uf string, environment int, accessKey, cscID, csc string) (map[string]any, error) {
	base, ok := nfceQRBaseURL[uf]
	if !ok {
		return nil, problem.BadRequest("NFC-e não disponível para a UF do emitente: " + uf)
	}
	consult, ok := nfceConsultURL[uf]
	if !ok {
		return nil, problem.BadRequest("URL de consulta de NFC-e não configurada para a UF: " + uf)
	}
	if csc == "" || cscID == "" {
		return nil, problem.BadRequest("configure o CSC da NFC-e em Configuração Fiscal antes de emitir")
	}

	qrCode := buildQRCode(qrBaseForEnv(base, environment), accessKey, cscID, csc, environment)
	return map[string]any{
		"qrCode":   qrCode,
		"urlChave": qrBaseForEnv(consult, environment),
	}, nil
}

func qrBaseForEnv(base nfceQRBase, environment int) string {
	if environment == 1 {
		return base.prod
	}
	return base.hom
}

// buildQRCode builds the QR Code string for online NFC-e (version 2.00).
func buildQRCode(baseURL, accessKey, cscID, csc string, environment int) string {
	tpAmb := strconv.Itoa(environment)
	dados := strings.Join([]string{accessKey, qrCodeVersion, tpAmb, cscID}, "|")
	sum := sha1.Sum([]byte(dados + csc))
	hash := strings.ToUpper(hex.EncodeToString(sum[:]))
	p := dados + "|" + hash
	return appendQueryParam(baseURL, "p="+p)
}

// appendQueryParam appends a query parameter respecting an existing "?".
func appendQueryParam(baseURL, param string) string {
	switch {
	case strings.HasSuffix(baseURL, "?"):
		return baseURL + param
	case strings.Contains(baseURL, "?"):
		return baseURL + "&" + param
	default:
		return baseURL + "?" + param
	}
}
