package alipay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	client := &Client{PrivateKey: key, AlipayPublicKey: &key.PublicKey}
	params := map[string]string{
		"method": "alipay.user.info.share",
		"app_id": "app",
		"sign":   "ignored",
	}

	signature, err := client.Sign(params)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if err := client.Verify(canonicalContent(params), signature); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}

func TestDecodeResponseRequiresSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	client := &Client{AlipayPublicKey: &key.PublicKey}
	var out OauthTokenResponse
	err = client.decodeResponse([]byte(`{"alipay_system_oauth_token_response":{"access_token":"token","user_id":"uid"}}`), "alipay_system_oauth_token_response", &out)
	if err == nil || !strings.Contains(err.Error(), "signature missing") {
		t.Fatalf("decodeResponse err = %v, want signature missing", err)
	}
}

func TestParseCertFileAndRootCertSN(t *testing.T) {
	certPEM := testCertificatePEM(t)
	path := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(path, certPEM, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cert, err := ParseCertFile(path)
	if err != nil {
		t.Fatalf("ParseCertFile failed: %v", err)
	}
	if sn := GetCertSN(cert); len(sn) != 32 {
		t.Fatalf("SN length = %d, want md5 hex length 32", len(sn))
	}

	rootSN, err := GetRootCertSN(certPEM)
	if err != nil {
		t.Fatalf("GetRootCertSN failed: %v", err)
	}
	if !strings.Contains(rootSN, GetCertSN(cert)) {
		t.Fatalf("root SN %q does not contain cert SN", rootSN)
	}
}

func testCertificatePEM(t *testing.T) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1001),
		Subject:      pkix.Name{CommonName: "test"},
		Issuer:       pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
