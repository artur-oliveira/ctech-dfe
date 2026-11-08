// Package ciottest holds test helpers shared by go-dfe, go-dfe-egress and worker tests: a throwaway PFX with the
// ICP-Brasil otherName OID and a fake ANTT server (8 services plus the generator). Test use only; never a real
// host, certificate or document.
package ciottest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Fixed fake values.
const (
	PFXPassword   = "test-pfx-password"
	GeneratorKey  = "test-generator-key"
	GeneratedID   = "TESTGEN00001"
	Verifier      = "1234"
	DuplicateID   = "DUPLICATE001" // declaring this identifier answers 219
	RejectFloorID = "REJECTFLOOR1" // declaring this identifier answers 291
	CarrierCNPJ   = "11222333000181"
)

var (
	oidSAN     = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidICPCNPJ = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3}
)

// PFX returns a base64 PKCS12 of a throwaway RSA certificate. holder is the value of the OID 2.16.76.1.3.3
// otherName ("" omits it); clientAuth adds the client EKU.
func PFX(t testing.TB, holder string, clientAuth bool) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TEST CIOT"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	if clientAuth {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	if holder != "" {
		val, _ := asn1.MarshalWithParams(holder, "utf8")
		on, _ := asn1.Marshal(struct {
			ID    asn1.ObjectIdentifier
			Value asn1.RawValue `asn1:"tag:0,explicit"`
		}{oidICPCNPJ, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: val}})
		on[0] = 0xA0
		san, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: on})
		tpl.ExtraExtensions = []pkix.Extension{{Id: oidSAN, Value: san}}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pfx, err := pkcs12.Modern.Encode(key, cert, nil, PFXPassword)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(pfx)
}

// Server is the fake ANTT: TLS with a required client certificate (mTLS is really exercised).
type Server struct {
	*httptest.Server
	Paths []string // every request path, in order
	// ClientCertSeen counts requests that presented a client certificate.
	ClientCertSeen int
	// GeneratorStatus, when non-zero, makes /gerar answer that status.
	GeneratorStatus int
}

// NewServer starts the fake and registers its cleanup.
func NewServer(t testing.TB) *Server {
	t.Helper()
	s := &Server{}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(s.serve))
	s.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.Paths = append(s.Paths, r.URL.Path)
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		s.ClientCertSeen++
	}
	switch {
	case r.URL.Path == "/token":
		if r.Header.Get("Chave") != GeneratorKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		reply(w, map[string]string{"token": "jwt"})
	case r.URL.Path == "/gerar":
		if s.GeneratorStatus != 0 {
			w.WriteHeader(s.GeneratorStatus)
			return
		}
		reply(w, map[string]any{"sucesso": true, "dados": map[string]string{"ciot": GeneratedID}})
	case strings.HasPrefix(r.URL.Path, "/api/"):
		s.service(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) service(w http.ResponseWriter, r *http.Request) {
	service := strings.TrimPrefix(r.URL.Path, "/api/")
	var body map[string]any
	if r.Method == http.MethodPost {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
	}
	if service == "DeclaracaoOperacaoTransporte" {
		switch body["IdOperacaoTransporte"] {
		case DuplicateID:
			reply(w, map[string]any{"Codigo": []int{219}, "Mensagem": []string{"identificador duplicado"}, "Protocolo": "P219"})
			return
		case RejectFloorID:
			reply(w, map[string]any{"Codigo": 291, "Mensagem": "frete abaixo do piso", "Protocolo": "P291"})
			return
		}
		reply(w, map[string]any{"Codigo": []int{110}, "Mensagem": []string{"dados inseridos"}, "Protocolo": "P110",
			"IdOperacaoTransporte": body["IdOperacaoTransporte"], "CodigoVerificador": Verifier, "AvisoTransportador": "aviso de teste"})
		return
	}
	reply(w, map[string]any{"Codigo": 111, "Mensagem": "consulta realizada", "Protocolo": "P111"})
}
