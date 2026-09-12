package certificates

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	bundleFilename = "authority.bundle"
	leafCacheLimit = 256
)

type Info struct {
	Available   bool      `json:"available"`
	Subject     string    `json:"subject"`
	Fingerprint string    `json:"fingerprint"`
	NotBefore   time.Time `json:"notBefore"`
	NotAfter    time.Time `json:"notAfter"`
}

type Authority struct {
	mu        sync.Mutex
	dir       string
	cert      *x509.Certificate
	key       *ecdsa.PrivateKey
	leaves    map[string]*tls.Certificate
	leafOrder []string
}

type diskBundle struct {
	Certificate []byte `json:"certificate"`
	PrivateKey  []byte `json:"privateKey"`
}

func Open(dir string) (*Authority, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("certificate directory is empty")
	}
	a := &Authority{dir: dir, leaves: make(map[string]*tls.Certificate)}
	raw, err := os.ReadFile(filepath.Join(dir, bundleFilename))
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read certificate authority: %w", err)
	}
	plain, err := unprotect(raw)
	if err != nil {
		return nil, fmt.Errorf("unprotect certificate authority: %w", err)
	}
	var bundle diskBundle
	if err := json.Unmarshal(plain, &bundle); err != nil {
		return nil, fmt.Errorf("decode certificate authority: %w", err)
	}
	cert, err := x509.ParseCertificate(bundle.Certificate)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(bundle.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse CA private key: %w", err)
	}
	key, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok || !cert.IsCA || cert.CheckSignatureFrom(cert) != nil || !key.PublicKey.Equal(cert.PublicKey) {
		return nil, errors.New("invalid certificate authority bundle")
	}
	a.cert, a.key = cert, key
	return a, nil
}

func (a *Authority) Info() Info {
	a.mu.Lock()
	defer a.mu.Unlock()
	return infoFor(a.cert)
}

func (a *Authority) Generate() (Info, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert != nil {
		return infoFor(a.cert), nil
	}
	path := filepath.Join(a.dir, bundleFilename)
	if _, err := os.Stat(path); err == nil {
		return Info{}, errors.New("certificate authority store already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Info{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Info{}, err
	}
	now := time.Now().UTC()
	serial, err := randomSerial()
	if err != nil {
		return Info{}, fmt.Errorf("generate CA serial: %w", err)
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: now.Local().Format("2006-01-02_15-04-05-0700"), Organization: []string{"MockCharles"}}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(-5*time.Minute).AddDate(1, 0, 0), KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true, IsCA: true, MaxPathLen: 0}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return Info{}, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return Info{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Info{}, err
	}
	plain, err := json.Marshal(diskBundle{Certificate: der, PrivateKey: keyDER})
	if err != nil {
		return Info{}, err
	}
	stored, err := protect(plain)
	if err != nil {
		return Info{}, fmt.Errorf("protect certificate authority: %w", err)
	}
	if err := writeAtomic(a.dir, path, stored); err != nil {
		return Info{}, err
	}
	a.cert, a.key = cert, key
	return infoFor(cert), nil
}

func (a *Authority) PublicDER() ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert == nil {
		return nil, errors.New("certificate authority is unavailable")
	}
	return append([]byte(nil), a.cert.Raw...), nil
}

// PublicFilename derives a stable, filesystem-safe name from the certificate.
func PublicFilename(der []byte) (string, error) {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return "", err
	}
	const layout = "2006-01-02_15-04-05-0700"
	if generated, err := time.Parse(layout, cert.Subject.CommonName); err == nil {
		return generated.Format(layout) + ".crt", nil
	}
	// Older authorities did not store generation time; retain their existing name.
	return "MockCharles-CA.crt", nil
}

func (a *Authority) Certificate(host string) (*tls.Certificate, error) {
	name, ip, err := validHost(host)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert == nil || a.key == nil {
		return nil, errors.New("certificate authority is unavailable")
	}
	now := time.Now().UTC()
	if now.Before(a.cert.NotBefore) {
		return nil, errors.New("certificate authority is not yet valid")
	}
	if !now.Before(a.cert.NotAfter) {
		return nil, errors.New("certificate authority has expired")
	}
	if cached := a.leaves[name]; cached != nil {
		if cached.Leaf != nil && now.Before(cached.Leaf.NotAfter) && !now.Before(cached.Leaf.NotBefore) {
			return cached, nil
		}
		delete(a.leaves, name)
		for i, cachedName := range a.leafOrder {
			if cachedName == name {
				a.leafOrder = append(a.leafOrder[:i], a.leafOrder[i+1:]...)
				break
			}
		}
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	notBefore := now.Add(-5 * time.Minute)
	expires := notBefore.Add(90 * 24 * time.Hour)
	if expires.After(a.cert.NotAfter) {
		expires = a.cert.NotAfter
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("generate leaf serial: %w", err)
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: notBefore, NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	if ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{name}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &leafKey.PublicKey, a.key)
	if err != nil {
		return nil, err
	}
	parsedLeaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	leaf := &tls.Certificate{Certificate: [][]byte{der, a.cert.Raw}, PrivateKey: leafKey, Leaf: parsedLeaf}
	if len(a.leafOrder) == leafCacheLimit {
		old := a.leafOrder[0]
		a.leafOrder = a.leafOrder[1:]
		delete(a.leaves, old)
	}
	a.leaves[name] = leaf
	a.leafOrder = append(a.leafOrder, name)
	return leaf, nil
}

func infoFor(cert *x509.Certificate) Info {
	if cert == nil {
		return Info{}
	}
	sum := sha256.Sum256(cert.Raw)
	return Info{Available: true, Subject: cert.Subject.String(), Fingerprint: strings.ToUpper(hex.EncodeToString(sum[:])), NotBefore: cert.NotBefore, NotAfter: cert.NotAfter}
}
func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	for {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return nil, err
		}
		if n.Sign() != 0 {
			return n, nil
		}
	}
}

func validHost(host string) (string, net.IP, error) {
	if host == "" || host != strings.TrimSpace(host) || strings.ContainsAny(host, "/\\*[] ") {
		return "", nil, errors.New("invalid certificate host")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), ip, nil
	}
	name := strings.ToLower(strings.TrimSuffix(host, "."))
	if len(name) == 0 || len(name) > 253 || strings.Contains(name, ":") {
		return "", nil, errors.New("invalid certificate host")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", nil, errors.New("invalid certificate host")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", nil, errors.New("invalid certificate host")
			}
		}
	}
	return name, nil, nil
}

func writeAtomic(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create certificate directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("secure certificate directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".authority-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(temp, path); err != nil {
		return fmt.Errorf("commit certificate authority: %w", err)
	}
	return nil
}
