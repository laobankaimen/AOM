package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureLocalCertificate creates a self-signed LAN certificate only if neither
// file exists. Users can instead supply a certificate trusted by their clients.
func EnsureLocalCertificate(dir string) (string, string, error) {
	certPath, keyPath := filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key")
	certInfo, certErr := os.Stat(certPath)
	keyInfo, keyErr := os.Stat(keyPath)
	if certErr == nil && keyErr == nil {
		if certInfo.IsDir() || keyInfo.IsDir() {
			return "", "", errors.New("证书路径不能是目录")
		}
		return certPath, keyPath, nil
	}
	if !os.IsNotExist(certErr) || !os.IsNotExist(keyErr) {
		return "", "", errors.New("证书或私钥文件不完整，请保留完整证书对或显式指定证书")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", "", e
	}
	private, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return "", "", e
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return "", "", e
	}
	hostname, _ := os.Hostname()
	names := []string{"localhost"}
	if hostname != "" {
		names = append(names, hostname)
	}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		ip, _, e := net.ParseCIDR(addr.String())
		if e == nil && !ip.IsLoopback() && ip.IsGlobalUnicast() {
			ips = append(ips, ip)
		}
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "AOM local workspace"}, NotBefore: time.Now().Add(-5 * time.Minute), NotAfter: time.Now().AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: names, IPAddresses: ips, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, &template, &template, &private.PublicKey, private)
	if e != nil {
		return "", "", e
	}
	pkcs8, e := x509.MarshalPKCS8PrivateKey(private)
	if e != nil {
		return "", "", e
	}
	keyFile, e := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", "", e
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	_, e = keyFile.Write(keyBytes)
	clear(keyBytes)
	if e == nil {
		e = keyFile.Sync()
	}
	closeErr := keyFile.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		os.Remove(keyPath)
		return "", "", e
	}
	certFile, e := os.OpenFile(certPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		os.Remove(keyPath)
		return "", "", e
	}
	_, e = certFile.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	if e == nil {
		e = certFile.Sync()
	}
	closeErr = certFile.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		os.Remove(certPath)
		os.Remove(keyPath)
		return "", "", e
	}
	return certPath, keyPath, nil
}
