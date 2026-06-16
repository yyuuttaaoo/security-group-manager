package alipay

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

func GetCertSN(cert *x509.Certificate) string {
	attrs := strings.Split(cert.Issuer.String(), ", ")
	for i := len(attrs)/2 - 1; i >= 0; i-- {
		opp := len(attrs) - 1 - i
		attrs[i], attrs[opp] = attrs[opp], attrs[i]
	}

	hash := md5.Sum([]byte(strings.Join(attrs, ",") + cert.SerialNumber.String()))
	return hex.EncodeToString(hash[:])
}

func GetRootCertSN(certContent []byte) (string, error) {
	var sns []string
	rest := certContent

	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if strings.HasPrefix(cert.SignatureAlgorithm.String(), "SHA") && strings.Contains(cert.SignatureAlgorithm.String(), "RSA") {
			sns = append(sns, GetCertSN(cert))
		} else if cert.SignatureAlgorithm == x509.UnknownSignatureAlgorithm {
			sns = append(sns, GetCertSN(cert))
		}
	}

	if len(sns) == 0 {
		return "", fmt.Errorf("failed to extract any root cert SN")
	}
	return strings.Join(sns, "_"), nil
}

func ParseCertFile(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}
