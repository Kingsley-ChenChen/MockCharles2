package certificates

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenAbsentAndGenerateReopenStable(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.Info(); got.Available {
		t.Fatalf("Info().Available = true before generation")
	}
	if _, err := a.PublicDER(); err == nil {
		t.Fatal("PublicDER succeeded before generation")
	}

	first, err := a.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Available || first.Subject == "" || first.Fingerprint == "" {
		t.Fatalf("incomplete info: %+v", first)
	}
	if first.NotAfter.Sub(first.NotBefore) < 5*365*24*time.Hour-time.Hour {
		t.Fatalf("CA validity too short: %v", first.NotAfter.Sub(first.NotBefore))
	}
	der, err := a.PublicDER()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	if first.Fingerprint != strings.ToUpper(hex.EncodeToString(sum[:])) {
		t.Fatalf("unexpected fingerprint %q", first.Fingerprint)
	}

	second, err := a.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("repeat Generate rotated authority: first=%+v second=%+v", first, second)
	}
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Info() != first {
		t.Fatalf("reopened info changed: %+v", b.Info())
	}
	der2, err := b.PublicDER()
	if err != nil {
		t.Fatal(err)
	}
	if string(der2) != string(der) {
		t.Fatal("reopened public certificate changed")
	}
}

func TestCertificateDNSAndIPSANVerifyAndHostsDiffer(t *testing.T) {
	a, _ := Open(t.TempDir())
	if _, err := a.Generate(); err != nil {
		t.Fatal(err)
	}
	caDER, _ := a.PublicDER()
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)

	dns := mustLeaf(t, a, "example.test")
	verifyLeaf(t, dns, roots, "example.test")
	ip := mustLeaf(t, a, "127.0.0.1")
	verifyLeaf(t, ip, roots, "127.0.0.1")
	if string(dns.Certificate[0]) == string(ip.Certificate[0]) {
		t.Fatal("different hosts received same leaf")
	}
	dnsAgain := mustLeaf(t, a, "example.test")
	if dns != dnsAgain {
		t.Fatal("leaf was not returned from cache")
	}
	leaf, _ := x509.ParseCertificate(dns.Certificate[0])
	if leaf.NotAfter.After(ca.NotAfter) {
		t.Fatal("leaf expires after CA")
	}
	if leaf.NotAfter.Sub(leaf.NotBefore) > 90*24*time.Hour+time.Minute {
		t.Fatalf("leaf validity exceeds 90 days: %v", leaf.NotAfter.Sub(leaf.NotBefore))
	}
}

func TestCertificateRejectsInvalidHostAndUnavailableAuthority(t *testing.T) {
	a, _ := Open(t.TempDir())
	if _, err := a.Certificate("example.test"); err == nil {
		t.Fatal("Certificate succeeded before generation")
	}
	if _, err := a.Generate(); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"", " ", "https://example.test", "example.test:443", "*.example.test", "bad host", "[::1]"} {
		if _, err := a.Certificate(host); err == nil {
			t.Errorf("Certificate(%q) succeeded", host)
		}
	}
}

func TestCorruptStoreRejectedWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, bundleFilename)
	want := []byte("corrupt authority")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("Open accepted corrupt store")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("Open overwrote corrupt store")
	}
}

func TestCertificateConcurrentAndCacheBounded(t *testing.T) {
	a, _ := Open(t.TempDir())
	if _, err := a.Generate(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.Certificate("same.example"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < leafCacheLimit+10; i++ {
		mustLeaf(t, a, "host"+itoa(i)+".example")
	}
	a.mu.Lock()
	size := len(a.leaves)
	a.mu.Unlock()
	if size != leafCacheLimit {
		t.Fatalf("cache size=%d want %d", size, leafCacheLimit)
	}
}

func TestCertificateRegeneratesExpiredCachedLeaf(t *testing.T) {
	a, _ := Open(t.TempDir())
	if _, err := a.Generate(); err != nil {
		t.Fatal(err)
	}
	first := mustLeaf(t, a, "expired.example")
	parsed, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	parsed.NotAfter = time.Now().Add(-time.Minute)
	first.Leaf = parsed
	second := mustLeaf(t, a, "expired.example")
	if second == first {
		t.Fatal("expired cached leaf was reused")
	}
}

func TestCertificateRejectsExpiredCAWithoutRotating(t *testing.T) {
	a, _ := Open(t.TempDir())
	info, err := a.Generate()
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.cert.NotAfter = time.Now().Add(-time.Minute)
	a.mu.Unlock()
	if _, err := a.Certificate("example.test"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Certificate error = %v, want expired CA error", err)
	}
	stable, err := a.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if stable.Fingerprint != info.Fingerprint {
		t.Fatal("Generate rotated expired authority")
	}
}

func TestGenerateDoesNotOverwriteAuthorityPublishedByAnotherHandle(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Generate(); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, bundleFilename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Generate(); err == nil {
		t.Fatal("second handle overwrote an authority published after Open")
	}
	got, err := os.ReadFile(filepath.Join(dir, bundleFilename))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("published authority changed")
	}
}

func mustLeaf(t *testing.T, a *Authority, host string) *tls.Certificate {
	t.Helper()
	c, err := a.Certificate(host)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func verifyLeaf(t *testing.T, c *tls.Certificate, roots *x509.CertPool, host string) {
	t.Helper()
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); err != nil {
		t.Fatal(err)
	}
}
func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	b := make([]byte, 0, 4)
	for ; i > 0; i /= 10 {
		b = append(b, digits[i%10])
	}
	for l, r := 0, len(b)-1; l < r; l, r = l+1, r-1 {
		b[l], b[r] = b[r], b[l]
	}
	return string(b)
}
